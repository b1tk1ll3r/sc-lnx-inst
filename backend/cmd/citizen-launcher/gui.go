package main

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

//go:embed web/*
var guiFiles embed.FS

type GUIStatus struct {
	Version           string         `json:"version"`
	InstalledVersion  string         `json:"installed_version,omitempty"`
	RestartRequired   bool           `json:"restart_required"`
	PackageAutoUpdate bool           `json:"package_auto_update"`
	Platform          PlatformStatus `json:"platform"`
	Game              GameStatus     `json:"game"`
	Launcher          Status         `json:"launcher"`
}

type GUIJob struct {
	ID       string `json:"id"`
	Action   string `json:"action"`
	State    string `json:"state"`
	Message  string `json:"message,omitempty"`
	Error    string `json:"error,omitempty"`
	Started  string `json:"started"`
	Finished string `json:"finished,omitempty"`
}

type jobStore struct {
	mu   sync.Mutex
	jobs map[string]*GUIJob
}

func friendlyActionError(action string, err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	switch {
	case strings.Contains(s, "Windows-Komponente"):
		return "Eine Windows-Komponente konnte nicht eingerichtet werden. Der technische Fehler ist unten verfügbar."
	case strings.Contains(s, "RSI latest.yml") || strings.Contains(s, "RSI Installer"):
		return "Die aktuelle RSI-Launcher-Version konnte nicht zuverlässig ermittelt werden."
	case strings.Contains(s, "RSI Launcher Installation"):
		return "Der RSI Launcher konnte nicht installiert oder aktualisiert werden."
	case strings.Contains(strings.ToLower(s), "vulkan"):
		return "Vulkan ist auf diesem System nicht spielbereit."
	case strings.Contains(strings.ToLower(s), "wine"):
		return "Der Wine-Stack konnte den Selbsttest oder die Reparatur nicht abschließen."
	default:
		if action == "repair" {
			return "Die automatische Reparatur konnte nicht vollständig abgeschlossen werden."
		}
		return "Die Aktion konnte nicht vollständig abgeschlossen werden."
	}
}

func (a *App) runGUI(args []string) error {
	noOpen := hasArg(args, "--no-open")
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	tokenBytes := make([]byte, 18)
	_, _ = rand.Read(tokenBytes)
	token := hex.EncodeToString(tokenBytes)
	base := "/" + token
	jobs := &jobStore{jobs: map[string]*GUIJob{}}
	mux := http.NewServeMux()
	sub, _ := fs.Sub(guiFiles, "web")
	mux.Handle(base+"/assets/", http.StripPrefix(base+"/assets/", http.FileServer(http.FS(sub))))
	mux.HandleFunc(base+"/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != base+"/" {
			http.NotFound(w, r)
			return
		}
		b, err := fs.ReadFile(sub, "index.html")
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		b = []byte(strings.ReplaceAll(string(b), "__BASE__", base))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(b)
	})
	jsonOut := func(w http.ResponseWriter, v any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}
	mux.HandleFunc(base+"/api/status", func(w http.ResponseWriter, r *http.Request) {
		st, _ := a.status(false)
		su, _ := a.selfUpdateStatus(false)
		jsonOut(w, GUIStatus{
			Version: appVersion, InstalledVersion: su.Installed, RestartRequired: su.RestartNeeded,
			PackageAutoUpdate: packageAutoUpdateActive(), Platform: detectPlatform(), Game: a.gameStatus(), Launcher: st,
		})
	})
	mux.HandleFunc(base+"/api/job/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, base+"/api/job/")
		jobs.mu.Lock()
		j := jobs.jobs[id]
		jobs.mu.Unlock()
		if j == nil {
			http.NotFound(w, r)
			return
		}
		jsonOut(w, j)
	})
	mux.HandleFunc(base+"/api/action/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "POST required", 405)
			return
		}
		action := strings.TrimPrefix(r.URL.Path, base+"/api/action/")
		if action == "restart" {
			target := a.selfPath
			if installedPackageVersion() != "" {
				if _, err := os.Stat("/usr/bin/citizen-launcher"); err == nil {
					target = "/usr/bin/citizen-launcher"
				}
			}
			if target == "" {
				http.Error(w, "launcher executable not found", 500)
				return
			}
			if err := exec.Command(target, "gui").Start(); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			jsonOut(w, map[string]string{"state": "restarting"})
			go func() { time.Sleep(450 * time.Millisecond); os.Exit(0) }()
			return
		}
		if action == "launch" {
			if err := a.gameLaunch(); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			jsonOut(w, map[string]string{"state": "started"})
			return
		}
		idBytes := make([]byte, 8)
		_, _ = rand.Read(idBytes)
		id := hex.EncodeToString(idBytes)
		j := &GUIJob{ID: id, Action: action, State: "running", Started: time.Now().Format(time.RFC3339)}
		jobs.mu.Lock()
		jobs.jobs[id] = j
		jobs.mu.Unlock()
		go func() {
			var err error
			var msg string
			switch action {
			case "setup":
				if e := a.enableAutopilot(); e != nil {
					a.logf("autopilot enable during setup: %v", e)
				}
				err = a.gameInstall()
			case "repair":
				err = a.gameRepair()
			case "maintain":
				err = a.maintainGamingStack()
			case "doctor":
				err = a.wineDoctor()
			case "autopilot-enable":
				err = a.enableAutopilot()
			case "autopilot-disable":
				err = a.disableAutopilot()
			case "support":
				var p string
				p, err = a.createSupportBundle()
				msg = p
			case "integration-update":
				err = a.update(false)
			default:
				err = errors.New("unknown action")
			}
			jobs.mu.Lock()
			defer jobs.mu.Unlock()
			j.Finished = time.Now().Format(time.RFC3339)
			if err != nil {
				j.State = "error"
				j.Message = friendlyActionError(action, err)
				j.Error = err.Error()
			} else {
				j.State = "done"
				j.Message = msg
			}
		}()
		jsonOut(w, j)
	})
	mux.HandleFunc(base+"/api/open/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "POST required", 405)
			return
		}
		what := strings.TrimPrefix(r.URL.Path, base+"/api/open/")
		var path string
		switch what {
		case "downloads":
			path = filepath.Join(a.home, "Downloads")
		case "logs":
			path = a.stateDir
		default:
			http.NotFound(w, r)
			return
		}
		_ = exec.Command("xdg-open", path).Start()
		jsonOut(w, map[string]string{"state": "opened"})
	})
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	url := "http://" + listener.Addr().String() + base + "/"
	fmt.Println(url)
	if !noOpen {
		if err := openGUIURL(url, a.cacheDir); err != nil {
			_ = listener.Close()
			return err
		}
	}
	return srv.Serve(listener)
}

func openGUIURL(url, cache string) error {
	candidates := []string{"chromium", "chromium-browser", "google-chrome", "google-chrome-stable", "brave-browser", "brave", "vivaldi", "vivaldi-stable"}
	for _, name := range candidates {
		if p, err := exec.LookPath(name); err == nil {
			cmd := exec.Command(p, "--app="+url, "--new-window")
			cmd.Stdout = nil
			cmd.Stderr = nil
			if cmd.Start() == nil {
				return nil
			}
		}
	}
	if p, err := exec.LookPath("xdg-open"); err == nil {
		return exec.Command(p, url).Start()
	}
	if p, err := exec.LookPath("gio"); err == nil {
		return exec.Command(p, "open", url).Start()
	}
	return fmt.Errorf("no browser opener found; open %s manually", url)
}
