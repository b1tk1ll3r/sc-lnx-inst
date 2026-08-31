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
	f, err := os.Create(target)
	if err != nil {
		return "", err
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	closeAll := func() { _ = tw.Close(); _ = gz.Close(); _ = f.Close() }
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
	addJSON("platform.json", detectPlatform())
	addJSON("game-status.json", a.gameStatus())
	if st, _ := a.status(false); true {
		addJSON("launcher-status.json", st)
	}
	addJSON("game-config.json", a.loadGameConfig())
	_ = add("README.txt", []byte("Citizen Launcher support bundle\nID: "+id+"\nPersonal paths, email/IP/MAC and common tokens are redacted on a best-effort basis.\n"))

	for name, cmd := range map[string][]string{
		"uname.txt":         {"uname", "-a"},
		"os-release.txt":    {"cat", "/etc/os-release"},
		"memory.txt":        {"free", "-h"},
		"disk.txt":          {"df", "-hT"},
		"pci.txt":           {"lspci", "-nnk"},
		"vulkan.txt":        {"vulkaninfo", "--summary"},
		"systemd-timer.txt": {"systemctl", "--user", "status", "citizen-launcher-maintenance.timer", "--no-pager"},
	} {
		if _, err := exec.LookPath(cmd[0]); err != nil {
			continue
		}
		out, _ := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
		_ = add(name, out)
	}
	for name, path := range map[string]string{
		"launcher.log":     a.logPath,
		"rsi-launcher.log": filepath.Join(a.stateDir, "rsi-launcher.log"),
	} {
		if b, err := tailFile(path, 700000); err == nil {
			_ = add(name, b)
		}
	}
	return target, nil
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
		s = regexp.MustCompile(`(?i)\\b`+regexp.QuoteMeta(u.Username)+`\\b`).ReplaceAllString(s, "<user>")
	}
	if h, _ := os.Hostname(); h != "" {
		s = strings.ReplaceAll(s, h, "<host>")
	}
	patterns := []struct{ re, repl string }{
		{`(?i)\\b[A-Z0-9._%+-]+@[A-Z0-9.-]+\\.[A-Z]{2,}\\b`, "<email>"},
		{`\\b(?:\\d{1,3}\\.){3}\\d{1,3}\\b`, "<ip>"},
		{`(?i)\\b(?:[0-9a-f]{2}:){5}[0-9a-f]{2}\\b`, "<mac>"},
		{`(?i)Bearer\\s+[A-Za-z0-9._~+/=-]+`, "Bearer <redacted>"},
		{`(?i)(token|secret|password|passwd|cookie|session)\\s*[:=]\\s*[^\\s,;]+`, "$1=<redacted>"},
	}
	for _, p := range patterns {
		s = regexp.MustCompile(p.re).ReplaceAllString(s, p.repl)
	}
	return []byte(s)
}
