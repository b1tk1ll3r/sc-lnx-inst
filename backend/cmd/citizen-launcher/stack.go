package main

import (
	"bufio"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	winetricksRepo = "Winetricks/winetricks"
	rsiBaseURL     = "https://install.robertsspaceindustries.com/rel/2"
	rsiLatestYML   = rsiBaseURL + "/latest.yml"
)

// Keep the mandatory prefix small and boring. PowerShell is deliberately not
// a base dependency: current LUG documentation exposes it as a separate
// maintenance/troubleshooting action, and PowerShell Core MSI installation can
// fail under otherwise healthy Wine prefixes.
var basePrefixWinetricksVerbs = []string{"arial", "tahoma", "win11"}

type GameConfig struct {
	Prefix            string `json:"prefix"`
	GameDir           string `json:"game_dir"`
	LauncherEXE       string `json:"launcher_exe"`
	RSIInstaller      string `json:"rsi_installer,omitempty"`
	WinetricksVersion string `json:"winetricks_version,omitempty"`
	InstalledAt       string `json:"installed_at,omitempty"`
	LastRepair        string `json:"last_repair,omitempty"`
}

type GameStatus struct {
	BackendVersion string `json:"backend_version"`
	Health         string `json:"health"`
	Hardware       string `json:"hardware"`
	HardwareReason string `json:"hardware_reason,omitempty"`
	GPU            string `json:"gpu,omitempty"`
	Vulkan         string `json:"vulkan,omitempty"`
	CPUAVX         bool   `json:"cpu_avx"`
	RAMGiB         int    `json:"ram_gib"`
	CombinedGiB    int    `json:"combined_gib"`
	DiskFreeGiB    int    `json:"disk_free_gib"`
	Prefix         string `json:"prefix"`
	PrefixState    string `json:"prefix_state"`
	LauncherState  string `json:"launcher_state"`
	GameState      string `json:"game_state"`
	WineVersion    string `json:"wine_version,omitempty"`
	DXVKVersion    string `json:"dxvk_version,omitempty"`
	LUGVersion     string `json:"lug_version,omitempty"`
	RSIInstaller   string `json:"rsi_installer,omitempty"`
	Autopilot      bool   `json:"autopilot"`
}

func (a *App) gameConfigPath() string { return filepath.Join(a.configDir, "game.json") }

func (a *App) loadGameConfig() GameConfig {
	cfg := GameConfig{}
	if data, err := os.ReadFile(a.gameConfigPath()); err == nil {
		_ = json.Unmarshal(data, &cfg)
	}
	if cfg.Prefix == "" {
		// Adopt an existing LUG prefix if one is configured, otherwise use our stable default.
		old := filepath.Join(envOr("XDG_CONFIG_HOME", filepath.Join(a.home, ".config")), "starcitizen-lug", "winedir.conf")
		if data, err := os.ReadFile(old); err == nil && strings.TrimSpace(string(data)) != "" {
			cfg.Prefix = strings.TrimSpace(string(data))
		} else {
			cfg.Prefix = filepath.Join(a.home, "Games", "star-citizen")
		}
	}
	if cfg.GameDir == "" {
		cfg.GameDir = filepath.Join(cfg.Prefix, "drive_c", "Program Files", "Roberts Space Industries", "StarCitizen")
	}
	if cfg.LauncherEXE == "" {
		cfg.LauncherEXE = filepath.Join(cfg.Prefix, "drive_c", "Program Files", "Roberts Space Industries", "RSI Launcher", "RSI Launcher.exe")
	}
	return cfg
}

func (a *App) saveGameConfig(cfg GameConfig) error {
	if err := os.MkdirAll(a.configDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := a.gameConfigPath() + ".new"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, a.gameConfigPath())
}

func (a *App) gameStatus() GameStatus {
	gc := a.loadGameConfig()
	uc := a.loadConfig()
	hs := a.hardwareStatus(gc.Prefix)
	st := GameStatus{
		BackendVersion: appVersion,
		Health:         "setup",
		Hardware:       hs.State,
		HardwareReason: hs.Reason,
		GPU:            hs.GPU,
		Vulkan:         hs.Vulkan,
		CPUAVX:         hs.AVX,
		RAMGiB:         hs.RAMGiB,
		CombinedGiB:    hs.CombinedGiB,
		DiskFreeGiB:    hs.DiskFreeGiB,
		Prefix:         gc.Prefix,
		PrefixState:    "missing",
		LauncherState:  "missing",
		GameState:      "missing",
		WineVersion:    readMetaVersion(filepath.Join(a.vendorDir, "wine", "meta.json")),
		DXVKVersion:    readMetaVersion(filepath.Join(a.vendorDir, "dxvk", "meta.json")),
		LUGVersion:     readMetaVersion(filepath.Join(a.vendorDir, "lug-helper", "meta.json")),
		RSIInstaller:   gc.RSIInstaller,
		Autopilot:      uc.AutoMaintain,
	}
	if hs.State == "blocked" {
		st.Health = "hardware-blocked"
		return st
	}
	if prefixInitialized(gc.Prefix) {
		st.PrefixState = "ready"
	} else if fi, err := os.Stat(gc.Prefix); err == nil && fi.IsDir() {
		st.PrefixState = "partial"
	}
	if _, err := os.Stat(gc.LauncherEXE); err == nil {
		st.LauncherState = "ready"
	}
	if _, err := os.Stat(filepath.Join(gc.GameDir, "LIVE", "Bin64", "StarCitizen.exe")); err == nil {
		st.GameState = "ready"
	} else if _, err := os.Stat(filepath.Join(gc.GameDir, "LIVE", "Data.p4k")); err == nil {
		st.GameState = "partial"
	}

	switch {
	case st.WineVersion == "":
		st.Health = "setup"
	case st.PrefixState == "partial":
		st.Health = "repair"
	case st.PrefixState != "ready":
		st.Health = "install"
	case st.LauncherState != "ready":
		st.Health = "launcher-repair"
	case st.GameState != "ready":
		st.Health = "install-game"
	default:
		st.Health = "ready"
	}
	return st
}

type hardwareInfo struct {
	State       string
	Reason      string
	GPU         string
	Vulkan      string
	AVX         bool
	RAMGiB      int
	CombinedGiB int
	DiskFreeGiB int
}

func (a *App) hardwareStatus(prefix string) hardwareInfo {
	h := hardwareInfo{State: "ready", Vulkan: "unknown"}
	if runtime.GOARCH != "amd64" {
		h.State = "blocked"
		h.Reason = "Star Citizen benötigt x86-64."
		return h
	}
	if data, err := os.ReadFile("/proc/cpuinfo"); err == nil {
		low := strings.ToLower(string(data))
		h.AVX = regexp.MustCompile(`(?m)^flags\s*:.*\bavx\b`).MatchString(low)
		if !h.AVX {
			h.State = "blocked"
			h.Reason = "Die CPU stellt AVX nicht bereit."
		}
	}
	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		vals := map[string]int64{}
		sc := bufio.NewScanner(strings.NewReader(string(data)))
		for sc.Scan() {
			f := strings.Fields(sc.Text())
			if len(f) >= 2 {
				n, _ := strconv.ParseInt(f[1], 10, 64)
				vals[strings.TrimSuffix(f[0], ":")] = n
			}
		}
		h.RAMGiB = int(vals["MemTotal"] / (1024 * 1024))
		h.CombinedGiB = int((vals["MemTotal"] + vals["SwapTotal"]) / (1024 * 1024))
		if h.RAMGiB < 14 && h.State != "blocked" {
			h.State = "blocked"
			h.Reason = "Weniger als 16 GiB RAM erkannt."
		}
	}
	var stat syscallStatfs
	if statfs(prefix, &stat) == nil {
		h.DiskFreeGiB = int((stat.Bavail * uint64(stat.Bsize)) / (1024 * 1024 * 1024))
	} else if statfs(a.home, &stat) == nil {
		h.DiskFreeGiB = int((stat.Bavail * uint64(stat.Bsize)) / (1024 * 1024 * 1024))
	}

	pci, _ := exec.Command("lspci", "-nnk").CombinedOutput()
	p := string(pci)
	if strings.TrimSpace(p) == "" {
		// lspci/pciutils is optional on minimal Debian installations. Detect the
		// QEMU VGA device through sysfs as a dependency-free fallback.
		vendors, _ := filepath.Glob("/sys/class/drm/card*/device/vendor")
		for _, vp := range vendors {
			v, _ := os.ReadFile(vp)
			d, _ := os.ReadFile(filepath.Join(filepath.Dir(vp), "device"))
			if strings.TrimSpace(string(v)) == "0x1234" && strings.TrimSpace(string(d)) == "0x1111" {
				p = "QEMU Standard VGA [1234:1111]\nKernel driver in use: bochs-drm"
				break
			}
		}
	}
	for _, line := range strings.Split(p, "\n") {
		low := strings.ToLower(line)
		if strings.Contains(low, "vga compatible controller") || strings.Contains(low, "3d controller") || strings.Contains(low, "display controller") {
			if h.GPU == "" {
				h.GPU = strings.TrimSpace(line)
			}
		}
	}
	lowp := strings.ToLower(p)
	if strings.Contains(lowp, "1234:1111") || strings.Contains(lowp, "kernel driver in use: bochs") {
		h.State = "blocked"
		h.Vulkan = "blocked"
		h.Reason = "QEMU Standard VGA/bochs-drm erkannt. Star Citizen benötigt eine echte Vulkan-GPU oder GPU-Passthrough."
		return h
	}
	if path, err := exec.LookPath("vulkaninfo"); err == nil {
		cmd := exec.Command(path, "--summary")
		out, err := cmd.CombinedOutput()
		if err != nil || strings.Contains(string(out), "VK_ERROR_INCOMPATIBLE_DRIVER") {
			h.State = "blocked"
			h.Vulkan = "blocked"
			h.Reason = "Vulkan ist nicht funktionsfähig."
			return h
		}
		h.Vulkan = "ready"
	} else {
		matches, _ := filepath.Glob("/usr/share/vulkan/icd.d/*.json")
		if len(matches) > 0 {
			h.Vulkan = "probable"
		} else {
			h.Vulkan = "unknown"
		}
	}
	return h
}

// Small local wrapper lets this file remain platform-specific only where needed.
type syscallStatfs struct {
	Type, Bsize                                       int64
	Blocks, Bfree, Bavail, Files, Ffree, Fsid0, Fsid1 uint64
	Namelen, Frsize, Flags                            int64
	Spare                                             [4]int64
}

func statfs(path string, s *syscallStatfs) error {
	// Use df instead of syscall.Statfs to keep the struct portable across Go versions.
	out, err := exec.Command("df", "-Pk", path).CombinedOutput()
	if err != nil {
		return err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return errors.New("df output")
	}
	f := strings.Fields(lines[len(lines)-1])
	if len(f) < 4 {
		return errors.New("df fields")
	}
	avail, _ := strconv.ParseUint(f[3], 10, 64)
	s.Bavail = avail
	s.Bsize = 1024
	return nil
}

func (a *App) currentRunner() (string, error) {
	p, err := filepath.EvalSymlinks(filepath.Join(a.vendorDir, "wine", "current"))
	if err != nil {
		return "", errors.New("kein kompatibler verwalteter Wine-Runner aktiv")
	}
	if _, err := os.Stat(filepath.Join(p, "bin", "wine")); err != nil {
		return "", err
	}
	return p, nil
}

func (a *App) runnerEnv(prefix string) ([]string, string, error) {
	runner, err := a.currentRunner()
	if err != nil {
		return nil, "", err
	}
	bin := filepath.Join(runner, "bin")
	env := append([]string{}, os.Environ()...)
	env = append(env,
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"WINE="+filepath.Join(bin, "wine"),
		"WINESERVER="+filepath.Join(bin, "wineserver"),
		"WINEPREFIX="+prefix,
		"WINEARCH=win64",
		"WINEDEBUG=-all",
		"WINEDLLOVERRIDES=dxwebsetup.exe,dotNetFx45_Full_setup.exe,winemenubuilder.exe=d",
	)
	return env, runner, nil
}

func (a *App) ensureLUGRuntime() (string, error) {
	appimage := filepath.Join(a.vendorDir, "lug-helper", "lug-helper.appimage")
	ver := readMetaVersion(filepath.Join(a.vendorDir, "lug-helper", "meta.json"))
	if appimage == "" || ver == "" {
		return "", errors.New("LUG AppImage runtime ist noch nicht synchronisiert")
	}
	base := filepath.Join(a.vendorDir, "lug-helper", "runtime", ver)
	if _, err := os.Stat(filepath.Join(base, "usr", "bin", "cabextract")); err == nil {
		return base, nil
	}
	if err := os.MkdirAll(filepath.Dir(base), 0o755); err != nil {
		return "", err
	}
	tmp, err := os.MkdirTemp(a.cacheDir, "lug-appimage-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	cmd := exec.Command(appimage, "--appimage-extract")
	cmd.Dir = tmp
	cmd.Env = append(os.Environ(), "APPIMAGE_EXTRACT_AND_RUN=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("LUG AppImage Runtime konnte nicht entpackt werden: %s", formatCommandFailure(err, out))
	}
	extracted := filepath.Join(tmp, "squashfs-root")
	if _, err := os.Stat(extracted); err != nil {
		return "", errors.New("AppImage-Extraktion erzeugte kein squashfs-root")
	}
	_ = os.RemoveAll(base + ".new")
	if err := copyTree(extracted, base+".new"); err != nil {
		return "", err
	}
	_ = os.RemoveAll(base)
	if err := os.Rename(base+".new", base); err != nil {
		return "", err
	}
	return base, nil
}

func (a *App) toolsEnv(base []string) ([]string, error) {
	// Prefer ordinary distro tools. Debian packages install these explicitly,
	// while most desktop distributions already provide curl/unzip.
	required := []string{"cabextract", "curl", "unzip"}
	allSystem := true
	for _, tool := range required {
		if _, err := exec.LookPath(tool); err != nil {
			allSystem = false
			break
		}
	}
	if allSystem {
		return append([]string{}, base...), nil
	}

	// Portable fallback: use only the extracted runtime of the optional LUG
	// AppImage as a toolbox. The LUG installer itself is never invoked.
	if err := a.syncLUGHelper(); err != nil {
		return base, fmt.Errorf("Systemwerkzeuge fehlen (%s) und portable Toolbox konnte nicht geladen werden: %w", strings.Join(required, ", "), err)
	}
	runtimeRoot, err := a.ensureLUGRuntime()
	if err != nil {
		return base, err
	}
	dirs := []string{filepath.Join(runtimeRoot, "usr", "bin"), filepath.Join(runtimeRoot, "bin"), filepath.Join(runtimeRoot, "usr", "sbin")}
	libdirs := []string{filepath.Join(runtimeRoot, "usr", "lib"), filepath.Join(runtimeRoot, "usr", "lib64"), filepath.Join(runtimeRoot, "lib"), filepath.Join(runtimeRoot, "lib64")}
	env := append([]string{}, base...)
	oldPath := os.Getenv("PATH")
	env = append(env, "PATH="+strings.Join(dirs, string(os.PathListSeparator))+string(os.PathListSeparator)+oldPath)
	existingLD := os.Getenv("LD_LIBRARY_PATH")
	ld := strings.Join(libdirs, string(os.PathListSeparator))
	if existingLD != "" {
		ld += string(os.PathListSeparator) + existingLD
	}
	env = append(env, "LD_LIBRARY_PATH="+ld)
	return env, nil
}

func (a *App) syncWinetricks() (string, string, error) {
	rel, err := githubLatest(winetricksRepo)
	if err != nil {
		return "", "", err
	}
	tag := rel.TagName
	if tag == "" {
		return "", "", errors.New("Winetricks-Release ohne Tag")
	}
	dir := filepath.Join(a.vendorDir, "winetricks", tag)
	target := filepath.Join(dir, "winetricks")
	if _, err := os.Stat(target); err == nil {
		return target, tag, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}
	u := "https://raw.githubusercontent.com/Winetricks/winetricks/refs/tags/" + tag + "/src/winetricks"
	if err := download(u, target+".new"); err != nil {
		return "", "", err
	}
	if err := os.Chmod(target+".new", 0o755); err != nil {
		return "", "", err
	}
	if err := os.Rename(target+".new", target); err != nil {
		return "", "", err
	}
	return target, tag, nil
}

func parseRSILatestYML(b []byte) (string, string, error) {
	var version, topPath, topSHA, firstURL, firstURLSHA string
	urlIndent := -1
	scanner := bufio.NewScanner(strings.NewReader(string(b)))
	for scanner.Scan() {
		raw := strings.TrimRight(scanner.Text(), "\r\n")
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(raw) - len(strings.TrimLeft(raw, " \t"))
		norm := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
		key, value, ok := strings.Cut(norm, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = yamlScalar(value)
		switch key {
		case "version":
			if version == "" {
				version = value
			}
		case "path":
			if indent == 0 && topPath == "" {
				topPath = value
			}
		case "url":
			if firstURL == "" {
				firstURL = value
				urlIndent = indent
			}
		case "sha512":
			if indent == 0 && topSHA == "" {
				topSHA = value
			} else if firstURL != "" && firstURLSHA == "" && indent > urlIndent {
				firstURLSHA = value
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", "", err
	}
	file := topPath
	sha := topSHA
	if file == "" {
		file = firstURL
		if sha == "" {
			sha = firstURLSHA
		}
	}
	// Electron-builder normally supplies path/url, but the version fallback
	// keeps us resilient if CIG trims redundant fields from latest.yml.
	if file == "" && version != "" {
		file = "RSI Launcher-Setup-" + version + ".exe"
	}
	if file == "" {
		return "", "", errors.New("RSI latest.yml enthält weder path, url noch version")
	}
	return file, sha, nil
}

func yamlScalar(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 2 {
		if (v[0] == '\'' && v[len(v)-1] == '\'') || (v[0] == '"' && v[len(v)-1] == '"') {
			v = v[1 : len(v)-1]
		}
	}
	return strings.TrimSpace(v)
}

func (a *App) latestRSIInstaller() (string, string, string, error) {
	req, _ := http.NewRequest("GET", rsiLatestYML, nil)
	req.Header.Set("User-Agent", "citizen-launcher/"+appVersion)
	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", "", fmt.Errorf("RSI latest.yml HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", "", "", err
	}
	file, sha, err := parseRSILatestYML(b)
	if err != nil {
		return "", "", "", err
	}
	ref, err := neturl.Parse(file)
	if err != nil {
		return "", "", "", fmt.Errorf("RSI Installer-Pfad ungültig: %w", err)
	}
	if ref.IsAbs() {
		return filepath.Base(ref.Path), ref.String(), sha, nil
	}
	base, _ := neturl.Parse(rsiBaseURL + "/")
	return filepath.Base(ref.Path), base.ResolveReference(ref).String(), sha, nil
}

func verifySHA512Base64(path, want string) error {
	if strings.TrimSpace(want) == "" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha512.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := base64.StdEncoding.EncodeToString(h.Sum(nil))
	if got != strings.TrimSpace(want) {
		return fmt.Errorf("SHA-512 mismatch for %s", filepath.Base(path))
	}
	return nil
}

func (a *App) backupIncompletePrefix(gc GameConfig) (GameConfig, string, error) {
	fi, err := os.Stat(gc.Prefix)
	if err != nil || !fi.IsDir() {
		return gc, "", nil
	}
	if prefixInitialized(gc.Prefix) {
		return gc, "", nil
	}
	if entries, err := os.ReadDir(gc.Prefix); err == nil && len(entries) == 0 {
		return gc, "", nil
	}
	stamp := time.Now().Format("20060102-150405")
	backup := gc.Prefix + "-incomplete-" + stamp
	if err := os.Rename(gc.Prefix, backup); err != nil {
		return gc, "", err
	}
	a.logf("backed up incomplete prefix %s -> %s", gc.Prefix, backup)
	return gc, backup, nil
}

func (a *App) gameInstall() error {
	unlock, err := a.acquireMaintenanceLock()
	if err != nil {
		return err
	}
	defer unlock()
	gc := a.loadGameConfig()
	hs := a.hardwareStatus(gc.Prefix)
	if hs.State == "blocked" {
		return fmt.Errorf("hardware nicht spielbereit: %s", hs.Reason)
	}

	// LUG is not part of the installation control path. A portable runtime is
	// fetched lazily only when required system tools are missing.
	if err := a.syncWineRunner(); err != nil {
		return fmt.Errorf("Wine: %w", err)
	}
	if _, _, err := a.syncWinetricks(); err != nil {
		return fmt.Errorf("Winetricks: %w", err)
	}

	// Adopt a complete existing prefix. Otherwise safely back up partial state.
	if !prefixInitialized(gc.Prefix) {
		_, backup, err := a.backupIncompletePrefix(gc)
		if err != nil {
			return err
		}
		if backup != "" {
			a.logf("partial prefix preserved at %s", backup)
		}
		if err := os.MkdirAll(gc.Prefix, 0o755); err != nil {
			return err
		}
		if err := a.initializePrefix(gc); err != nil {
			return err
		}
	}
	if err := a.ensurePrefixComponents(gc); err != nil {
		return err
	}
	if err := a.ensureRSILauncher(gc, true); err != nil {
		return err
	}
	if err := a.writeOwnedLaunchFiles(gc); err != nil {
		return err
	}
	if err := a.writeLUGCompatibilityConfig(gc); err != nil {
		a.logf("LUG compatibility config warning: %v", err)
	}
	if err := a.syncDXVK(); err != nil {
		a.logf("DXVK post-install warning: %v", err)
	}

	gc.RSIInstaller = readText(filepath.Join(a.vendorDir, "rsi", "current.txt"))
	gc.WinetricksVersion = readMetaVersion(filepath.Join(a.vendorDir, "winetricks", "meta.json"))
	gc.InstalledAt = time.Now().Format(time.RFC3339)
	if err := a.saveGameConfig(gc); err != nil {
		return err
	}
	a.logf("game stack installed prefix=%s wine=%s", gc.Prefix, readMetaVersion(filepath.Join(a.vendorDir, "wine", "meta.json")))
	return nil
}

func (a *App) initializePrefix(gc GameConfig) error {
	env, runner, err := a.runnerEnv(gc.Prefix)
	if err != nil {
		return err
	}
	env, err = a.toolsEnv(env)
	if err != nil {
		return err
	}
	bin := filepath.Join(runner, "bin")
	cmd := exec.Command(filepath.Join(bin, "wineboot"), "-u")
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Wine-Prefix konnte nicht erstellt werden: %s", formatCommandFailure(err, out))
	}
	wait := exec.Command(filepath.Join(bin, "wineserver"), "-w")
	wait.Env = env
	_, _ = wait.CombinedOutput()
	deadline := time.Now().Add(30 * time.Second)
	for !prefixInitialized(gc.Prefix) && time.Now().Before(deadline) {
		time.Sleep(300 * time.Millisecond)
	}
	if !prefixInitialized(gc.Prefix) {
		return errors.New("wineboot meldete Erfolg, aber drive_c/system.reg/user.reg fehlen")
	}
	return nil
}

func (a *App) ensurePrefixComponents(gc GameConfig) error {
	wt, tag, err := a.syncWinetricks()
	if err != nil {
		return err
	}
	env, _, err := a.runnerEnv(gc.Prefix)
	if err != nil {
		return err
	}
	env, err = a.toolsEnv(env)
	if err != nil {
		return err
	}
	cache := filepath.Join(a.cacheDir, "winetricks-cache", tag)
	_ = os.MkdirAll(cache, 0o755)
	env = append(env, "W_CACHE="+cache, "WINETRICKS_DOWNLOADER=curl")

	for _, verb := range basePrefixWinetricksVerbs {
		cmd := exec.Command(wt, "-q", verb)
		cmd.Env = env
		out, runErr := cmd.CombinedOutput()
		a.logf("winetricks %s verb=%s output=%s", tag, verb, compactDiagnostic(string(out)))
		if runErr != nil {
			return fmt.Errorf("Windows-Komponente %s: %s", verb, formatCommandFailure(runErr, out))
		}
	}

	runner, _ := a.currentRunner()
	wine := filepath.Join(runner, "bin", "wine")
	reg := exec.Command(wine, "reg", "add", `HKEY_CURRENT_USER\\Software\\Wine\\FileOpenAssociations`, `/v`, `Enable`, `/d`, `N`, `/f`)
	reg.Env = env
	out, err := reg.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Wine Registry: %s", formatCommandFailure(err, out))
	}
	_ = writeMeta(filepath.Join(a.vendorDir, "winetricks", "meta.json"), componentMeta{Version: tag, Updated: time.Now().Format(time.RFC3339)})
	return nil
}

func (a *App) ensureRSILauncher(gc GameConfig, forceIfMissing bool) error {
	file, u, sha, err := a.latestRSIInstaller()
	if err != nil {
		return err
	}
	metaDir := filepath.Join(a.vendorDir, "rsi")
	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		return err
	}
	old := readText(filepath.Join(metaDir, "current.txt"))
	_, launcherErr := os.Stat(gc.LauncherEXE)
	if launcherErr == nil && old == file && !forceIfMissing {
		return nil
	}
	if launcherErr == nil && old == file {
		return nil
	}
	cache := filepath.Join(a.cacheDir, file)
	if _, err := os.Stat(cache); err != nil {
		if err := download(u, cache); err != nil {
			return err
		}
	}
	if err := verifySHA512Base64(cache, sha); err != nil {
		_ = os.Remove(cache)
		return err
	}
	env, runner, err := a.runnerEnv(gc.Prefix)
	if err != nil {
		return err
	}
	wine := filepath.Join(runner, "bin", "wine")
	cmd := exec.Command(wine, cache, "/S")
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	a.logf("RSI installer %s output=%s", file, compactDiagnostic(string(out)))
	if err != nil {
		return fmt.Errorf("RSI Launcher Installation: %s", formatCommandFailure(err, out))
	}
	ws := exec.Command(filepath.Join(runner, "bin", "wineserver"), "-k")
	ws.Env = env
	_, _ = ws.CombinedOutput()
	deadline := time.Now().Add(20 * time.Second)
	for {
		if _, err := os.Stat(gc.LauncherEXE); err == nil {
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("RSI Installer beendet, Launcher fehlt: %s", gc.LauncherEXE)
		}
		time.Sleep(500 * time.Millisecond)
	}
	if err := os.WriteFile(filepath.Join(metaDir, "current.txt"), []byte(file+"\n"), 0o644); err != nil {
		return err
	}
	return nil
}

func (a *App) writeOwnedLaunchFiles(gc GameConfig) error {
	binDir := filepath.Join(a.dataDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return err
	}
	script := filepath.Join(binDir, "star-citizen-launch")
	backend := filepath.Join(a.libDir, "citizen-launcher")
	content := "#!/usr/bin/env bash\nexec " + shellQuote(backend) + " game-launch\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		return err
	}
	apps := filepath.Join(envOr("XDG_DATA_HOME", filepath.Join(a.home, ".local", "share")), "applications")
	_ = os.MkdirAll(apps, 0o755)
	desktop := `[Desktop Entry]
Name=Star Citizen
Comment=Star Citizen via Citizen Launcher
Type=Application
Categories=Game;
Terminal=false
StartupNotify=true
StartupWMClass=rsi launcher.exe
Exec=` + script + `\n`
	if err := os.WriteFile(filepath.Join(apps, "citizen-launcher-star-citizen.desktop"), []byte(desktop), 0o644); err != nil {
		return err
	}
	return nil
}

func (a *App) writeLUGCompatibilityConfig(gc GameConfig) error {
	dir := filepath.Join(envOr("XDG_CONFIG_HOME", filepath.Join(a.home, ".config")), "starcitizen-lug")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "winedir.conf"), []byte(gc.Prefix+"\n"), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "gamedir.conf"), []byte(gc.GameDir+"\n"), 0o644); err != nil {
		return err
	}
	return nil
}

func (a *App) gameLaunch() error {
	gc := a.loadGameConfig()
	st := a.gameStatus()
	// On non-systemd distributions (or a stopped user timer), maintenance still
	// happens opportunistically when the player launches the game. Never block
	// the launch on network/update work.
	if cfg := a.loadConfig(); cfg.AutoMaintain {
		stale := true
		if t, err := time.Parse(time.RFC3339, cfg.LastMaintenance); err == nil {
			stale = time.Since(t) > 6*time.Hour
		}
		if stale && a.selfPath != "" {
			_ = exec.Command(a.selfPath, "maintain").Start()
		}
	}
	if st.Hardware == "blocked" {
		return errors.New(st.HardwareReason)
	}
	if !prefixInitialized(gc.Prefix) {
		return errors.New("Wine-Prefix fehlt; zuerst Setup ausführen")
	}
	if _, err := os.Stat(gc.LauncherEXE); err != nil {
		return errors.New("RSI Launcher fehlt; Reparatur ausführen")
	}
	env, runner, err := a.runnerEnv(gc.Prefix)
	if err != nil {
		return err
	}
	env = append(env,
		"__GL_SHADER_DISK_CACHE=1",
		"__GL_SHADER_DISK_CACHE_SIZE=10737418240",
		"__GL_SHADER_DISK_CACHE_PATH="+gc.Prefix,
		"MESA_SHADER_CACHE_DIR="+gc.Prefix,
		"MESA_SHADER_CACHE_MAX_SIZE=10G",
	)
	log := filepath.Join(a.stateDir, "rsi-launcher.log")
	_ = os.MkdirAll(a.stateDir, 0o755)
	f, err := os.OpenFile(log, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	cmd := exec.Command(filepath.Join(runner, "bin", "wine"), gc.LauncherEXE)
	cmd.Env = env
	cmd.Stdout = f
	cmd.Stderr = f
	if err := cmd.Start(); err != nil {
		f.Close()
		return err
	}
	_ = f.Close()
	a.logf("RSI Launcher started pid=%d", cmd.Process.Pid)
	return nil
}

func (a *App) gameRepair() error {
	gc := a.loadGameConfig()
	hs := a.hardwareStatus(gc.Prefix)
	if hs.State == "blocked" {
		return errors.New(hs.Reason)
	}
	if err := a.syncWineRunner(); err != nil {
		return err
	}
	if !prefixInitialized(gc.Prefix) {
		return a.gameInstall()
	}
	if err := a.ensurePrefixComponents(gc); err != nil {
		return err
	}
	if err := a.ensureRSILauncher(gc, false); err != nil {
		return err
	}
	if err := a.syncDXVK(); err != nil {
		return fmt.Errorf("DXVK: %w", err)
	}
	if err := a.writeOwnedLaunchFiles(gc); err != nil {
		return err
	}
	gc.LastRepair = time.Now().Format(time.RFC3339)
	_ = a.saveGameConfig(gc)
	return nil
}

func (a *App) syncRSILauncherIfNeeded() error {
	gc := a.loadGameConfig()
	if !prefixInitialized(gc.Prefix) || prefixBusy(gc.Prefix) {
		return nil
	}
	return a.ensureRSILauncher(gc, false)
}

func readText(path string) string { b, _ := os.ReadFile(path); return strings.TrimSpace(string(b)) }
func shellQuote(s string) string  { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }

// Keep sort referenced here because installation diagnostics intentionally sort paths in future additions.
var _ = sort.Strings
