package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	appVersion = "0.9.1"
	pluginID   = "local.omarchy-citizen"
	appID      = "io.github.citizenlauncher.CitizenLauncher"
)

type Config struct {
	AutoApply         bool   `json:"auto_apply"`
	AutoMaintain      bool   `json:"auto_maintain"`
	TrustedRemote     string `json:"trusted_remote,omitempty"`
	LastCheck         string `json:"last_check,omitempty"`
	LastUpdate        string `json:"last_update,omitempty"`
	LastResult        string `json:"last_result,omitempty"`
	LastMaintenance   string `json:"last_maintenance,omitempty"`
	MaintenanceResult string `json:"maintenance_result,omitempty"`
}

type Status struct {
	BackendVersion    string `json:"backend_version"`
	PluginID          string `json:"plugin_id"`
	PluginDir         string `json:"plugin_dir"`
	Managed           string `json:"managed"`
	AutoApply         bool   `json:"auto_apply"`
	AutoMaintain      bool   `json:"auto_maintain"`
	LUGVersion        string `json:"lug_version,omitempty"`
	WineVersion       string `json:"wine_version,omitempty"`
	DXVKVersion       string `json:"dxvk_version,omitempty"`
	LastMaintenance   string `json:"last_maintenance,omitempty"`
	MaintenanceResult string `json:"maintenance_result,omitempty"`
	Remote            string `json:"remote,omitempty"`
	TrustedRemote     string `json:"trusted_remote,omitempty"`
	Branch            string `json:"branch,omitempty"`
	LocalCommit       string `json:"local_commit,omitempty"`
	RemoteCommit      string `json:"remote_commit,omitempty"`
	Dirty             bool   `json:"dirty"`
	Update            string `json:"update"`
	LastCheck         string `json:"last_check,omitempty"`
	LastUpdate        string `json:"last_update,omitempty"`
	LastResult        string `json:"last_result,omitempty"`
}

type App struct {
	home       string
	pluginDir  string
	configDir  string
	stateDir   string
	dataDir    string
	cacheDir   string
	vendorDir  string
	configPath string
	logPath    string
	libDir     string
	selfPath   string
}

func main() {
	app, err := newApp()
	if err != nil {
		fatal(err)
	}
	app.rotateLog()

	args := os.Args[1:]
	if len(args) == 0 {
		args = []string{"status"}
	}

	switch args[0] {
	case "version", "--version", "-v":
		fmt.Println(appVersion)
	case "status":
		st, _ := app.status(false)
		if hasArg(args[1:], "--json") {
			printJSON(st)
		} else {
			printStatusKV(st)
		}
	case "check":
		st, err := app.status(true)
		app.recordCheck(st, err)
		if hasArg(args[1:], "--json") {
			printJSON(st)
		} else {
			printStatusKV(st)
		}
		if err != nil {
			os.Exit(2)
		}
	case "update":
		if err := app.update(false); err != nil {
			app.logf("manual update failed: %v", err)
			fatal(err)
		}
	case "tick":
		if err := app.tick(); err != nil {
			app.logf("tick failed: %v", err)
			os.Exit(2)
		}
	case "maintain":
		if err := app.maintainGamingStack(); err != nil {
			app.logf("maintenance failed: %v", err)
			fatal(err)
		}
	case "doctor":
		if err := app.wineDoctor(); err != nil {
			fatal(err)
		}
	case "game-status":
		st := app.gameStatus()
		if hasArg(args[1:], "--json") {
			printJSON(st)
		} else {
			printGameStatusKV(st)
		}
	case "game-install":
		if err := app.gameInstall(); err != nil {
			fatal(err)
		}
	case "game-launch":
		if err := app.gameLaunch(); err != nil {
			fatal(err)
		}
	case "game-repair":
		if err := app.gameRepair(); err != nil {
			fatal(err)
		}
	case "gui":
		if err := app.runGUI(args[1:]); err != nil {
			fatal(err)
		}
	case "platform":
		ps := detectPlatform()
		if hasArg(args[1:], "--json") {
			printJSON(ps)
		} else {
			printPlatformKV(ps)
		}
	case "support":
		path, err := app.createSupportBundle()
		if err != nil {
			fatal(err)
		}
		fmt.Println(path)
	case "autopilot":
		if len(args) < 2 {
			fatal(errors.New("usage: autopilot enable|disable|status"))
		}
		switch args[1] {
		case "enable":
			if err := app.enableAutopilot(); err != nil {
				fatal(err)
			}
		case "disable":
			if err := app.disableAutopilot(); err != nil {
				fatal(err)
			}
		case "status":
			cfg := app.loadConfig()
			if cfg.AutoMaintain {
				fmt.Println("enabled")
			} else {
				fmt.Println("disabled")
			}
		default:
			fatal(errors.New("usage: autopilot enable|disable|status"))
		}
	case "auto":
		if len(args) < 2 {
			fatal(errors.New("usage: auto enable|disable|status"))
		}
		switch args[1] {
		case "enable":
			if err := app.enableAuto(); err != nil {
				fatal(err)
			}
		case "disable":
			if err := app.disableAuto(); err != nil {
				fatal(err)
			}
		case "status":
			cfg := app.loadConfig()
			if cfg.AutoApply {
				fmt.Println("enabled")
			} else {
				fmt.Println("disabled")
			}
		default:
			fatal(errors.New("usage: auto enable|disable|status"))
		}
	case "install-service":
		if err := app.installService(); err != nil {
			fatal(err)
		}
	case "uninstall-service":
		if err := app.uninstallService(); err != nil {
			fatal(err)
		}
	case "self-sync":
		if err := app.selfSync(); err != nil {
			fatal(err)
		}
	default:
		fatal(fmt.Errorf("unknown command: %s", args[0]))
	}
}

func newApp() (*App, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	self, _ := os.Executable()
	self, _ = filepath.EvalSymlinks(self)
	configHome := envOr("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	stateHome := envOr("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	dataHome := envOr("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	cacheHome := envOr("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	a := &App{
		home:       home,
		pluginDir:  filepath.Join(configHome, "omarchy", "plugins", pluginID),
		configDir:  filepath.Join(configHome, "citizen-launcher"),
		stateDir:   filepath.Join(stateHome, "citizen-launcher"),
		dataDir:    filepath.Join(dataHome, "citizen-launcher"),
		cacheDir:   filepath.Join(cacheHome, "citizen-launcher"),
		configPath: filepath.Join(configHome, "citizen-launcher", "updater.json"),
		logPath:    filepath.Join(stateHome, "citizen-launcher", "updater.log"),
		libDir:     filepath.Join(home, ".local", "lib", "citizen-launcher"),
		selfPath:   self,
	}
	a.vendorDir = filepath.Join(a.dataDir, "vendor")
	_ = a.migrateLegacyPaths(configHome, stateHome, dataHome)
	return a, nil
}

func (a *App) migrateLegacyPaths(configHome, stateHome, dataHome string) error {
	pairs := [][2]string{
		{filepath.Join(configHome, "omarchy-citizen"), a.configDir},
		{filepath.Join(stateHome, "omarchy-citizen"), a.stateDir},
		{filepath.Join(dataHome, "omarchy-citizen"), a.dataDir},
	}
	for _, pair := range pairs {
		if _, err := os.Stat(pair[1]); err == nil {
			continue
		}
		if fi, err := os.Stat(pair[0]); err == nil && fi.IsDir() {
			_ = os.MkdirAll(filepath.Dir(pair[1]), 0o755)
			if err := os.Rename(pair[0], pair[1]); err != nil {
				// Cross-filesystem and permission-safe fallback: leave legacy data in place.
				// loadGameConfig() separately adopts the legacy Star Citizen prefix.
				a.logf("legacy migration warning %s -> %s: %v", pair[0], pair[1], err)
			}
		}
	}
	return nil
}

func (a *App) status(fetch bool) (Status, error) {
	cfg := a.loadConfig()
	st := Status{
		BackendVersion:    appVersion,
		PluginID:          pluginID,
		PluginDir:         a.pluginDir,
		Managed:           "manual",
		AutoApply:         cfg.AutoApply,
		AutoMaintain:      cfg.AutoMaintain,
		LUGVersion:        readMetaVersion(filepath.Join(a.vendorDir, "lug-helper", "meta.json")),
		WineVersion:       readMetaVersion(filepath.Join(a.vendorDir, "wine", "meta.json")),
		DXVKVersion:       readMetaVersion(filepath.Join(a.vendorDir, "dxvk", "meta.json")),
		LastMaintenance:   cfg.LastMaintenance,
		MaintenanceResult: cfg.MaintenanceResult,
		TrustedRemote:     cfg.TrustedRemote,
		Update:            "unavailable",
		LastCheck:         cfg.LastCheck,
		LastUpdate:        cfg.LastUpdate,
		LastResult:        cfg.LastResult,
	}

	if !isGitRepo(a.pluginDir) || !commandExists("omarchy") {
		st.Managed = "standalone"
		st.Update = "external-release"
		return st, nil
	}
	st.Managed = "omarchy-git"

	remote, _ := git(a.pluginDir, "remote", "get-url", "origin")
	st.Remote = strings.TrimSpace(remote)
	branch, _ := git(a.pluginDir, "branch", "--show-current")
	st.Branch = strings.TrimSpace(branch)
	local, _ := git(a.pluginDir, "rev-parse", "HEAD")
	st.LocalCommit = short(strings.TrimSpace(local))
	dirty, _ := git(a.pluginDir, "status", "--porcelain")
	st.Dirty = strings.TrimSpace(dirty) != ""

	if fetch {
		if st.Dirty {
			st.Update = "blocked-dirty"
			return st, errors.New("plugin checkout has local changes")
		}
		if cfg.TrustedRemote != "" && st.Remote != cfg.TrustedRemote {
			st.Update = "blocked-remote-changed"
			return st, errors.New("git origin differs from trusted remote")
		}
		if _, err := git(a.pluginDir, "fetch", "--quiet", "origin"); err != nil {
			st.Update = "check-failed"
			return st, err
		}
	}

	upstream, err := git(a.pluginDir, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err != nil {
		st.Update = "no-upstream"
		return st, nil
	}
	upstream = strings.TrimSpace(upstream)
	remoteCommit, err := git(a.pluginDir, "rev-parse", upstream)
	if err != nil {
		st.Update = "unknown"
		return st, err
	}
	remoteCommit = strings.TrimSpace(remoteCommit)
	st.RemoteCommit = short(remoteCommit)
	localFull, _ := git(a.pluginDir, "rev-parse", "HEAD")
	localFull = strings.TrimSpace(localFull)

	switch {
	case st.Dirty:
		st.Update = "blocked-dirty"
	case localFull == remoteCommit:
		st.Update = "current"
	default:
		ancestor, err := runExit(a.pluginDir, "git", "merge-base", "--is-ancestor", localFull, remoteCommit)
		if err == nil && ancestor == 0 {
			st.Update = "available"
		} else {
			st.Update = "diverged"
		}
	}
	return st, nil
}

func (a *App) tick() error {
	cfg := a.loadConfig()
	var problems []string

	if cfg.AutoMaintain {
		if err := a.maintainGamingStack(); err != nil {
			problems = append(problems, "gaming stack: "+err.Error())
		}
	}

	if cfg.AutoApply {
		st, err := a.status(true)
		a.recordCheck(st, err)
		if err != nil {
			problems = append(problems, "plugin check: "+err.Error())
		} else if st.Update == "available" {
			if err := a.update(true); err != nil {
				problems = append(problems, "plugin update: "+err.Error())
			}
		}
	}

	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func (a *App) update(automatic bool) error {
	unlock, err := a.acquireUpdateLock()
	if err != nil {
		return err
	}
	defer unlock()

	st, err := a.status(true)
	a.recordCheck(st, err)
	if err != nil {
		return err
	}
	if st.Managed != "omarchy-git" {
		return errors.New("this update path is only available for the optional Omarchy Git integration")
	}
	cfg := a.loadConfig()
	if cfg.TrustedRemote != "" && st.Remote != cfg.TrustedRemote {
		return errors.New("refusing update: git origin changed after auto-update trust was granted")
	}
	if st.Dirty {
		return errors.New("refusing update: plugin contains local changes")
	}
	if st.Update == "current" {
		a.logf("update: already current at %s", st.LocalCommit)
		return nil
	}
	if st.Update != "available" {
		return fmt.Errorf("update cannot be applied in state %q", st.Update)
	}

	before := st.LocalCommit
	a.logf("update begin automatic=%v remote=%q from=%s to=%s", automatic, st.Remote, st.LocalCommit, st.RemoteCommit)

	cmd := exec.Command("omarchy", "plugin", "update", pluginID, "--yes")
	cmd.Stdout = io.MultiWriter(os.Stdout, a.logWriter())
	cmd.Stderr = io.MultiWriter(os.Stderr, a.logWriter())
	if err := cmd.Run(); err != nil {
		a.updateConfig(func(c *Config) {
			c.LastResult = "update-failed"
		})
		return fmt.Errorf("omarchy plugin update failed: %w", err)
	}

	if err := a.validatePlugin(); err != nil {
		a.updateConfig(func(c *Config) {
			c.LastResult = "post-validation-failed"
		})
		return fmt.Errorf("post-update validation failed: %w", err)
	}

	after, _ := git(a.pluginDir, "rev-parse", "HEAD")
	after = short(strings.TrimSpace(after))
	a.updateConfig(func(c *Config) {
		c.LastUpdate = time.Now().Format(time.RFC3339)
		c.LastResult = "updated:" + before + "->" + after
	})
	a.logf("update success from=%s to=%s", before, after)
	a.notify("Citizen Launcher", "Plugin-Update installiert: "+before+" → "+after)

	if err := a.selfSync(); err != nil {
		a.logf("backend self-sync warning: %v", err)
	}
	_ = run("", "omarchy-shell", "shell", "rescanPlugins")
	if !automatic {
		// Current Omarchy releases can keep stale third-party bar QML after a rescan.
		// A user-triggered update may safely request a full shell restart so the
		// freshly validated plugin code becomes active immediately.
		_ = run("", "omarchy", "restart", "shell")
	}
	return nil
}

func (a *App) enableAuto() error {
	st, err := a.status(true)
	if err != nil {
		return err
	}
	if st.Managed != "omarchy-git" || st.Remote == "" {
		return errors.New("integration auto-updates require the optional Omarchy Git integration")
	}
	if st.Dirty {
		return errors.New("cannot enable auto updates while the plugin checkout has local changes")
	}
	if err := a.validatePlugin(); err != nil {
		return err
	}
	if err := a.installService(); err != nil {
		return err
	}
	a.updateConfig(func(c *Config) {
		c.AutoApply = true
		c.TrustedRemote = st.Remote
		c.LastResult = "auto-enabled"
	})
	if err := run("", "systemctl", "--user", "enable", "--now", "citizen-launcher-maintenance.timer"); err != nil {
		return err
	}
	a.logf("auto updates enabled trusted_remote=%q", st.Remote)
	fmt.Println("enabled")
	return nil
}

func (a *App) disableAuto() error {
	a.updateConfig(func(c *Config) {
		c.AutoApply = false
		c.LastResult = "auto-disabled"
	})
	_ = run("", "systemctl", "--user", "disable", "--now", "citizen-launcher-maintenance.timer")
	a.logf("auto updates disabled")
	fmt.Println("disabled")
	return nil
}

func (a *App) installService() error {
	if err := os.MkdirAll(filepath.Join(a.home, ".config", "systemd", "user"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(a.libDir, 0o755); err != nil {
		return err
	}
	if err := a.selfSync(); err != nil {
		return err
	}

	servicePath := filepath.Join(a.home, ".config", "systemd", "user", "citizen-launcher-maintenance.service")
	timerPath := filepath.Join(a.home, ".config", "systemd", "user", "citizen-launcher-maintenance.timer")
	backendPath := filepath.Join(a.libDir, "citizen-launcher")

	service := `[Unit]\nDescription=Citizen Launcher Autopilot maintenance\nAfter=network-online.target\nWants=network-online.target\n\n[Service]\nType=oneshot\nExecStart=` + backendPath + ` tick\n`
	timer := `[Unit]\nDescription=Maintain Citizen Launcher gaming stack automatically\n\n[Timer]\nOnBootSec=3min\nOnUnitActiveSec=6h\nRandomizedDelaySec=15min\nPersistent=true\nUnit=citizen-launcher-maintenance.service\n\n[Install]\nWantedBy=timers.target\n`

	if err := os.WriteFile(servicePath, []byte(strings.ReplaceAll(service, `\n`, "\n")), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(timerPath, []byte(strings.ReplaceAll(timer, `\n`, "\n")), 0o644); err != nil {
		return err
	}
	return run("", "systemctl", "--user", "daemon-reload")
}

func (a *App) uninstallService() error {
	_ = run("", "systemctl", "--user", "disable", "--now", "citizen-launcher-maintenance.timer")
	_ = os.Remove(filepath.Join(a.home, ".config", "systemd", "user", "citizen-launcher-maintenance.service"))
	_ = os.Remove(filepath.Join(a.home, ".config", "systemd", "user", "citizen-launcher-maintenance.timer"))
	_ = run("", "systemctl", "--user", "daemon-reload")
	return nil
}

func (a *App) selfSync() error {
	if err := os.MkdirAll(a.libDir, 0o755); err != nil {
		return err
	}
	target := filepath.Join(a.libDir, "citizen-launcher")
	source := a.selfPath
	// Omarchy integration ships a bundled copy; prefer it when present.
	bundled := filepath.Join(a.pluginDir, "backend", "bin", "citizen-launcher")
	if _, err := os.Stat(bundled); err == nil {
		source = bundled
	}
	if source == "" {
		return errors.New("cannot locate Citizen Launcher executable")
	}
	same, _ := sameFileHash(source, target)
	if !same {
		tmp := target + ".new"
		if err := copyFile(source, tmp, 0o755); err != nil {
			return err
		}
		if err := os.Rename(tmp, target); err != nil {
			return err
		}
	}
	// Compatibility alias for older Omarchy Citizen adapters.
	legacyDir := filepath.Join(a.home, ".local", "lib", "omarchy-citizen")
	_ = os.MkdirAll(legacyDir, 0o755)
	legacy := filepath.Join(legacyDir, "omarchy-citizen-backend")
	_ = os.Remove(legacy)
	_ = os.Symlink(target, legacy)
	a.logf("self-sync installed %s", target)
	return nil
}

func (a *App) acquireUpdateLock() (func(), error) {
	if err := os.MkdirAll(a.stateDir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(a.stateDir, "update.lock")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, errors.New("another Omarchy Citizen update is already running")
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
}

func (a *App) notify(title, body string) {
	if _, err := exec.LookPath("omarchy-notification-send"); err == nil {
		_ = exec.Command("omarchy-notification-send", title, body).Run()
		return
	}
	if _, err := exec.LookPath("notify-send"); err == nil {
		_ = exec.Command("notify-send", title, body).Run()
	}
}

func (a *App) validatePlugin() error {
	cmd := exec.Command("omarchy", "plugin", "validate", a.pluginDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("plugin validation failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func (a *App) recordCheck(st Status, err error) {
	a.updateConfig(func(c *Config) {
		c.LastCheck = time.Now().Format(time.RFC3339)
		if err != nil {
			c.LastResult = "check-failed:" + err.Error()
		} else {
			c.LastResult = "check:" + st.Update
		}
	})
	a.logf("check managed=%s update=%s local=%s remote=%s dirty=%v err=%v", st.Managed, st.Update, st.LocalCommit, st.RemoteCommit, st.Dirty, err)
}

func (a *App) loadConfig() Config {
	var cfg Config
	data, err := os.ReadFile(a.configPath)
	if err == nil {
		_ = json.Unmarshal(data, &cfg)
	}
	return cfg
}

func (a *App) updateConfig(fn func(*Config)) {
	cfg := a.loadConfig()
	fn(&cfg)
	_ = os.MkdirAll(a.configDir, 0o755)
	data, _ := json.MarshalIndent(cfg, "", "  ")
	data = append(data, '\n')
	tmp := a.configPath + ".tmp"
	if os.WriteFile(tmp, data, 0o600) == nil {
		_ = os.Rename(tmp, a.configPath)
	}
}

func (a *App) rotateLog() {
	_ = os.MkdirAll(a.stateDir, 0o755)
	info, err := os.Stat(a.logPath)
	if err != nil || info.Size() <= 4*1024*1024 {
		return
	}
	f, err := os.Open(a.logPath)
	if err != nil {
		return
	}
	defer f.Close()
	keep := int64(2 * 1024 * 1024)
	if _, err := f.Seek(-keep, io.SeekEnd); err != nil {
		return
	}
	data, _ := io.ReadAll(f)
	_ = os.WriteFile(a.logPath, data, 0o600)
}

func (a *App) logWriter() io.Writer {
	_ = os.MkdirAll(a.stateDir, 0o755)
	f, err := os.OpenFile(a.logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return io.Discard
	}
	return &autoCloseWriter{f: f}
}

func (a *App) logf(format string, args ...any) {
	_ = os.MkdirAll(a.stateDir, 0o755)
	f, err := os.OpenFile(a.logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s version=%s %s\n", time.Now().Format(time.RFC3339), appVersion, fmt.Sprintf(format, args...))
}

type autoCloseWriter struct{ f *os.File }

func (w *autoCloseWriter) Write(p []byte) (int, error) { return w.f.Write(p) }

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func run(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runExit(dir, name string, args ...string) (int, error) {
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	err := cmd.Run()
	if err == nil {
		return 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), nil
	}
	return -1, err
}

func isGitRepo(dir string) bool {
	out, err := git(dir, "rev-parse", "--is-inside-work-tree")
	return err == nil && strings.TrimSpace(out) == "true"
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func sameFileHash(a, b string) (bool, error) {
	ha, err := fileHash(a)
	if err != nil {
		return false, err
	}
	hb, err := fileHash(b)
	if err != nil {
		return false, err
	}
	return ha == hb, nil
}

func fileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
func short(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}
func hasArg(args []string, needle string) bool {
	for _, a := range args {
		if a == needle {
			return true
		}
	}
	return false
}
func printJSON(v any) { enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  "); _ = enc.Encode(v) }
func printStatusKV(s Status) {
	fmt.Printf("backend_version=%s\n", s.BackendVersion)
	fmt.Printf("managed=%s\n", s.Managed)
	fmt.Printf("auto_apply=%t\n", s.AutoApply)
	fmt.Printf("auto_maintain=%t\n", s.AutoMaintain)
	fmt.Printf("lug_version=%s\n", s.LUGVersion)
	fmt.Printf("wine_version=%s\n", s.WineVersion)
	fmt.Printf("dxvk_version=%s\n", s.DXVKVersion)
	fmt.Printf("last_maintenance=%s\n", s.LastMaintenance)
	fmt.Printf("maintenance_result=%s\n", s.MaintenanceResult)
	fmt.Printf("update=%s\n", s.Update)
	fmt.Printf("remote=%s\n", s.Remote)
	fmt.Printf("trusted_remote=%s\n", s.TrustedRemote)
	fmt.Printf("branch=%s\n", s.Branch)
	fmt.Printf("local_commit=%s\n", s.LocalCommit)
	fmt.Printf("remote_commit=%s\n", s.RemoteCommit)
	fmt.Printf("dirty=%t\n", s.Dirty)
	fmt.Printf("last_check=%s\n", s.LastCheck)
	fmt.Printf("last_update=%s\n", s.LastUpdate)
	fmt.Printf("last_result=%s\n", s.LastResult)
}
func commandExists(name string) bool { _, err := exec.LookPath(name); return err == nil }

func printPlatformKV(p PlatformStatus) {
	fmt.Printf("id=%s\n", p.ID)
	fmt.Printf("name=%s\n", p.Name)
	fmt.Printf("version=%s\n", p.Version)
	fmt.Printf("id_like=%s\n", p.IDLike)
	fmt.Printf("package_manager=%s\n", p.PackageManager)
	fmt.Printf("desktop=%s\n", p.Desktop)
	fmt.Printf("session=%s\n", p.Session)
	fmt.Printf("systemd_user=%t\n", p.SystemdUser)
	fmt.Printf("omarchy=%t\n", p.Omarchy)
}

func fatal(err error) { fmt.Fprintln(os.Stderr, "citizen-launcher:", err); os.Exit(1) }

func printGameStatusKV(s GameStatus) {
	fmt.Printf("backend_version=%s\n", s.BackendVersion)
	fmt.Printf("health=%s\n", s.Health)
	fmt.Printf("hardware=%s\n", s.Hardware)
	fmt.Printf("hardware_reason=%s\n", s.HardwareReason)
	fmt.Printf("gpu=%s\n", s.GPU)
	fmt.Printf("vulkan=%s\n", s.Vulkan)
	fmt.Printf("cpu_avx=%t\n", s.CPUAVX)
	fmt.Printf("ram_gib=%d\n", s.RAMGiB)
	fmt.Printf("combined_gib=%d\n", s.CombinedGiB)
	fmt.Printf("disk_free_gib=%d\n", s.DiskFreeGiB)
	fmt.Printf("prefix=%s\n", s.Prefix)
	fmt.Printf("prefix_state=%s\n", s.PrefixState)
	fmt.Printf("launcher_state=%s\n", s.LauncherState)
	fmt.Printf("game_state=%s\n", s.GameState)
	fmt.Printf("wine_version=%s\n", s.WineVersion)
	fmt.Printf("dxvk_version=%s\n", s.DXVKVersion)
	fmt.Printf("lug_version=%s\n", s.LUGVersion)
	fmt.Printf("rsi_installer=%s\n", s.RSIInstaller)
	fmt.Printf("autopilot=%t\n", s.Autopilot)
}
