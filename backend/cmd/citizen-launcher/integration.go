package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	requiredMapCount = int64(16777216)
	requiredNoFile   = uint64(524288)
)

var safePrefixPath = regexp.MustCompile(`^[A-Za-z0-9_./-]+$`)

func validatePrefixPath(prefix string) error {
	if prefix == "" || !filepath.IsAbs(prefix) {
		return errors.New("Der Wine-Prefix benötigt einen absoluten Linux-Pfad")
	}
	if filepath.Clean(prefix) != prefix {
		return errors.New("Der Installationspfad ist nicht normalisiert")
	}
	if !safePrefixPath.MatchString(prefix) {
		return errors.New("Der Star-Citizen-Pfad darf keine Leerzeichen, Umlaute oder Sonderzeichen enthalten. Empfohlen: ~/Games/star-citizen")
	}
	if fi, err := os.Lstat(prefix); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		return errors.New("Der Wine-Prefix darf kein symbolischer Link sein")
	}
	return nil
}

func rotateFile(path string, maxBytes, keepBytes int64) {
	if maxBytes <= 0 || keepBytes <= 0 || keepBytes >= maxBytes {
		return
	}
	fi, err := os.Stat(path)
	if err != nil || fi.Size() <= maxBytes {
		return
	}
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	if _, err := f.Seek(-keepBytes, 2); err != nil {
		return
	}
	b, err := io.ReadAll(f)
	if err != nil {
		return
	}
	_ = atomicWriteFile(path, b, 0o600)
}

// stableExecutable returns a path that survives application upgrades.
// Package installs always prefer /usr/bin, user installs prefer ~/.local/bin.
func (a *App) stableExecutable() string {
	if installedPackageVersion() != "" {
		if fi, err := os.Stat("/usr/bin/citizen-launcher"); err == nil && fi.Mode().IsRegular() {
			return "/usr/bin/citizen-launcher"
		}
	}
	userBin := filepath.Join(a.home, ".local", "bin", "citizen-launcher")
	if fi, err := os.Stat(userBin); err == nil && fi.Mode().IsRegular() {
		return userBin
	}
	if a.selfPath != "" {
		return a.selfPath
	}
	return filepath.Join(a.libDir, "citizen-launcher")
}

func desktopExecQuote(s string) string {
	// Desktop Entry Exec quoting follows a shell-like double-quoted subset.
	repl := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "`", "\\`", "$", "\\$")
	return `"` + repl.Replace(s) + `"`
}

func (a *App) userApplicationsDir() string {
	return filepath.Join(envOr("XDG_DATA_HOME", filepath.Join(a.home, ".local", "share")), "applications")
}

func (a *App) mainDesktopPath() string {
	return filepath.Join(a.userApplicationsDir(), appID+".desktop")
}

func (a *App) starDesktopPath() string {
	return filepath.Join(a.userApplicationsDir(), "citizen-launcher-star-citizen.desktop")
}

// migrateUserInstall removes an old ~/.local shadow install when a native
// system package is now authoritative. It only removes files that identify themselves
// as Citizen Launcher and are not newer than the installed package.
func (a *App) migrateUserInstall() {
	if os.Geteuid() == 0 {
		return
	}
	a.migrateUserInstallUnlocked()
}

func (a *App) migrateUserInstallUnlocked() {
	if installedPackageVersion() == "" {
		return
	}
	installed := installedPackageVersion()
	oldBin := filepath.Join(a.home, ".local", "bin", "citizen-launcher")
	if oldBin != a.selfPath {
		if out, err := exec.Command(oldBin, "--version").CombinedOutput(); err == nil {
			v := strings.TrimSpace(string(out))
			if v != "" && compareVersions(v, installed) <= 0 {
				if err := os.Remove(oldBin); err == nil {
					a.logf("removed shadowed legacy user binary %s version=%s", oldBin, v)
				}
			}
		}
	}

	// A user-local desktop entry overrides the system package entry. Remove only
	// the legacy Citizen Launcher entry, never arbitrary user shortcuts.
	if b, err := os.ReadFile(a.mainDesktopPath()); err == nil {
		s := string(b)
		if strings.Contains(s, "Name=Citizen Launcher") && strings.Contains(s, "citizen-launcher") {
			_ = os.Remove(a.mainDesktopPath())
			a.logf("removed legacy user desktop entry so packaged desktop entry can take precedence")
		}
	}

	// Rewrite old per-user systemd units to the stable package binary if they exist.
	service := filepath.Join(a.home, ".config", "systemd", "user", "citizen-launcher-maintenance.service")
	if b, err := os.ReadFile(service); err == nil && strings.Contains(string(b), ".local/lib/citizen-launcher") {
		if err := a.installService(); err != nil {
			a.logf("legacy user service migration warning: %v", err)
		}
	}
}

// repairDesktopIntegration repairs files created by older versions. It is safe
// to call on every user-level launcher start and does not touch game data.
func (a *App) repairDesktopIntegration() {
	if os.Geteuid() == 0 {
		return
	}
	a.repairDesktopIntegrationUnlocked()
}

func (a *App) repairDesktopIntegrationUnlocked() {
	gc := a.loadGameConfig()
	if _, err := os.Stat(gc.LauncherEXE); err == nil || gc.InstalledAt != "" {
		if err := a.writeOwnedLaunchFiles(gc); err != nil {
			a.logf("desktop integration repair warning: %v", err)
		}
	}
}

func (a *App) choosePrefixDirectory() (string, error) {
	gc := a.loadGameConfig()
	if prefixInitialized(gc.Prefix) || gc.LauncherStateReady() {
		return "", errors.New("Der Installationsort kann nach der Einrichtung nicht automatisch verschoben werden")
	}
	start := filepath.Dir(gc.Prefix)
	var cmd *exec.Cmd
	if p, err := exec.LookPath("kdialog"); err == nil {
		cmd = exec.Command(p, "--getexistingdirectory", start, "--title", "Star-Citizen-Installationsordner wählen")
	} else if p, err := exec.LookPath("zenity"); err == nil {
		cmd = exec.Command(p, "--file-selection", "--directory", "--title=Star-Citizen-Installationsordner wählen", "--filename="+start+string(os.PathSeparator))
	} else {
		return "", errors.New("Kein grafischer Ordnerdialog gefunden (kdialog oder zenity)")
	}
	out, err := cmd.Output()
	if err != nil {
		return "", errors.New("Ordnerauswahl abgebrochen")
	}
	parent := strings.TrimSpace(string(out))
	if parent == "" || !filepath.IsAbs(parent) {
		return "", errors.New("Ungültiger Installationsordner")
	}
	prefix := filepath.Clean(filepath.Join(parent, "star-citizen"))
	if err := validatePrefixPath(prefix); err != nil {
		return "", err
	}
	gc.Prefix = prefix
	gc.GameDir = filepath.Join(prefix, "drive_c", "Program Files", "Roberts Space Industries", "StarCitizen")
	gc.LauncherEXE = filepath.Join(prefix, "drive_c", "Program Files", "Roberts Space Industries", "RSI Launcher", "RSI Launcher.exe")
	if err := a.saveGameConfig(gc); err != nil {
		return "", err
	}
	return prefix, nil
}

func (g GameConfig) LauncherStateReady() bool {
	_, err := os.Stat(g.LauncherEXE)
	return err == nil
}

// setLaunchNoFile raises the soft file limit for Wine when the session hard
// limit already permits it. Package installations also ship a PAM limits file
// so future sessions get the required hard limit automatically.
func setLaunchNoFile() error {
	var lim syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &lim); err != nil {
		return err
	}
	if lim.Max < requiredNoFile {
		return fmt.Errorf("Hard-Limit für offene Dateien ist %d, benötigt werden %d", lim.Max, requiredNoFile)
	}
	if lim.Cur < requiredNoFile {
		lim.Cur = requiredNoFile
		if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &lim); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) prepareSystem(pid int) error {
	if os.Geteuid() == 0 {
		return prepareSystemRoot(pid)
	}
	if !commandExists("pkexec") {
		return errors.New("Für die einmalige Systemvorbereitung wird Polkit/pkexec benötigt")
	}
	self := a.stableExecutable()
	targetPID := pid
	if targetPID <= 1 {
		targetPID = os.Getpid()
	}
	args := []string{self, "prepare-system", "--root", "--pid", strconv.Itoa(targetPID)}
	cmd := exec.Command("pkexec", args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

func prepareSystemRoot(targetPID int) error {
	if os.Geteuid() != 0 {
		return errors.New("prepare-system --root requires root")
	}
	const sysctlPath = "/etc/sysctl.d/90-citizen-launcher.conf"
	const limitsPath = "/etc/security/limits.d/90-citizen-launcher.conf"
	if err := os.WriteFile(sysctlPath, []byte("# Citizen Launcher / Star Citizen\nvm.max_map_count = 16777216\n"), 0o644); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(limitsPath), 0o755); err != nil {
		return err
	}
	limits := "# Citizen Launcher / Star Citizen\n* soft nofile 524288\n* hard nofile 524288\n"
	if err := os.WriteFile(limitsPath, []byte(limits), 0o644); err != nil {
		return err
	}
	if commandExists("sysctl") {
		if out, err := exec.Command("sysctl", "-w", "vm.max_map_count=16777216").CombinedOutput(); err != nil {
			return fmt.Errorf("vm.max_map_count: %s", formatCommandFailure(err, out))
		}
	}
	// Best-effort immediate upgrade of the caller's rlimit. Validate ownership
	// using PKEXEC_UID before touching another process.
	if targetPID > 1 && commandExists("prlimit") {
		uidText := strings.TrimSpace(os.Getenv("PKEXEC_UID"))
		if uidText != "" {
			status, _ := os.ReadFile(filepath.Join("/proc", strconv.Itoa(targetPID), "status"))
			if strings.Contains(string(status), "Uid:\t"+uidText+"\t") {
				_ = exec.Command("prlimit", "--pid", strconv.Itoa(targetPID), "--nofile=524288:524288").Run()
			}
		}
	}
	return nil
}

func (a *App) ensureSystemPrepared() error {
	r := a.systemReadiness(a.loadGameConfig().Prefix)
	if r.MapCountOK && r.NoFileHardOK {
		return nil
	}
	if err := a.prepareSystem(os.Getpid()); err != nil {
		return err
	}
	// vm.max_map_count applies immediately. PAM limits may require a new login,
	// but try to raise the current process immediately when permitted.
	_ = setLaunchNoFile()
	time.Sleep(100 * time.Millisecond)
	r = a.systemReadiness(a.loadGameConfig().Prefix)
	if !r.MapCountOK {
		return fmt.Errorf("vm.max_map_count ist weiterhin zu niedrig (%d)", r.MapCount)
	}
	return nil
}
