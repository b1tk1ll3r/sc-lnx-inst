package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type guiInstanceInfo struct {
	PID     int    `json:"pid"`
	URL     string `json:"url"`
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

func (a *App) existingGUIURL() (string, bool) {
	for i := 0; i < 12; i++ {
		b, err := os.ReadFile(a.guiInfoPath())
		if err == nil {
			var info guiInstanceInfo
			if json.Unmarshal(b, &info) == nil && info.URL != "" && processAlive(info.PID) {
				client := &http.Client{Timeout: 500 * time.Millisecond}
				resp, err := client.Get(strings.TrimRight(info.URL, "/") + "/api/ping")
				if err == nil {
					_ = resp.Body.Close()
					if resp.StatusCode == http.StatusOK {
						return info.URL, true
					}
				}
			}
		}
		time.Sleep(150 * time.Millisecond)
	}
	return "", false
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
