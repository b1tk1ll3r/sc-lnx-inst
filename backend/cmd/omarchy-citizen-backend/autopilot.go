package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

const (
	lugRepo  = "starcitizen-lug/lug-helper"
	wineRepo = "starcitizen-lug/lug-wine"
	dxvkRepo = "doitsujin/dxvk"
)

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

type componentMeta struct {
	Version string `json:"version"`
	Asset   string `json:"asset,omitempty"`
	URL     string `json:"url,omitempty"`
	Updated string `json:"updated"`
}

type wineRejection struct {
	Version     string `json:"version"`
	Reason      string `json:"reason"`
	Fingerprint string `json:"fingerprint"`
	Updated     string `json:"updated"`
}

func (a *App) enableAutopilot() error {
	if err := a.installService(); err != nil {
		return err
	}
	cfg := a.loadConfig()
	cfg.AutoMaintain = true

	// Plugin auto-update is enabled automatically only for a clean, trusted Git checkout.
	// The gaming stack does not depend on Git and remains fully auto-maintained either way.
	st, _ := a.status(true)
	if st.Managed == "git" && !st.Dirty && st.Remote != "" {
		cfg.AutoApply = true
		if cfg.TrustedRemote == "" {
			cfg.TrustedRemote = st.Remote
		}
	}
	cfg.LastResult = "autopilot-enabled"
	a.saveConfig(cfg)

	if err := run("", "systemctl", "--user", "enable", "--now", "omarchy-citizen-update.timer"); err != nil {
		return err
	}
	a.logf("autopilot enabled auto_plugin=%v", cfg.AutoApply)

	// First maintenance is intentionally synchronous: after installation the stack
	// is already usable/current instead of waiting for the timer.
	if err := a.maintainGamingStack(); err != nil {
		return err
	}
	fmt.Println("enabled")
	return nil
}

func (a *App) disableAutopilot() error {
	cfg := a.loadConfig()
	cfg.AutoMaintain = false
	cfg.LastResult = "autopilot-disabled"
	a.saveConfig(cfg)
	a.logf("autopilot disabled")
	fmt.Println("disabled")
	return nil
}

func (a *App) maintainGamingStack() error {
	unlock, err := a.acquireMaintenanceLock()
	if err != nil {
		return err
	}
	defer unlock()

	a.logf("maintenance begin")
	var problems []string

	if err := a.syncLUGHelper(); err != nil {
		problems = append(problems, "LUG Helper: "+err.Error())
	}
	if err := a.syncWineRunner(); err != nil {
		problems = append(problems, "Wine runner: "+err.Error())
	}
	if err := a.syncDXVK(); err != nil {
		problems = append(problems, "DXVK: "+err.Error())
	}

	result := "ok"
	if len(problems) > 0 {
		result = strings.Join(problems, " | ")
	}
	a.updateConfig(func(c *Config) {
		c.LastMaintenance = time.Now().Format(time.RFC3339)
		c.MaintenanceResult = result
	})
	a.logf("maintenance end result=%s", result)

	if len(problems) > 0 {
		return errors.New(result)
	}
	return nil
}

func (a *App) syncLUGHelper() error {
	release, err := githubLatest(lugRepo)
	if err != nil {
		return err
	}
	asset, err := selectAsset(release.Assets, func(n string) bool {
		n = strings.ToLower(n)
		return strings.HasSuffix(n, ".appimage") && strings.Contains(n, "lug-helper")
	})
	if err != nil {
		return fmt.Errorf("release %s: %w", release.TagName, err)
	}

	dir := filepath.Join(a.vendorDir, "lug-helper")
	metaPath := filepath.Join(dir, "meta.json")
	target := filepath.Join(dir, "lug-helper.appimage")
	if readMetaVersion(metaPath) == release.TagName {
		if fi, err := os.Stat(target); err == nil && fi.Mode().IsRegular() {
			return nil
		}
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp := target + ".new"
	if err := download(asset.BrowserDownloadURL, tmp); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o755); err != nil {
		return err
	}
	if err := os.Rename(tmp, target); err != nil {
		return err
	}
	if err := writeMeta(metaPath, componentMeta{
		Version: release.TagName, Asset: asset.Name, URL: asset.BrowserDownloadURL,
		Updated: time.Now().Format(time.RFC3339),
	}); err != nil {
		return err
	}
	a.logf("LUG Helper updated to %s asset=%s", release.TagName, asset.Name)
	return nil
}

func (a *App) syncWineRunner() error {
	releases, err := githubRecent(wineRepo, 10)
	if err != nil {
		return err
	}
	if len(releases) == 0 {
		return errors.New("LUG Wine repository returned no releases")
	}

	base := filepath.Join(a.vendorDir, "wine")
	metaPath := filepath.Join(base, "meta.json")
	current := filepath.Join(base, "current")
	rejectPath := filepath.Join(base, "rejected.json")
	fingerprint := compatibilityFingerprint()
	rejections := readWineRejections(rejectPath)

	if err := os.MkdirAll(base, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(a.cacheDir, 0o755); err != nil {
		return err
	}

	var failures []string
	for _, release := range releases {
		asset, assetErr := selectAsset(release.Assets, func(n string) bool {
			n = strings.ToLower(n)
			if strings.Contains(n, "staging") || strings.Contains(n, "checksum") ||
				strings.Contains(n, "sha") || strings.Contains(n, "experimental") ||
				strings.Contains(n, "wayland") {
				return false
			}
			return strings.Contains(n, "lug-wine-tkg-git") &&
				(strings.HasSuffix(n, ".tar.xz") || strings.HasSuffix(n, ".tar.zst") ||
					strings.HasSuffix(n, ".tar.gz") || strings.HasSuffix(n, ".tgz"))
		})
		if assetErr != nil {
			continue
		}

		// If this exact immutable release already failed on the same hardware /
		// libc environment, do not waste bandwidth testing it every six hours.
		if rejection, ok := rejections[release.TagName]; ok && rejection.Fingerprint == fingerprint {
			failures = append(failures, release.TagName+" cached-incompatible: "+rejection.Reason)
			continue
		}

		// Already-active compatible version: no download/test needed.
		if readMetaVersion(metaPath) == release.TagName {
			if p, _ := filepath.EvalSymlinks(current); p != "" {
				if _, err := os.Stat(filepath.Join(p, "bin", "wine")); err == nil {
					a.activateRunnerForPrefix(p)
					return nil
				}
			}
		}

		archive := filepath.Join(a.cacheDir, "wine-"+release.TagName+"-"+filepath.Base(asset.Name))
		if err := download(asset.BrowserDownloadURL, archive); err != nil {
			failures = append(failures, release.TagName+" download: "+err.Error())
			continue
		}

		stage, err := os.MkdirTemp(a.cacheDir, "wine-stage-")
		if err != nil {
			return err
		}

		extract := exec.Command("tar", "-xf", archive, "-C", stage)
		out, extractErr := extract.CombinedOutput()
		if extractErr != nil {
			os.RemoveAll(stage)
			failures = append(failures, release.TagName+" extract: "+formatCommandFailure(extractErr, out))
			continue
		}

		runnerRoot, err := findRootContaining(stage, filepath.Join("bin", "wine"))
		if err != nil {
			os.RemoveAll(stage)
			failures = append(failures, release.TagName+" layout: "+err.Error())
			continue
		}

		diag, testErr := a.selfTestRunnerDetailed(runnerRoot)
		if testErr != nil {
			reason := testErr.Error()
			a.logf("Wine candidate %s rejected: %s | diagnostic=%s", release.TagName, reason, compactDiagnostic(diag))
			rejections[release.TagName] = wineRejection{
				Version: release.TagName, Reason: reason, Fingerprint: fingerprint,
				Updated: time.Now().Format(time.RFC3339),
			}
			_ = writeWineRejections(rejectPath, rejections)
			failures = append(failures, release.TagName+" incompatible: "+reason)
			os.RemoveAll(stage)
			_ = os.Remove(archive)
			continue
		}

		target := filepath.Join(base, release.TagName)
		_ = os.RemoveAll(target + ".new")
		if err := copyTree(runnerRoot, target+".new"); err != nil {
			os.RemoveAll(stage)
			return err
		}
		_ = os.RemoveAll(target)
		if err := os.Rename(target+".new", target); err != nil {
			os.RemoveAll(stage)
			return err
		}

		tmpLink := current + ".new"
		_ = os.Remove(tmpLink)
		if err := os.Symlink(target, tmpLink); err != nil {
			os.RemoveAll(stage)
			return err
		}
		if err := os.Rename(tmpLink, current); err != nil {
			os.RemoveAll(stage)
			return err
		}

		if err := writeMeta(metaPath, componentMeta{
			Version: release.TagName, Asset: asset.Name, URL: asset.BrowserDownloadURL,
			Updated: time.Now().Format(time.RFC3339),
		}); err != nil {
			os.RemoveAll(stage)
			return err
		}
		delete(rejections, release.TagName)
		_ = writeWineRejections(rejectPath, rejections)

		a.activateRunnerForPrefix(target)
		a.logf("Wine runner activated: %s asset=%s diagnostic=%s", release.TagName, asset.Name, compactDiagnostic(diag))
		os.RemoveAll(stage)
		_ = os.Remove(archive)
		return nil
	}

	if len(failures) == 0 {
		return errors.New("no stable LUG Wine asset found in recent releases")
	}
	return fmt.Errorf("no compatible stable LUG Wine runner found; %s", strings.Join(failures, " | "))
}

func (a *App) selfTestRunner(runner string) error {
	_, err := a.selfTestRunnerDetailed(runner)
	return err
}

func (a *App) selfTestRunnerDetailed(runner string) (string, error) {
	bin := filepath.Join(runner, "bin")
	wine := filepath.Join(bin, "wine")
	wineboot := filepath.Join(bin, "wineboot")
	wineserver := filepath.Join(bin, "wineserver")

	for _, required := range []string{wine, wineboot, wineserver} {
		fi, err := os.Stat(required)
		if err != nil {
			return "", fmt.Errorf("runner is incomplete: %s missing", filepath.Base(required))
		}
		if fi.Mode()&0o111 == 0 {
			return "", fmt.Errorf("runner is incomplete: %s is not executable", filepath.Base(required))
		}
	}

	tmp, err := os.MkdirTemp(a.cacheDir, "wine-selftest-prefix-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)

	env := append(os.Environ(),
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"WINE="+wine,
		"WINEPREFIX="+tmp,
		"WINEARCH=win64",
		"WINEDEBUG=-all",
		"WINEDLLOVERRIDES=winemenubuilder.exe=d",
	)

	var diagnostic []string

	// Capture the runner's own version before prefix creation. This also catches
	// loader/CPU failures that otherwise produce an empty wineboot stdout.
	versionCmd := exec.Command(wine, "--version")
	versionCmd.Env = env
	versionOut, versionErr := versionCmd.CombinedOutput()
	if versionErr != nil {
		return strings.Join(diagnostic, "\n"), fmt.Errorf(
			"wine --version failed: %s", formatCommandFailure(versionErr, versionOut))
	}
	diagnostic = append(diagnostic, "wine="+strings.TrimSpace(string(versionOut)))

	// Use the runner-provided wineboot directly, matching how regular Wine
	// launchers create a prefix. Never fall back to `wine wineboot`.
	bootCmd := exec.Command(wineboot, "-u")
	bootCmd.Env = env
	bootOut, bootErr := bootCmd.CombinedOutput()
	if bootErr != nil {
		return strings.Join(diagnostic, "\n"), fmt.Errorf(
			"wineboot failed: %s", formatCommandFailure(bootErr, bootOut))
	}
	if text := strings.TrimSpace(string(bootOut)); text != "" {
		diagnostic = append(diagnostic, "wineboot="+text)
	}

	// Let wineserver finish pending registry writes before validating the prefix.
	waitCmd := exec.Command(wineserver, "-w")
	waitCmd.Env = env
	waitOut, waitErr := waitCmd.CombinedOutput()
	if waitErr != nil {
		return strings.Join(diagnostic, "\n"), fmt.Errorf(
			"wineserver -w failed: %s", formatCommandFailure(waitErr, waitOut))
	}

	deadline := time.Now().Add(20 * time.Second)
	for {
		ready := true
		for _, p := range []string{"drive_c", "system.reg", "user.reg"} {
			if _, err := os.Stat(filepath.Join(tmp, p)); err != nil {
				ready = false
				break
			}
		}
		if ready {
			break
		}
		if time.Now().After(deadline) {
			return strings.Join(diagnostic, "\n"),
				errors.New("wineboot returned successfully but did not create drive_c/system.reg/user.reg")
		}
		time.Sleep(250 * time.Millisecond)
	}

	cmd := exec.Command(wine, "cmd.exe", "/c", "echo", "%APPDATA%")
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		return strings.Join(diagnostic, "\n"), fmt.Errorf(
			"APPDATA probe failed: %s", formatCommandFailure(err, out))
	}
	value := strings.TrimSpace(string(out))
	diagnostic = append(diagnostic, "APPDATA="+value)
	if value == "" || strings.Contains(value, "%APPDATA%") {
		return strings.Join(diagnostic, "\n"), errors.New("Wine returned no usable APPDATA path")
	}

	killCmd := exec.Command(wineserver, "-k")
	killCmd.Env = env
	_, _ = killCmd.CombinedOutput()

	return strings.Join(diagnostic, "\n"), nil
}

func formatCommandFailure(err error, out []byte) string {
	text := strings.TrimSpace(string(out))
	if text == "" {
		return err.Error()
	}
	// Keep status output compact but preserve enough detail in updater.log.
	if len(text) > 1200 {
		text = text[len(text)-1200:]
	}
	return err.Error() + ": " + text
}

func compactDiagnostic(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 600 {
		return s[:600] + "…"
	}
	return s
}

func (a *App) wineDoctor() error {
	current := filepath.Join(a.vendorDir, "wine", "current")
	root, err := filepath.EvalSymlinks(current)
	if err != nil {
		return errors.New("managed Wine runner is not installed yet")
	}
	fmt.Println("runner=" + root)
	if err := a.selfTestRunner(root); err != nil {
		fmt.Println("wine_selftest=failed")
		return err
	}
	fmt.Println("wine_selftest=ok")
	return nil
}

func (a *App) activateRunnerForPrefix(runner string) {
	prefix := a.configuredPrefix()
	if prefix == "" || !prefixInitialized(prefix) || prefixBusy(prefix) {
		return
	}
	launch := filepath.Join(prefix, "sc-launch.sh")
	data, err := os.ReadFile(launch)
	if err != nil {
		return
	}
	want := `export wine_path="` + filepath.Join(runner, "bin") + `"`
	lines := strings.Split(string(data), "\n")
	found := false
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "export wine_path=") {
			lines[i] = want
			found = true
			break
		}
	}
	if !found {
		return
	}
	tmp := launch + ".new"
	if os.WriteFile(tmp, []byte(strings.Join(lines, "\n")), 0o755) == nil {
		_ = os.Rename(tmp, launch)
		a.logf("launch script switched to managed Wine %s", runner)
	}
}

func (a *App) syncDXVK() error {
	prefix := a.configuredPrefix()
	if prefix == "" || !prefixInitialized(prefix) {
		return nil
	}
	if prefixBusy(prefix) {
		a.logf("DXVK update deferred: Star Citizen Wine prefix is active")
		return nil
	}

	release, err := githubLatest(dxvkRepo)
	if err != nil {
		return err
	}
	asset, err := selectAsset(release.Assets, func(n string) bool {
		n = strings.ToLower(n)
		return strings.HasPrefix(n, "dxvk-") && strings.HasSuffix(n, ".tar.gz")
	})
	if err != nil {
		return err
	}

	base := filepath.Join(a.vendorDir, "dxvk")
	metaPath := filepath.Join(base, "meta.json")
	if readMetaVersion(metaPath) == release.TagName {
		return nil
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(a.cacheDir, 0o755); err != nil {
		return err
	}
	archive := filepath.Join(a.cacheDir, filepath.Base(asset.Name))
	if err := download(asset.BrowserDownloadURL, archive); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(a.cacheDir, "dxvk-stage-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if out, err := exec.Command("tar", "-xf", archive, "-C", stage).CombinedOutput(); err != nil {
		return fmt.Errorf("extract DXVK: %s", strings.TrimSpace(string(out)))
	}
	root, err := findRootContaining(stage, filepath.Join("x64", "dxgi.dll"))
	if err != nil {
		return err
	}

	system32 := filepath.Join(prefix, "drive_c", "windows", "system32")
	syswow64 := filepath.Join(prefix, "drive_c", "windows", "syswow64")
	backup := filepath.Join(a.stateDir, "dxvk-backup-"+time.Now().Format("20060102-150405"))
	_ = os.MkdirAll(backup, 0o700)

	if err := installDLLDir(filepath.Join(root, "x64"), system32, backup); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(root, "x32")); err == nil {
		if err := installDLLDir(filepath.Join(root, "x32"), syswow64, backup); err != nil {
			return err
		}
	}
	if err := writeMeta(metaPath, componentMeta{
		Version: release.TagName, Asset: asset.Name, URL: asset.BrowserDownloadURL,
		Updated: time.Now().Format(time.RFC3339),
	}); err != nil {
		return err
	}
	a.logf("DXVK updated to %s", release.TagName)
	_ = os.Remove(archive)
	return nil
}

func installDLLDir(srcDir, dstDir, backup string) error {
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".dll") {
			continue
		}
		src := filepath.Join(srcDir, e.Name())
		dst := filepath.Join(dstDir, e.Name())
		if _, err := os.Stat(dst); err == nil {
			_ = copyFile(dst, filepath.Join(backup, filepath.Base(dstDir)+"-"+e.Name()), 0o644)
		}
		tmp := dst + ".new"
		if err := copyFile(src, tmp, 0o644); err != nil {
			return err
		}
		if err := os.Rename(tmp, dst); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) configuredPrefix() string {
	configHome := envOr("XDG_CONFIG_HOME", filepath.Join(a.home, ".config"))
	data, err := os.ReadFile(filepath.Join(configHome, "starcitizen-lug", "winedir.conf"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func prefixInitialized(prefix string) bool {
	for _, p := range []string{"drive_c", "system.reg", "user.reg"} {
		if _, err := os.Stat(filepath.Join(prefix, p)); err != nil {
			return false
		}
	}
	return true
}

func prefixBusy(prefix string) bool {
	proc, err := os.ReadDir("/proc")
	if err != nil {
		return false
	}
	needle := []byte("WINEPREFIX=" + prefix)
	for _, e := range proc {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "" || name[0] < '0' || name[0] > '9' {
			continue
		}
		data, err := os.ReadFile(filepath.Join("/proc", name, "environ"))
		if err == nil && strings.Contains(string(data), string(needle)) {
			return true
		}
	}
	return false
}

func githubRecent(repo string, limit int) ([]githubRelease, error) {
	if limit <= 0 {
		limit = 10
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=%d", repo, limit)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "omarchy-citizen/"+appVersion)
	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub %s returned HTTP %d", repo, resp.StatusCode)
	}
	var releases []githubRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(&releases); err != nil {
		return nil, err
	}
	return releases, nil
}

func compatibilityFingerprint() string {
	var parts []string
	if out, err := exec.Command("getconf", "GNU_LIBC_VERSION").CombinedOutput(); err == nil {
		parts = append(parts, strings.TrimSpace(string(out)))
	}
	if data, err := os.ReadFile("/proc/cpuinfo"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(strings.ToLower(line), "flags") {
				parts = append(parts, strings.TrimSpace(line))
				break
			}
		}
	}
	if out, err := exec.Command("uname", "-m").CombinedOutput(); err == nil {
		parts = append(parts, strings.TrimSpace(string(out)))
	}
	return strings.Join(parts, " | ")
}

func readWineRejections(path string) map[string]wineRejection {
	result := map[string]wineRejection{}
	data, err := os.ReadFile(path)
	if err != nil {
		return result
	}
	_ = json.Unmarshal(data, &result)
	return result
}

func writeWineRejections(path string, values map[string]wineRejection) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(values, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := path + ".new"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func githubLatest(repo string) (githubRelease, error) {
	var rel githubRelease
	req, err := http.NewRequest("GET", "https://api.github.com/repos/"+repo+"/releases/latest", nil)
	if err != nil {
		return rel, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "omarchy-citizen/"+appVersion)
	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return rel, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return rel, fmt.Errorf("GitHub %s returned HTTP %d", repo, resp.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&rel); err != nil {
		return rel, err
	}
	if rel.TagName == "" {
		return rel, errors.New("release has no tag")
	}
	return rel, nil
}

func selectAsset(assets []githubAsset, match func(string) bool) (githubAsset, error) {
	var candidates []githubAsset
	for _, a := range assets {
		if match(a.Name) {
			candidates = append(candidates, a)
		}
	}
	if len(candidates) == 0 {
		return githubAsset{}, errors.New("no matching release asset")
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Name < candidates[j].Name })
	return candidates[0], nil
}

func download(url, target string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "omarchy-citizen/"+appVersion)
	client := &http.Client{Timeout: 20 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download returned HTTP %d", resp.StatusCode)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, resp.Body)
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(target)
		return copyErr
	}
	return closeErr
}

func findRootContaining(base, relative string) (string, error) {
	var matches []string
	err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && d.Name() == filepath.Base(filepath.Dir(relative)) {
			candidate := filepath.Join(filepath.Dir(path), relative)
			if _, err := os.Stat(candidate); err == nil {
				matches = append(matches, filepath.Dir(path))
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		// Robust fallback: inspect every directory for the relative path.
		_ = filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
			if err == nil && d.IsDir() {
				if _, e := os.Stat(filepath.Join(path, relative)); e == nil {
					matches = append(matches, path)
				}
			}
			return nil
		})
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("archive does not contain %s", relative)
	}
	sort.Strings(matches)
	return matches[0], nil
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		}
		mode := info.Mode().Perm()
		if mode == 0 {
			mode = 0o644
		}
		return copyFile(path, target, mode)
	})
}

func readMetaVersion(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var meta componentMeta
	if json.Unmarshal(data, &meta) != nil {
		return ""
	}
	return meta.Version
}

func writeMeta(path string, meta componentMeta) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(meta, "", "  ")
	data = append(data, '\n')
	tmp := path + ".new"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (a *App) saveConfig(cfg Config) {
	_ = os.MkdirAll(a.configDir, 0o755)
	data, _ := json.MarshalIndent(cfg, "", "  ")
	data = append(data, '\n')
	tmp := a.configPath + ".tmp"
	if os.WriteFile(tmp, data, 0o600) == nil {
		_ = os.Rename(tmp, a.configPath)
	}
}

func (a *App) acquireMaintenanceLock() (func(), error) {
	if err := os.MkdirAll(a.stateDir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(a.stateDir, "maintenance.lock")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, errors.New("another Omarchy Citizen maintenance run is active")
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
}
