package main

import (
	"bufio"
	"os"
	"os/exec"
	"strings"
)

type PlatformStatus struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Version        string `json:"version"`
	IDLike         string `json:"id_like,omitempty"`
	PackageManager string `json:"package_manager"`
	Desktop        string `json:"desktop,omitempty"`
	Session        string `json:"session,omitempty"`
	SystemdUser    bool   `json:"systemd_user"`
	Omarchy        bool   `json:"omarchy"`
}

func detectPlatform() PlatformStatus {
	p := PlatformStatus{ID: "linux", Name: "Linux"}
	if f, err := os.Open("/etc/os-release"); err == nil {
		defer f.Close()
		vals := map[string]string{}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			k, v, ok := strings.Cut(line, "=")
			if ok {
				vals[k] = strings.Trim(strings.TrimSpace(v), `"'`)
			}
		}
		if vals["ID"] != "" {
			p.ID = vals["ID"]
		}
		if vals["PRETTY_NAME"] != "" {
			p.Name = vals["PRETTY_NAME"]
		} else if vals["NAME"] != "" {
			p.Name = vals["NAME"]
		}
		p.Version = vals["VERSION_ID"]
		p.IDLike = vals["ID_LIKE"]
	}
	switch {
	case commandExists("apt-get"):
		p.PackageManager = "apt"
	case commandExists("dnf"):
		p.PackageManager = "dnf"
	case commandExists("pacman"):
		p.PackageManager = "pacman"
	case commandExists("zypper"):
		p.PackageManager = "zypper"
	case commandExists("apk"):
		p.PackageManager = "apk"
	default:
		p.PackageManager = "unknown"
	}
	p.Desktop = envOr("XDG_CURRENT_DESKTOP", envOr("DESKTOP_SESSION", ""))
	p.Session = envOr("XDG_SESSION_TYPE", "")
	if commandExists("systemctl") {
		cmd := exec.Command("systemctl", "--user", "show-environment")
		p.SystemdUser = cmd.Run() == nil
	}
	p.Omarchy = commandExists("omarchy")
	return p
}
