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
	"syscall"
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
	ActiveJob         *GUIJob        `json:"active_job,omitempty"`
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
	mu       sync.Mutex
	jobs     map[string]*GUIJob
	activeID string
}

func (s *jobStore) active() *GUIJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.activeID == "" {
		return nil
	}
	j := s.jobs[s.activeID]
	if j == nil || j.State != "running" {
		s.activeID = ""
		return nil
	}
	copy := *j
	return &copy
}

func (s *jobStore) start(action string) (*GUIJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.activeID != "" {
		if j := s.jobs[s.activeID]; j != nil && j.State == "running" {
			return nil, fmt.Errorf("%s läuft bereits", friendlyActionName(j.Action))
		}
		s.activeID = ""
	}
	idBytes := make([]byte, 8)
	if _, err := rand.Read(idBytes); err != nil {
		return nil, fmt.Errorf("sichere Job-ID konnte nicht erzeugt werden: %w", err)
	}
	j := &GUIJob{ID: hex.EncodeToString(idBytes), Action: action, State: "running", Started: time.Now().Format(time.RFC3339)}
	s.jobs[j.ID] = j
	s.activeID = j.ID
	return j, nil
}

func (s *jobStore) finish(id string, err error, msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j := s.jobs[id]
	if j == nil {
		return
	}
	j.Finished = time.Now().Format(time.RFC3339)
	if err != nil {
		j.State = "error"
		j.Message = friendlyActionError(j.Action, err)
		j.Error = err.Error()
	} else {
		j.State = "done"
		j.Message = msg
	}
	if s.activeID == id {
		s.activeID = ""
	}
	// Bound memory for a GUI process that may stay open for days.
	if len(s.jobs) > 60 {
		for key, old := range s.jobs {
			if old.State != "running" && key != id {
				delete(s.jobs, key)
			}
			if len(s.jobs) <= 30 {
				break
			}
		}
	}
}

func (s *jobStore) get(id string) *GUIJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	j := s.jobs[id]
	if j == nil {
		return nil
	}
	copy := *j
	return &copy
}

func friendlyActionName(action string) string {
	switch action {
	case "setup":
		return "Einrichtung"
	case "repair":
		return "Reparatur"
	case "maintain":
		return "Gaming-Stack-Wartung"
	case "doctor":
		return "Wine-Selbsttest"
	case "support":
		return "Support-Paket"
	case "prepare-system":
		return "Systemvorbereitung"
	case "choose-prefix":
		return "Ordnerauswahl"
	default:
		return "Eine Aktion"
	}
}

func friendlyActionError(action string, err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	switch {
	case errors.Is(err, ErrLauncherAlreadyRunning):
		return "Der RSI Launcher läuft bereits."
	case strings.Contains(s, "PowerShell-Kompatibilität") || strings.Contains(s, "PowerShell Selbsttest"):
		return "Die für den RSI Launcher benötigte Windows-Kompatibilität konnte nicht vollständig eingerichtet werden. Citizen Launcher hat keine Spieldaten verändert."
	case strings.Contains(s, "Windows-Komponente"):
		return "Eine Windows-Komponente konnte nicht eingerichtet werden. Die technischen Details sind unten verfügbar."
	case strings.Contains(s, "RSI latest.yml") || strings.Contains(s, "RSI Installer"):
		return "Die aktuelle RSI-Launcher-Version konnte nicht zuverlässig ermittelt werden."
	case strings.Contains(s, "RSI Launcher Installation"):
		return "Der RSI Launcher konnte nicht installiert oder aktualisiert werden."
	case strings.Contains(strings.ToLower(s), "vulkan"):
		return "Vulkan ist auf diesem System nicht spielbereit."
	case strings.Contains(strings.ToLower(s), "nofile") || strings.Contains(strings.ToLower(s), "max_map_count"):
		return "Eine notwendige Linux-Systemeinstellung konnte nicht vorbereitet werden."
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
	guiLock, acquired, err := a.acquireGUILock()
	if err != nil {
		return err
	}
	if !acquired {
		if existing, ok := a.existingGUIURL(); ok {
			fmt.Println(existing)
			if noOpen {
				return nil
			}
			return openGUIURL(existing, a.cacheDir)
		}
		return errors.New("Citizen Launcher läuft bereits, reagiert aber noch nicht. Bitte kurz warten und erneut öffnen.")
	}
	defer releaseFileLock(guiLock)
	defer os.Remove(a.guiInfoPath())

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	tokenBytes := make([]byte, 18)
	if _, err := rand.Read(tokenBytes); err != nil {
		_ = listener.Close()
		return fmt.Errorf("sicheres GUI-Token konnte nicht erzeugt werden: %w", err)
	}
	base := "/" + hex.EncodeToString(tokenBytes)
	jobs := &jobStore{jobs: map[string]*GUIJob{}}
	mux := http.NewServeMux()
	sub, _ := fs.Sub(guiFiles, "web")

	var activityMu sync.Mutex
	lastActivity := time.Now()
	touch := func() { activityMu.Lock(); lastActivity = time.Now(); activityMu.Unlock() }
	secure := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			touch()
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Referrer-Policy", "no-referrer")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
			next.ServeHTTP(w, r)
		})
	}

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
	mux.HandleFunc(base+"/api/ping", func(w http.ResponseWriter, r *http.Request) {
		jsonOut(w, map[string]any{"ok": true, "version": appVersion})
	})
	mux.HandleFunc(base+"/api/status", func(w http.ResponseWriter, r *http.Request) {
		st, _ := a.status(false)
		su, _ := a.selfUpdateStatus(false)
		jsonOut(w, GUIStatus{Version: appVersion, InstalledVersion: su.Installed, RestartRequired: su.RestartNeeded,
			PackageAutoUpdate: packageAutoUpdateActive(), Platform: detectPlatform(), Game: a.gameStatus(), Launcher: st, ActiveJob: jobs.active()})
	})
	mux.HandleFunc(base+"/api/job/", func(w http.ResponseWriter, r *http.Request) {
		j := jobs.get(strings.TrimPrefix(r.URL.Path, base+"/api/job/"))
		if j == nil {
			http.NotFound(w, r)
			return
		}
		jsonOut(w, j)
	})

	var srv *http.Server
	mux.HandleFunc(base+"/api/action/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", 405)
			return
		}
		action := strings.TrimPrefix(r.URL.Path, base+"/api/action/")
		if action == "restart" {
			target := a.stableExecutable()
			if target == "" {
				http.Error(w, "launcher executable not found", 500)
				return
			}
			jsonOut(w, map[string]string{"state": "restarting"})
			go func() {
				time.Sleep(350 * time.Millisecond)
				_ = listener.Close()
				if err := syscall.Exec(target, []string{target, "gui"}, os.Environ()); err != nil {
					a.logf("GUI restart exec failed: %v", err)
					os.Exit(1)
				}
			}()
			return
		}
		if action == "launch" {
			if active := jobs.active(); active != nil {
				http.Error(w, friendlyActionName(active.Action)+" läuft noch", 409)
				return
			}
			if err := a.gameLaunch(); err != nil {
				if errors.Is(err, ErrLauncherAlreadyRunning) {
					jsonOut(w, map[string]string{"state": "launcher-running"})
					return
				}
				if errors.Is(err, ErrGameAlreadyRunning) {
					jsonOut(w, map[string]string{"state": "game-running"})
					return
				}
				http.Error(w, err.Error(), 500)
				return
			}
			jsonOut(w, map[string]string{"state": "started"})
			return
		}

		j, err := jobs.start(action)
		if err != nil {
			http.Error(w, err.Error(), 409)
			return
		}
		go func() {
			var runErr error
			var msg string
			switch action {
			case "setup":
				if e := a.enableAutopilot(); e != nil {
					a.logf("autopilot enable during setup: %v", e)
				}
				runErr = a.gameInstall()
			case "repair":
				runErr = a.gameRepair()
			case "maintain":
				runErr = a.maintainGamingStack()
			case "doctor":
				runErr = a.wineDoctor()
			case "prepare-system":
				runErr = a.ensureSystemPrepared()
			case "choose-prefix":
				msg, runErr = a.choosePrefixDirectory()
			case "autopilot-enable":
				runErr = a.enableAutopilot()
			case "autopilot-disable":
				runErr = a.disableAutopilot()
			case "support":
				msg, runErr = a.createSupportBundle()
			case "integration-update":
				runErr = a.update(false)
			default:
				runErr = errors.New("unknown action")
			}
			jobs.finish(j.ID, runErr, msg)
		}()
		jsonOut(w, j)
	})
	mux.HandleFunc(base+"/api/open/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
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
		if err := exec.Command("xdg-open", path).Start(); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		jsonOut(w, map[string]string{"state": "opened"})
	})

	srv = &http.Server{Handler: secure(mux), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	url := "http://" + listener.Addr().String() + base + "/"
	if err := a.writeGUIInfo(guiInstanceInfo{PID: os.Getpid(), URL: url, Started: time.Now().Format(time.RFC3339)}); err != nil {
		_ = listener.Close()
		return err
	}
	fmt.Println(url)
	if !noOpen {
		if err := openGUIURL(url, a.cacheDir); err != nil {
			_ = listener.Close()
			return err
		}
	}

	// Browser app windows do not give us a portable close notification. Status
	// polling acts as a heartbeat. Never stop the backend while a long-running job is active.
	go func() {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for range t.C {
			activityMu.Lock()
			idle := time.Since(lastActivity)
			activityMu.Unlock()
			if idle > 10*time.Minute && jobs.active() == nil {
				_ = srv.Close()
				return
			}
		}
	}()
	if err := srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, net.ErrClosed) {
		return err
	}
	return nil
}

func openGUIURL(url, cache string) error {
	candidates := []string{"chromium", "chromium-browser", "google-chrome", "google-chrome-stable", "brave-browser", "brave", "vivaldi", "vivaldi-stable"}
	for _, name := range candidates {
		if p, err := exec.LookPath(name); err == nil {
			cmd := exec.Command(p, "--app="+url, "--new-window", "--class=CitizenLauncher")
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
