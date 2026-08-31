package main

import (
	"archive/zip"
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
	"syscall"
	"time"
)

const (
	winetricksVersion = "20260125"
	winetricksSHA256  = "431f82fc74000e6c864409f1d8fb495d696c03928808e3e8acffc45179312a7b"
	rsiBaseURL        = "https://install.robertsspaceindustries.com/rel/2"
	rsiLatestYML      = rsiBaseURL + "/latest.yml"
)

// Keep Winetricks focused on small, proven prefix tweaks. PowerShell is managed
// separately below because the MSI-based Winetricks path has failed on otherwise
// healthy modern Wine builds. We install the official portable PowerShell Core
// ZIP plus the RSI-compatible PowerShell wrapper instead.
var basePrefixWinetricksVerbs = []string{"arial", "tahoma", "win11"}

const (
	wineFileAssociationsKey  = "HKEY_CURRENT_USER\\Software\\Wine\\FileOpenAssociations"
	powerShellCoreVersion    = "7.4.19"
	powerShellCoreURL        = "https://github.com/PowerShell/PowerShell/releases/download/v7.4.19/PowerShell-7.4.19-win-x64.zip"
	powerShellCoreSHA256     = "cd62ad6d8174cc6fb85b335a0058444bc934fe27c39fa97fe342134286d28af9"
	powerShellWrapperVersion = "3.0.5"
	powerShellWrapperURL     = "https://github.com/ProjectSynchro/powershell-wrapper-for-wine/releases/download/v3.0.5/powershell-wrapper.zip"
	powerShellWrapperSHA256  = "08f866265e0395f4bc5ddb18f3dff345771d039a62911e506c62084fd2533ec3"
)

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
	BackendVersion  string          `json:"backend_version"`
	Health          string          `json:"health"`
	Hardware        string          `json:"hardware"`
	HardwareReason  string          `json:"hardware_reason,omitempty"`
	GPU             string          `json:"gpu,omitempty"`
	Vulkan          string          `json:"vulkan,omitempty"`
	CPUAVX          bool            `json:"cpu_avx"`
	RAMGiB          int             `json:"ram_gib"`
	CombinedGiB     int             `json:"combined_gib"`
	DiskFreeGiB     int             `json:"disk_free_gib"`
	Prefix          string          `json:"prefix"`
	PrefixState     string          `json:"prefix_state"`
	LauncherState   string          `json:"launcher_state"`
	GameState       string          `json:"game_state"`
	WineVersion     string          `json:"wine_version,omitempty"`
	DXVKVersion     string          `json:"dxvk_version,omitempty"`
	DXVKState       string          `json:"dxvk_state"`
	PowerShellState string          `json:"powershell_state"`
	LUGVersion      string          `json:"lug_version,omitempty"`
	RSIInstaller    string          `json:"rsi_installer,omitempty"`
	Autopilot       bool            `json:"autopilot"`
	System          SystemReadiness `json:"system"`
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
	return atomicWriteFile(a.gameConfigPath(), data, 0o600)
}

func (a *App) gameStatus() GameStatus {
	gc := a.loadGameConfig()
	uc := a.loadConfig()
	hs := a.hardwareStatus(gc.Prefix)
	sys := a.systemReadiness(gc.Prefix)
	st := GameStatus{
		BackendVersion:  appVersion,
		Health:          "setup",
		Hardware:        hs.State,
		HardwareReason:  hs.Reason,
		GPU:             hs.GPU,
		Vulkan:          hs.Vulkan,
		CPUAVX:          hs.AVX,
		RAMGiB:          hs.RAMGiB,
		CombinedGiB:     hs.CombinedGiB,
		DiskFreeGiB:     hs.DiskFreeGiB,
		Prefix:          gc.Prefix,
		PrefixState:     "missing",
		LauncherState:   "missing",
		GameState:       "missing",
		WineVersion:     readMetaVersion(filepath.Join(a.vendorDir, "wine", "meta.json")),
		DXVKVersion:     readMetaVersion(filepath.Join(a.vendorDir, "dxvk", "meta.json")),
		DXVKState:       "missing",
		PowerShellState: "missing",
		LUGVersion:      readMetaVersion(filepath.Join(a.vendorDir, "lug-helper", "meta.json")),
		RSIInstaller:    gc.RSIInstaller,
		Autopilot:       uc.AutoMaintain,
		System:          sys,
	}
	// Always inspect the existing stack before deciding the overall health.
	// Support bundles must remain truthful even when hardware/preflight is
	// blocked; otherwise a perfectly valid prefix/launcher appears as missing.
	prefixPathErr := validatePrefixPath(gc.Prefix)
	if prefixInitialized(gc.Prefix) {
		st.PrefixState = "ready"
	} else if fi, err := os.Stat(gc.Prefix); err == nil && fi.IsDir() {
		st.PrefixState = "partial"
	}
	if st.PrefixState == "ready" {
		st.DXVKState = a.dxvkState(gc.Prefix, st.DXVKVersion)
		st.PowerShellState = a.powerShellState(gc.Prefix)
	}
	if _, err := os.Stat(gc.LauncherEXE); err == nil {
		st.LauncherState = "ready"
	}
	if _, err := os.Stat(filepath.Join(gc.GameDir, "LIVE", "Bin64", "StarCitizen.exe")); err == nil {
		st.GameState = "ready"
	} else if _, err := os.Stat(filepath.Join(gc.GameDir, "LIVE", "Data.p4k")); err == nil {
		st.GameState = "partial"
	}

	// Health precedence is evaluated only after component discovery so diagnosis
	// never loses useful state.
	if prefixPathErr != nil {
		st.Health = "hardware-blocked"
		st.HardwareReason = prefixPathErr.Error()
		return st
	}
	if hs.State == "blocked" {
		st.Health = "hardware-blocked"
		return st
	}
	if sys.State == "blocked" {
		st.Health = "hardware-blocked"
		st.HardwareReason = sys.Reason
		return st
	}

	// Around 150 GiB of free space is the current LUG quick-start recommendation
	// for a fresh install. Do not block an already-installed game because free
	// space naturally drops after installation.
	if st.GameState == "missing" && sys.StorageFreeGiB > 0 && sys.StorageFreeGiB < 150 {
		st.Health = "hardware-blocked"
		st.HardwareReason = fmt.Sprintf("Für eine Neuinstallation sind ungefähr 150 GiB freier Speicher empfohlen; erkannt wurden %d GiB.", sys.StorageFreeGiB)
		return st
	}

	switch {
	case sys.State == "prepare":
		st.Health = "system-prepare"
	case st.WineVersion == "":
		st.Health = "setup"
	case st.PrefixState == "partial":
		st.Health = "repair"
	case st.PrefixState != "ready":
		st.Health = "install"
	case st.DXVKState != "ready":
		st.Health = "repair"
	case st.PowerShellState != "ready":
		st.Health = "repair"
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

type vulkanDevice struct {
	Name     string
	Type     string
	APIMajor int
	APIMinor int
}

func parseVulkanDevices(summary string) []vulkanDevice {
	var devices []vulkanDevice
	var current *vulkanDevice
	flush := func() {
		if current != nil && (current.Name != "" || current.Type != "" || current.APIMajor != 0) {
			devices = append(devices, *current)
		}
	}
	apiRE := regexp.MustCompile(`^([0-9]+)\.([0-9]+)`)
	gpuRE := regexp.MustCompile(`^GPU[0-9]+:$`)
	for _, raw := range strings.Split(summary, "\n") {
		line := strings.TrimSpace(raw)
		if gpuRE.MatchString(line) {
			flush()
			current = &vulkanDevice{}
			continue
		}
		if current == nil {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key, val = strings.TrimSpace(key), strings.TrimSpace(val)
		switch key {
		case "deviceName":
			current.Name = val
		case "deviceType":
			current.Type = val
		case "apiVersion":
			if m := apiRE.FindStringSubmatch(val); len(m) == 3 {
				current.APIMajor, _ = strconv.Atoi(m[1])
				current.APIMinor, _ = strconv.Atoi(m[2])
			}
		}
	}
	flush()
	return devices
}

func softwareVulkanDevice(d vulkanDevice) bool {
	low := strings.ToLower(d.Name + " " + d.Type)
	return strings.Contains(low, "physical_device_type_cpu") ||
		strings.Contains(low, "llvmpipe") || strings.Contains(low, "lavapipe") || strings.Contains(low, "softpipe")
}

func vulkanDeviceRank(d vulkanDevice) int {
	if softwareVulkanDevice(d) {
		return -1
	}
	switch strings.ToUpper(d.Type) {
	case "PHYSICAL_DEVICE_TYPE_DISCRETE_GPU":
		return 30
	case "PHYSICAL_DEVICE_TYPE_INTEGRATED_GPU":
		return 20
	case "PHYSICAL_DEVICE_TYPE_VIRTUAL_GPU":
		return 10
	default:
		if d.Name != "" {
			return 1
		}
	}
	return 0
}

func selectVulkanHardwareDevice(summary string) (vulkanDevice, bool) {
	best, rank := vulkanDevice{}, -1
	for _, d := range parseVulkanDevices(summary) {
		r := vulkanDeviceRank(d)
		if r > rank {
			best, rank = d, r
		}
	}
	return best, rank >= 0
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
		if h.RAMGiB < 16 && h.State != "blocked" {
			h.State = "blocked"
			h.Reason = "Weniger als 16 GiB RAM erkannt."
		} else if h.CombinedGiB < 48 && h.State != "blocked" {
			h.State = "blocked"
			h.Reason = fmt.Sprintf("RAM + Swap/ZRAM ergeben %d GiB; für Star Citizen werden mindestens 48 GiB benötigt.", h.CombinedGiB)
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
		// lspci/pciutils is optional on minimal installations. Detect QEMU VGA
		// through sysfs as a dependency-free fallback.
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
	qemuFallback := strings.Contains(strings.ToLower(p), "1234:1111") || strings.Contains(strings.ToLower(p), "kernel driver in use: bochs")

	if path, err := exec.LookPath("vulkaninfo"); err == nil {
		cmd := exec.Command(path, "--summary")
		out, err := cmd.CombinedOutput()
		text := string(out)
		if err != nil || strings.Contains(text, "VK_ERROR_INCOMPATIBLE_DRIVER") {
			h.State = "blocked"
			h.Vulkan = "blocked"
			h.Reason = "Vulkan ist nicht funktionsfähig."
			return h
		}
		dev, hasHardware := selectVulkanHardwareDevice(text)
		if !hasHardware {
			h.State = "blocked"
			h.Vulkan = "blocked"
			if qemuFallback {
				h.Reason = "QEMU Standard VGA/bochs-drm erkannt und keine echte Vulkan-GPU verfügbar. Für Star Citizen ist GPU-Passthrough erforderlich."
			} else {
				h.Reason = "Es wurde nur ein Software-Vulkan-Gerät erkannt. Star Citizen benötigt eine echte Vulkan-GPU."
			}
			return h
		}
		if dev.Name != "" {
			h.GPU = dev.Name
		}
		if dev.APIMajor < 1 || (dev.APIMajor == 1 && dev.APIMinor < 3) {
			h.State = "blocked"
			h.Vulkan = "blocked"
			h.Reason = fmt.Sprintf("Vulkan %d.%d erkannt; der aktuelle DXVK-Stack benötigt Vulkan 1.3 oder neuer.", dev.APIMajor, dev.APIMinor)
			return h
		}
		h.Vulkan = "ready"
	} else {
		// Without vulkaninfo we can only make a provisional assessment. QEMU's
		// bochs framebuffer is still a hard block because it is not a Vulkan GPU.
		if qemuFallback {
			h.State = "blocked"
			h.Vulkan = "blocked"
			h.Reason = "QEMU Standard VGA/bochs-drm erkannt. Star Citizen benötigt eine echte Vulkan-GPU oder GPU-Passthrough."
			return h
		}
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

func envValue(env []string, key string) string {
	for i := len(env) - 1; i >= 0; i-- {
		k, v, ok := strings.Cut(env[i], "=")
		if ok && k == key {
			return v
		}
	}
	return ""
}

func setEnvValue(env []string, key, value string) []string {
	env = environmentWithout(env, key)
	return append(env, key+"="+value)
}

func environmentWithout(env []string, keys ...string) []string {
	remove := map[string]bool{}
	for _, k := range keys {
		remove[k] = true
	}
	out := make([]string, 0, len(env))
	for _, item := range env {
		k, _, ok := strings.Cut(item, "=")
		if ok && remove[k] {
			continue
		}
		out = append(out, item)
	}
	return out
}

func (a *App) runnerEnv(prefix string) ([]string, string, error) {
	runner, err := a.currentRunner()
	if err != nil {
		return nil, "", err
	}
	bin := filepath.Join(runner, "bin")
	env := environmentWithout(os.Environ(), "SDL_VIDEODRIVER", "WINE", "WINESERVER", "WINEPREFIX", "WINEARCH", "WINEDEBUG", "WINEDLLOVERRIDES")
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
	oldPath := envValue(env, "PATH")
	newPath := strings.Join(dirs, string(os.PathListSeparator))
	if oldPath != "" {
		newPath += string(os.PathListSeparator) + oldPath
	}
	env = setEnvValue(env, "PATH", newPath)
	existingLD := envValue(env, "LD_LIBRARY_PATH")
	ld := strings.Join(libdirs, string(os.PathListSeparator))
	if existingLD != "" {
		ld += string(os.PathListSeparator) + existingLD
	}
	env = setEnvValue(env, "LD_LIBRARY_PATH", ld)
	return env, nil
}

func (a *App) syncWinetricks() (string, string, error) {
	// Pin the exact Winetricks build used by this Citizen Launcher release.
	// Gaming-stack updates should be deterministic: a future Winetricks release
	// must first pass our regression tests before a Launcher release adopts it.
	tag := winetricksVersion
	dir := filepath.Join(a.vendorDir, "winetricks", tag)
	target := filepath.Join(dir, "winetricks")
	if err := verifySHA256Hex(target, winetricksSHA256); err == nil {
		_ = os.Chmod(target, 0o755)
		return target, tag, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}
	u := "https://raw.githubusercontent.com/Winetricks/winetricks/refs/tags/" + tag + "/src/winetricks"
	tmp := target + ".new"
	_ = os.Remove(tmp)
	if err := download(u, tmp); err != nil {
		return "", "", err
	}
	if err := verifySHA256Hex(tmp, winetricksSHA256); err != nil {
		_ = os.Remove(tmp)
		return "", "", fmt.Errorf("Winetricks Integritätsprüfung: %w", err)
	}
	if err := os.Chmod(tmp, 0o755); err != nil {
		_ = os.Remove(tmp)
		return "", "", err
	}
	if err := os.Rename(tmp, target); err != nil {
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
	lock, err := a.stackOperationLock(2 * time.Second)
	if err != nil {
		return err
	}
	defer releaseFileLock(lock)
	return a.gameInstallUnlocked()
}

func (a *App) gameInstallUnlocked() error {
	gc := a.loadGameConfig()
	if err := validatePrefixPath(gc.Prefix); err != nil {
		return err
	}
	if prefixInitialized(gc.Prefix) && prefixBusy(gc.Prefix) {
		return errors.New("Der Star-Citizen-Wine-Prefix wird gerade verwendet. Bitte RSI Launcher, Wine-Konfiguration und Spiel schließen und erneut versuchen.")
	}
	hs := a.hardwareStatus(gc.Prefix)
	if hs.State == "blocked" {
		return fmt.Errorf("hardware nicht spielbereit: %s", hs.Reason)
	}
	if sys := a.systemReadiness(gc.Prefix); sys.State == "blocked" {
		return errors.New(sys.Reason)
	} else if sys.State == "prepare" {
		if err := a.ensureSystemPrepared(); err != nil {
			return fmt.Errorf("Systemvorbereitung: %w", err)
		}
	}
	if sys := a.systemReadiness(gc.Prefix); sys.StorageFreeGiB > 0 && sys.StorageFreeGiB < 150 {
		return fmt.Errorf("zu wenig freier Speicher: %d GiB; für eine Neuinstallation werden ungefähr 150 GiB empfohlen", sys.StorageFreeGiB)
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
	if err := a.syncDXVK(); err != nil {
		return fmt.Errorf("DXVK: %w", err)
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
	if waitOut, waitErr := wait.CombinedOutput(); waitErr != nil {
		return fmt.Errorf("Wine-Prefix Abschluss: %s", formatCommandFailure(waitErr, waitOut))
	}
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

	// RSI Launcher 2.x uses Windows PowerShell for installer support, game
	// directory setup and verification. Avoid the fragile MSI path entirely:
	// install a verified portable PowerShell Core plus the Wine wrapper.
	if err := a.ensurePowerShell(gc, env); err != nil {
		return fmt.Errorf("PowerShell-Kompatibilität: %w", err)
	}

	runner, _ := a.currentRunner()
	wine := filepath.Join(runner, "bin", "wine")
	// This is a convenience tweak only: prevent Wine from creating host file
	// associations. It must never make the Star Citizen installation fail.
	// Use a normal Go string so Wine receives single registry separators.
	reg := exec.Command(wine, "reg", "add", wineFileAssociationsKey, "/v", "Enable", "/t", "REG_SZ", "/d", "N", "/f")
	reg.Env = env
	out, regErr := reg.CombinedOutput()
	if regErr != nil {
		a.logf("non-fatal Wine registry association tweak warning: %s", formatCommandFailure(regErr, out))
	}
	// Let Wine finish registry writes before DXVK changes the same prefix.
	wait := exec.Command(filepath.Join(runner, "bin", "wineserver"), "-w")
	wait.Env = env
	if waitOut, waitErr := wait.CombinedOutput(); waitErr != nil {
		return fmt.Errorf("Wine-Prefix Abschluss: %s", formatCommandFailure(waitErr, waitOut))
	}
	_ = writeMeta(filepath.Join(a.vendorDir, "winetricks", "meta.json"), componentMeta{Version: tag, Updated: time.Now().Format(time.RFC3339)})
	return nil
}

func (a *App) powerShellPaths(prefix string) (core, profile, wrapper64, wrapper32 string) {
	coreDir := filepath.Join(prefix, "drive_c", "Program Files", "PowerShell", "7")
	return filepath.Join(coreDir, "pwsh.exe"), filepath.Join(coreDir, "profile.ps1"),
		filepath.Join(prefix, "drive_c", "windows", "system32", "WindowsPowerShell", "v1.0", "powershell.exe"),
		filepath.Join(prefix, "drive_c", "windows", "syswow64", "WindowsPowerShell", "v1.0", "powershell.exe")
}

func (a *App) powerShellState(prefix string) string {
	core, profile, wrapper64, wrapper32 := a.powerShellPaths(prefix)
	for _, path := range []string{core, profile, wrapper64, wrapper32} {
		fi, err := os.Stat(path)
		if err != nil || fi.IsDir() || fi.Size() < 1024 {
			return "repair"
		}
	}
	marker := readText(filepath.Join(a.vendorDir, "powershell", "marker.txt"))
	managed := powerShellCoreVersion + "+wrapper-" + powerShellWrapperVersion + "\n" + prefix
	adopted := "verified\n" + prefix
	if marker != managed && marker != adopted {
		return "repair"
	}
	return "ready"
}

func (a *App) writePowerShellMarker(prefix, version string) error {
	markerDir := filepath.Join(a.vendorDir, "powershell")
	if err := os.MkdirAll(markerDir, 0o755); err != nil {
		return err
	}
	return atomicWriteFile(filepath.Join(markerDir, "marker.txt"), []byte(version+"\n"+prefix+"\n"), 0o600)
}

func (a *App) probePowerShell(gc GameConfig, env []string) error {
	core, _, wrapper64, _ := a.powerShellPaths(gc.Prefix)
	runner, err := a.currentRunner()
	if err != nil {
		return err
	}
	wine := filepath.Join(runner, "bin", "wine")

	// Validate PowerShell Core directly first. Then validate the compatibility
	// wrapper using the invocation shape documented by the wrapper project.
	// Wine does not reliably forward stdout from a Windows child spawned by
	// another Windows executable, while wrapper v3+ deliberately propagates the
	// child's exit status. A successful exit is therefore authoritative.
	coreProbe := exec.Command(wine, core, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", "exit 0")
	coreProbe.Env = env
	if out, err := coreProbe.CombinedOutput(); err != nil {
		return fmt.Errorf("PowerShell Core Selbsttest: %s", formatCommandFailure(err, out))
	}
	wrapperProbe := exec.Command(wine, wrapper64, "-NoLogo", "-NonInteractive", "exit 0")
	wrapperProbe.Env = env
	if out, err := wrapperProbe.CombinedOutput(); err != nil {
		return fmt.Errorf("PowerShell Wrapper Selbsttest: %s", formatCommandFailure(err, out))
	}
	wait := exec.Command(filepath.Join(runner, "bin", "wineserver"), "-w")
	wait.Env = env
	if out, err := wait.CombinedOutput(); err != nil {
		return fmt.Errorf("PowerShell Abschluss: %s", formatCommandFailure(err, out))
	}
	return nil
}

func verifySHA256Hex(path, want string) error {
	got, err := fileHash(path)
	if err != nil {
		return err
	}
	if !strings.EqualFold(strings.TrimSpace(got), strings.TrimSpace(want)) {
		return fmt.Errorf("SHA-256 mismatch for %s", filepath.Base(path))
	}
	return nil
}

func extractZipSafe(path, dir string) error {
	r, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer r.Close()
	cleanRoot, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cleanRoot, 0o755); err != nil {
		return err
	}
	for _, zf := range r.File {
		name := filepath.Clean(filepath.FromSlash(zf.Name))
		if name == "." || filepath.IsAbs(name) || name == ".." || strings.HasPrefix(name, ".."+string(os.PathSeparator)) {
			return fmt.Errorf("unsicherer ZIP-Pfad: %q", zf.Name)
		}
		target := filepath.Join(cleanRoot, name)
		rel, err := filepath.Rel(cleanRoot, target)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return fmt.Errorf("ZIP-Pfad verlässt Zielverzeichnis: %q", zf.Name)
		}
		if zf.FileInfo().Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("Symlink im ZIP wird nicht akzeptiert: %q", zf.Name)
		}
		if zf.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := zf.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			rc.Close()
			return err
		}
		_, copyErr := io.Copy(out, io.LimitReader(rc, 512<<20))
		closeErr := out.Close()
		rc.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func (a *App) ensurePowerShell(gc GameConfig, env []string) error {
	if a.powerShellState(gc.Prefix) == "ready" {
		return nil
	}
	// Adopt a working existing Winetricks/manual installation instead of
	// overwriting it merely because Citizen Launcher did not create it.
	core, profile, wrapper64Existing, wrapper32Existing := a.powerShellPaths(gc.Prefix)
	allPresent := true
	for _, path := range []string{core, profile, wrapper64Existing, wrapper32Existing} {
		fi, err := os.Stat(path)
		if err != nil || fi.IsDir() || fi.Size() < 1024 {
			allPresent = false
			break
		}
	}
	if allPresent {
		if err := a.probePowerShell(gc, env); err == nil {
			if err := a.writePowerShellMarker(gc.Prefix, "verified"); err != nil {
				return err
			}
			a.logf("adopted existing working PowerShell compatibility layer")
			return nil
		}
	}

	cacheDir := filepath.Join(a.cacheDir, "powershell")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return err
	}
	coreArchive := filepath.Join(cacheDir, "PowerShell-"+powerShellCoreVersion+"-win-x64.zip")
	wrapperArchive := filepath.Join(cacheDir, "powershell-wrapper-"+powerShellWrapperVersion+".zip")
	for _, asset := range []struct{ url, path, sha string }{
		{powerShellCoreURL, coreArchive, powerShellCoreSHA256},
		{powerShellWrapperURL, wrapperArchive, powerShellWrapperSHA256},
	} {
		if err := verifySHA256Hex(asset.path, asset.sha); err != nil {
			_ = os.Remove(asset.path)
			if err := download(asset.url, asset.path+".part"); err != nil {
				_ = os.Remove(asset.path + ".part")
				return err
			}
			if err := verifySHA256Hex(asset.path+".part", asset.sha); err != nil {
				_ = os.Remove(asset.path + ".part")
				return err
			}
			if err := os.Rename(asset.path+".part", asset.path); err != nil {
				return err
			}
		}
	}

	stage, err := os.MkdirTemp(a.cacheDir, "powershell-stage-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	coreStage := filepath.Join(stage, "core")
	wrapperStage := filepath.Join(stage, "wrapper")
	if err := extractZipSafe(coreArchive, coreStage); err != nil {
		return fmt.Errorf("PowerShell Core entpacken: %w", err)
	}
	if err := extractZipSafe(wrapperArchive, wrapperStage); err != nil {
		return fmt.Errorf("PowerShell Wrapper entpacken: %w", err)
	}
	if _, err := os.Stat(filepath.Join(coreStage, "pwsh.exe")); err != nil {
		return errors.New("PowerShell Core ZIP enthält pwsh.exe nicht")
	}

	coreTarget := filepath.Join(gc.Prefix, "drive_c", "Program Files", "PowerShell", "7")
	_, _, wrapper64Target, wrapper32Target := a.powerShellPaths(gc.Prefix)
	managedTargets := []string{coreTarget, wrapper64Target, wrapper32Target}
	backups := make(map[string]string, len(managedTargets))
	for _, target := range managedTargets {
		backup := target + ".citizen-backup"
		_ = os.RemoveAll(backup)
		if _, err := os.Lstat(target); err == nil {
			if err := os.Rename(target, backup); err != nil {
				for original, saved := range backups {
					_ = os.Rename(saved, original)
				}
				return fmt.Errorf("vorhandene PowerShell-Datei sichern: %w", err)
			}
			backups[target] = backup
		}
	}
	restore := true
	defer func() {
		if !restore {
			return
		}
		for _, target := range managedTargets {
			_ = os.RemoveAll(target)
		}
		for original, saved := range backups {
			_ = os.Rename(saved, original)
		}
	}()
	if err := copyTree(coreStage, coreTarget); err != nil {
		return fmt.Errorf("PowerShell Core installieren: %w", err)
	}

	findWrapper := func(bits string) (string, error) {
		candidates := []string{
			filepath.Join(wrapperStage, bits, "powershell.exe"),
			filepath.Join(wrapperStage, "powershell"+bits+".exe"),
		}
		for _, p := range candidates {
			if fi, err := os.Stat(p); err == nil && !fi.IsDir() && fi.Size() > 1024 {
				return p, nil
			}
		}
		return "", fmt.Errorf("%s-bit PowerShell Wrapper fehlt", bits)
	}
	wrapper64, err := findWrapper("64")
	if err != nil {
		return err
	}
	wrapper32, err := findWrapper("32")
	if err != nil {
		return err
	}
	profileSource := filepath.Join(wrapperStage, "profile.ps1")
	if _, err := os.Stat(profileSource); err != nil {
		return errors.New("PowerShell Wrapper profile.ps1 fehlt")
	}
	for _, pair := range [][2]string{{wrapper64, wrapper64Target}, {wrapper32, wrapper32Target}, {profileSource, filepath.Join(coreTarget, "profile.ps1")}} {
		if err := os.MkdirAll(filepath.Dir(pair[1]), 0o755); err != nil {
			return err
		}
		if err := copyFile(pair[0], pair[1], 0o644); err != nil {
			return err
		}
	}

	runner, err := a.currentRunner()
	if err != nil {
		return err
	}
	wine := filepath.Join(runner, "bin", "wine")
	reg := exec.Command(wine, "reg", "add", `HKEY_CURRENT_USER\Software\Wine\DllOverrides`, "/v", "powershell.exe", "/t", "REG_SZ", "/d", "native,builtin", "/f")
	reg.Env = env
	if out, err := reg.CombinedOutput(); err != nil {
		return fmt.Errorf("PowerShell DLL-Override: %s", formatCommandFailure(err, out))
	}
	wait := exec.Command(filepath.Join(runner, "bin", "wineserver"), "-w")
	wait.Env = env
	if out, err := wait.CombinedOutput(); err != nil {
		return fmt.Errorf("PowerShell Registry Abschluss: %s", formatCommandFailure(err, out))
	}
	if err := a.probePowerShell(gc, env); err != nil {
		return err
	}
	if err := a.writePowerShellMarker(gc.Prefix, powerShellCoreVersion+"+wrapper-"+powerShellWrapperVersion); err != nil {
		return err
	}
	restore = false
	for _, saved := range backups {
		_ = os.RemoveAll(saved)
	}
	a.logf("PowerShell compatibility ready core=%s wrapper=%s", powerShellCoreVersion, powerShellWrapperVersion)
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
	stable := a.stableExecutable()
	script := filepath.Join(binDir, "star-citizen-launch")
	content := "#!/usr/bin/env bash\nexec " + shellQuote(stable) + " game-launch \"$@\"\n"
	if err := atomicWriteFile(script, []byte(content), 0o755); err != nil {
		return err
	}

	apps := a.userApplicationsDir()
	if err := os.MkdirAll(apps, 0o755); err != nil {
		return err
	}
	desktop := "[Desktop Entry]\n" +
		"Name=Star Citizen\n" +
		"Comment=Star Citizen via Citizen Launcher\n" +
		"Type=Application\n" +
		"Categories=Game;\n" +
		"Terminal=false\n" +
		"StartupNotify=true\n" +
		"StartupWMClass=RSI Launcher.exe\n" +
		"Icon=citizen-launcher\n" +
		"TryExec=" + stable + "\n" +
		"Exec=" + desktopExecQuote(stable) + " game-launch\n"
	if strings.Contains(desktop, `\\n`) {
		return errors.New("internal desktop-entry newline error")
	}
	if err := atomicWriteFile(a.starDesktopPath(), []byte(desktop), 0o644); err != nil {
		return err
	}
	if commandExists("update-desktop-database") {
		_ = exec.Command("update-desktop-database", apps).Run()
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
	if err := validatePrefixPath(gc.Prefix); err != nil {
		return err
	}
	state := a.prefixProcessState(gc.Prefix)
	if state.Game {
		return ErrGameAlreadyRunning
	}
	if state.RSI {
		return ErrLauncherAlreadyRunning
	}

	lock, err := a.stackOperationLock(8 * time.Second)
	if err != nil {
		return err
	}
	defer releaseFileLock(lock)

	state = a.prefixProcessState(gc.Prefix)
	if state.Game {
		return ErrGameAlreadyRunning
	}
	if state.RSI {
		return ErrLauncherAlreadyRunning
	}
	if prefixBusy(gc.Prefix) {
		return errors.New("Der Wine-Prefix wird bereits von einem anderen Prozess verwendet. Bitte laufende Wine-Werkzeuge schließen und erneut versuchen.")
	}

	st := a.gameStatus()
	triggerMaintenance := false
	if cfg := a.loadConfig(); cfg.AutoMaintain {
		triggerMaintenance = true
		if t, err := time.Parse(time.RFC3339, cfg.LastMaintenance); err == nil {
			triggerMaintenance = time.Since(t) > 6*time.Hour
		}
	}
	if st.Hardware == "blocked" {
		return errors.New(st.HardwareReason)
	}
	if st.System.State == "blocked" {
		return errors.New(st.System.Reason)
	}
	if st.System.State == "prepare" {
		if err := a.ensureSystemPrepared(); err != nil {
			return fmt.Errorf("Systemvorbereitung: %w", err)
		}
	}
	if err := setLaunchNoFile(); err != nil {
		a.logf("launch nofile warning: %v", err)
	}
	if !prefixInitialized(gc.Prefix) {
		return errors.New("Wine-Prefix fehlt; zuerst Setup ausführen")
	}
	if _, err := os.Stat(gc.LauncherEXE); err != nil {
		return errors.New("RSI Launcher fehlt; Reparatur ausführen")
	}
	if st.DXVKState != "ready" {
		return errors.New("DXVK ist nicht vollständig aktiviert; bitte Automatisch reparieren ausführen")
	}
	if st.PowerShellState != "ready" {
		return errors.New("Die RSI-PowerShell-Kompatibilität ist unvollständig; bitte Automatisch reparieren ausführen")
	}
	env, runner, err := a.runnerEnv(gc.Prefix)
	if err != nil {
		return err
	}
	env = append(env,
		"__GL_SHADER_DISK_CACHE=1",
		"__GL_SHADER_DISK_CACHE_SIZE=10737418240",
		"__GL_SHADER_DISK_CACHE_PATH="+gc.Prefix,
		"__GL_SHADER_DISK_CACHE_SKIP_CLEANUP=1",
		"MESA_SHADER_CACHE_DIR="+gc.Prefix,
		"MESA_SHADER_CACHE_MAX_SIZE=10G",
	)

	// A running game/launcher was ruled out above. Clearing the dedicated
	// wineserver here only removes stale prefix processes from previous crashes.
	ws := exec.Command(filepath.Join(runner, "bin", "wineserver"), "-k")
	ws.Env = env
	_, _ = ws.CombinedOutput()

	log := filepath.Join(a.stateDir, "rsi-launcher.log")
	_ = os.MkdirAll(a.stateDir, 0o755)
	rotateFile(log, 8*1024*1024, 4*1024*1024)
	f, err := os.OpenFile(log, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	cmd := exec.Command(filepath.Join(runner, "bin", "wine"), gc.LauncherEXE)
	cmd.Env = env
	cmd.Dir = filepath.Dir(gc.LauncherEXE)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdout = f
	cmd.Stderr = f
	if err := cmd.Start(); err != nil {
		_ = f.Close()
		return err
	}
	_ = f.Close()
	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()

	// Wine may hand off to the Windows child and let the wrapper process exit.
	// Treat the launch as successful when the actual RSI process appears.
	deadline := time.Now().Add(10 * time.Second)
	var wrapperErr error
	for time.Now().Before(deadline) {
		if a.rsiLauncherRunning(gc.Prefix) {
			a.logf("RSI Launcher started pid=%d", cmd.Process.Pid)
			if triggerMaintenance && !detectPlatform().SystemdUser {
				stable := a.stableExecutable()
				go func() {
					time.Sleep(1500 * time.Millisecond)
					_ = exec.Command(stable, "maintain").Start()
				}()
			}
			return nil
		}
		select {
		case wrapperErr = <-waitCh:
			if !a.rsiLauncherRunning(gc.Prefix) {
				if wrapperErr != nil {
					return fmt.Errorf("RSI Launcher wurde früh beendet: %v. Details: %s", wrapperErr, log)
				}
				return fmt.Errorf("RSI Launcher wurde beendet, bevor sein Prozess sichtbar wurde. Details: %s", log)
			}
		default:
		}
		time.Sleep(250 * time.Millisecond)
	}
	if a.rsiLauncherRunning(gc.Prefix) {
		return nil
	}
	return fmt.Errorf("RSI Launcher wurde gestartet, hat aber innerhalb von 10 Sekunden keinen laufenden Launcher-Prozess erzeugt. Details: %s", log)
}

func (a *App) gameRepair() error {
	lock, err := a.stackOperationLock(2 * time.Second)
	if err != nil {
		return err
	}
	defer releaseFileLock(lock)

	gc := a.loadGameConfig()
	if err := validatePrefixPath(gc.Prefix); err != nil {
		return err
	}
	if prefixInitialized(gc.Prefix) && prefixBusy(gc.Prefix) {
		return errors.New("Reparatur pausiert: RSI Launcher, Star Citizen oder ein anderes Wine-Werkzeug verwendet den Prefix noch.")
	}
	hs := a.hardwareStatus(gc.Prefix)
	if hs.State == "blocked" {
		return errors.New(hs.Reason)
	}
	if sys := a.systemReadiness(gc.Prefix); sys.State == "blocked" {
		return errors.New(sys.Reason)
	} else if sys.State == "prepare" {
		if err := a.ensureSystemPrepared(); err != nil {
			return err
		}
	}
	if err := a.syncWineRunner(); err != nil {
		return err
	}
	if !prefixInitialized(gc.Prefix) {
		return a.gameInstallUnlocked()
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
