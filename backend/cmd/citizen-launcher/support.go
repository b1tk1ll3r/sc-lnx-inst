package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

func (a *App) createSupportBundle() (string, error) {
	stamp := time.Now().Format("20060102-150405")
	id := fmt.Sprintf("CL-%s-%d", stamp, os.Getpid())
	downloads := filepath.Join(a.home, "Downloads")
	if out, err := exec.Command("xdg-user-dir", "DOWNLOAD").Output(); err == nil && strings.TrimSpace(string(out)) != "" {
		downloads = strings.TrimSpace(string(out))
	}
	if err := os.MkdirAll(downloads, 0o755); err != nil {
		return "", err
	}
	target := filepath.Join(downloads, "Citizen-Launcher-Support-"+id+".tar.gz")
	// O_EXCL + 0600: the bundle contains logs and must not be world-readable.
	f, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", err
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	closed := false
	closeAll := func() {
		if closed {
			return
		}
		closed = true
		_ = tw.Close()
		_ = gz.Close()
		_ = f.Close()
	}
	defer closeAll()

	add := func(name string, data []byte) error {
		data = sanitizeSupport(data, a.home)
		h := &tar.Header{Name: name, Mode: 0o600, Size: int64(len(data)), ModTime: time.Now()}
		if err := tw.WriteHeader(h); err != nil {
			return err
		}
		_, err := tw.Write(data)
		return err
	}
	addJSON := func(name string, v any) {
		b, _ := json.MarshalIndent(v, "", "  ")
		b = append(b, '\n')
		_ = add(name, b)
	}

	gc := a.loadGameConfig()
	addJSON("platform.json", detectPlatform())
	addJSON("game-status.json", a.gameStatus())
	st, _ := a.status(false)
	addJSON("launcher-status.json", st)
	addJSON("self-update.json", func() SelfUpdateStatus { s, _ := a.selfUpdateStatus(false); return s }())
	addJSON("game-config.json", gc)
	addJSON("system-readiness.json", a.systemReadiness(gc.Prefix))
	_ = add("README.txt", []byte("Citizen Launcher support bundle\nID: "+id+"\nPersonal paths, email/IP/MAC and common auth/token values are redacted on a best-effort basis. Review before sharing if desired.\n"))

	for name, cmd := range map[string][]string{
		"uname.txt":                {"uname", "-a"},
		"os-release.txt":           {"cat", "/etc/os-release"},
		"memory.txt":               {"free", "-h"},
		"swap.txt":                 {"swapon", "--show"},
		"disk.txt":                 {"df", "-hT"},
		"mount.txt":                {"findmnt", "-T", gc.Prefix, "-o", "TARGET,FSTYPE,OPTIONS"},
		"pci.txt":                  {"lspci", "-nnk"},
		"vulkan.txt":               {"vulkaninfo", "--summary"},
		"limits.txt":               {"sh", "-c", "printf 'vm.max_map_count='; cat /proc/sys/vm/max_map_count; printf 'nofile='; ulimit -Sn; printf 'nofile-hard='; ulimit -Hn"},
		"maintenance-timer.txt":    {"systemctl", "--user", "status", "citizen-launcher-maintenance.timer", "--no-pager"},
		"package-update-timer.txt": {"systemctl", "status", "citizen-launcher-self-update.timer", "--no-pager"},
	} {
		if _, err := exec.LookPath(cmd[0]); err != nil {
			continue
		}
		out, _ := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
		_ = add(name, out)
	}

	for name, path := range map[string]string{
		"launcher.log": a.logPath,
		"rsi-wine.log": filepath.Join(a.stateDir, "rsi-launcher.log"),
	} {
		if b, err := tailFile(path, 900000); err == nil {
			_ = add(name, b)
		}
	}

	// Product-owned desktop integration is useful for diagnosing launch failures.
	for name, path := range map[string]string{
		"desktop-citizen-launcher.txt": a.mainDesktopPath(),
		"desktop-star-citizen.txt":     a.starDesktopPath(),
		"launch-wrapper.txt":           filepath.Join(a.dataDir, "bin", "star-citizen-launch"),
	} {
		if b, err := os.ReadFile(path); err == nil {
			_ = add(name, b)
		}
	}

	// Current RSI launcher / game / EAC logs, capped and sanitized.
	for name, path := range a.gameLogCandidates(gc) {
		if b, err := tailFile(path, 1200000); err == nil {
			_ = add(name, b)
		}
	}

	closeAll()
	return target, nil
}

func (a *App) gameLogCandidates(gc GameConfig) map[string]string {
	out := map[string]string{}
	usersRoot := filepath.Join(gc.Prefix, "drive_c", "users")
	users, _ := os.ReadDir(usersRoot)
	for _, u := range users {
		if !u.IsDir() || strings.EqualFold(u.Name(), "Public") {
			continue
		}
		roam := filepath.Join(usersRoot, u.Name(), "AppData", "Roaming")
		if _, err := os.Stat(filepath.Join(roam, "rsilauncher", "logs", "log.log")); err == nil {
			out["rsi-internal.log"] = filepath.Join(roam, "rsilauncher", "logs", "log.log")
		}
		matches, _ := filepath.Glob(filepath.Join(roam, "EasyAntiCheat", "*", "*", "anticheatlauncher.log"))
		sort.Strings(matches)
		if len(matches) > 0 {
			out["eac.log"] = matches[len(matches)-1]
		}
	}
	gameLog := filepath.Join(gc.GameDir, "LIVE", "Game.log")
	if _, err := os.Stat(gameLog); err == nil {
		out["game.log"] = gameLog
	}
	return out
}

func tailFile(path string, max int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	off := int64(0)
	if st.Size() > max {
		off = st.Size() - max
	}
	if _, err = f.Seek(off, 0); err != nil {
		return nil, err
	}
	var b bytes.Buffer
	_, err = b.ReadFrom(f)
	return b.Bytes(), err
}

func sanitizeSupport(in []byte, home string) []byte {
	s := string(in)
	if home != "" {
		s = strings.ReplaceAll(s, home, "~")
	}
	if u, err := user.Current(); err == nil && u.Username != "" {
		s = regexp.MustCompile(`(?i)\b`+regexp.QuoteMeta(u.Username)+`\b`).ReplaceAllString(s, "<user>")
	}
	if h, _ := os.Hostname(); h != "" {
		s = strings.ReplaceAll(s, h, "<host>")
	}
	patterns := []struct{ re, repl string }{
		{`(?i)\b[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}\b`, "<email>"},
		{`\b(?:\d{1,3}\.){3}\d{1,3}\b`, "<ip>"},
		{`(?i)\b(?:[0-9a-f]{2}:){5}[0-9a-f]{2}\b`, "<mac>"},
		{`(?i)\bBearer\s+[A-Za-z0-9._~+/=-]+`, "Bearer <redacted>"},
		{`\beyJ[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`, "<jwt>"},
		{`(?i)"?(authorization|token|access_token|refresh_token|secret|password|passwd|cookie|session|session_id|sessionid|handle|nickname|geid)"?\s*[:=]\s*"?[^\s",;]+"?`, "$1=<redacted>"},
		{`(?i)([?&](?:token|access_token|refresh_token|session|auth|key|secret)=)[^&#\s]+`, "$1<redacted>"},
	}
	for _, p := range patterns {
		s = regexp.MustCompile(p.re).ReplaceAllString(s, p.repl)
	}
	return []byte(s)
}
