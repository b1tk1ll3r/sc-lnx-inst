package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// guiInstanceInfo lets a second `citizen-launcher gui` call ask the running
// instance for a fresh one-time login URL. The file is 0600 in the user's
// state dir; Control is the only credential that can mint login URLs.
type guiInstanceInfo struct {
	PID     int    `json:"pid"`
	Addr    string `json:"addr"`
	Control string `json:"control"`
	Started string `json:"started"`
}

func (a *App) acquireGUILock() (*os.File, bool, error) {
	if err := os.MkdirAll(a.stateDir, 0o755); err != nil {
		return nil, false, err
	}
	f, err := os.OpenFile(filepath.Join(a.stateDir, "gui.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, false, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return f, true, nil
}

func releaseFileLock(f *os.File) {
	if f == nil {
		return
	}
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	_ = f.Close()
}

func (a *App) guiInfoPath() string { return filepath.Join(a.stateDir, "gui-instance.json") }

func (a *App) writeGUIInfo(info guiInstanceInfo) error {
	b, _ := json.MarshalIndent(info, "", "  ")
	b = append(b, '\n')
	return atomicWriteFile(a.guiInfoPath(), b, 0o600)
}

// existingGUIURL asks the running GUI instance for a new one-time login URL.
func (a *App) existingGUIURL() (string, bool) {
	for i := 0; i < 12; i++ {
		if u, ok := a.requestGUILoginURL(); ok {
			return u, true
		}
		time.Sleep(150 * time.Millisecond)
	}
	return "", false
}

func (a *App) requestGUILoginURL() (string, bool) {
	b, err := os.ReadFile(a.guiInfoPath())
	if err != nil {
		return "", false
	}
	var info guiInstanceInfo
	if json.Unmarshal(b, &info) != nil || info.Addr == "" || info.Control == "" || !processAlive(info.PID) {
		return "", false
	}
	host, _, err := net.SplitHostPort(info.Addr)
	if err != nil || host != "127.0.0.1" {
		return "", false
	}
	req, err := http.NewRequest(http.MethodPost, "http://"+info.Addr+guiControlPath, nil)
	if err != nil {
		return "", false
	}
	req.Header.Set(guiControlHeader, info.Control)
	resp, err := (&http.Client{Timeout: 500 * time.Millisecond}).Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", false
	}
	var out struct {
		URL string `json:"url"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&out) != nil || !strings.HasPrefix(out.URL, "http://"+info.Addr+"/") {
		return "", false
	}
	return out.URL, true
}

func processAlive(pid int) bool {
	if pid <= 1 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}

func atomicWriteFile(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err := f.Chmod(mode); err != nil {
		_ = f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

var (
	ErrLauncherAlreadyRunning = errors.New("RSI Launcher läuft bereits")
	ErrGameAlreadyRunning     = errors.New("Star Citizen läuft bereits")
)

func processHasEnvValue(data []byte, key, value string) bool {
	want := key + "=" + value
	for _, field := range strings.Split(string(data), "\x00") {
		if field == want {
			return true
		}
	}
	return false
}

type prefixProcessState struct {
	RSI  bool
	Game bool
}

func (a *App) prefixProcessState(prefix string) prefixProcessState {
	var state prefixProcessState
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return state
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := strconv.Atoi(e.Name()); err != nil {
			continue
		}
		env, err := os.ReadFile(filepath.Join("/proc", e.Name(), "environ"))
		if err != nil || !processHasEnvValue(env, "WINEPREFIX", prefix) {
			continue
		}
		cmdline, _ := os.ReadFile(filepath.Join("/proc", e.Name(), "cmdline"))
		cmd := strings.ToLower(strings.ReplaceAll(string(cmdline), "\x00", " "))
		switch {
		case strings.Contains(cmd, "starcitizen.exe") || strings.Contains(cmd, "star citizen\\live\\bin64"):
			state.Game = true
		case strings.Contains(cmd, "rsi launcher") || strings.Contains(cmd, "rsilauncher"):
			state.RSI = true
		}
		if state.RSI && state.Game {
			return state
		}
	}
	return state
}

func (a *App) rsiLauncherRunning(prefix string) bool { return a.prefixProcessState(prefix).RSI }
func (a *App) starCitizenRunning(prefix string) bool { return a.prefixProcessState(prefix).Game }

func (a *App) stackOperationLock(wait time.Duration) (*os.File, error) {
	if err := os.MkdirAll(a.stateDir, 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(a.stateDir, "maintenance.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(wait)
	for {
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return f, nil
		}
		if time.Now().After(deadline) {
			_ = f.Close()
			return nil, fmt.Errorf("Gaming-Stack wird gerade gewartet; bitte einen Moment warten")
		}
		time.Sleep(200 * time.Millisecond)
	}
}
