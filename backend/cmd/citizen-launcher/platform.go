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
	Variant        string `json:"variant,omitempty"`
	IDLike         string `json:"id_like,omitempty"`
	Family         string `json:"family"`
	PackageManager string `json:"package_manager"`
	Desktop        string `json:"desktop,omitempty"`
	Session        string `json:"session,omitempty"`
	SystemdUser    bool   `json:"systemd_user"`
	Immutable      bool   `json:"immutable"`
	Omarchy        bool   `json:"omarchy"`
}

func detectPlatform() PlatformStatus {
	vals := readOSRelease("/etc/os-release")
	p := platformFromOSRelease(vals)

	// Prefer the package manager that belongs to the detected distro family. This
	// avoids mis-detection on systems that happen to have foreign packaging tools
	// installed for development or compatibility purposes.
	switch p.Family {
	case "debian":
		if commandExists("apt-get") {
			p.PackageManager = "apt"
		}
	case "fedora":
		if commandExists("rpm-ostree") && p.Immutable {
			p.PackageManager = "rpm-ostree"
		} else if commandExists("dnf5") {
			p.PackageManager = "dnf5"
		} else if commandExists("dnf") {
			p.PackageManager = "dnf"
		}
	case "arch":
		if commandExists("pacman") {
			p.PackageManager = "pacman"
		}
	case "suse":
		if commandExists("transactional-update") && p.Immutable {
			p.PackageManager = "transactional-update"
		} else if commandExists("zypper") {
			p.PackageManager = "zypper"
		}
	}
	if p.PackageManager == "" {
		switch {
		case commandExists("apt-get"):
			p.PackageManager = "apt"
		case commandExists("dnf5"):
			p.PackageManager = "dnf5"
		case commandExists("dnf"):
			p.PackageManager = "dnf"
		case commandExists("pacman"):
			p.PackageManager = "pacman"
		case commandExists("zypper"):
			p.PackageManager = "zypper"
		case commandExists("xbps-install"):
			p.PackageManager = "xbps"
		case commandExists("emerge"):
			p.PackageManager = "portage"
		case commandExists("apk"):
			p.PackageManager = "apk"
		default:
			p.PackageManager = "unknown"
		}
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

func readOSRelease(path string) map[string]string {
	vals := map[string]string{}
	f, err := os.Open(path)
	if err != nil {
		return vals
	}
	defer f.Close()
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
	return vals
}

func platformFromOSRelease(vals map[string]string) PlatformStatus {
	p := PlatformStatus{ID: "linux", Name: "Linux", Family: "other"}
	if vals["ID"] != "" {
		p.ID = strings.ToLower(vals["ID"])
	}
	if vals["PRETTY_NAME"] != "" {
		p.Name = vals["PRETTY_NAME"]
	} else if vals["NAME"] != "" {
		p.Name = vals["NAME"]
	}
	p.Version = vals["VERSION_ID"]
	p.Variant = vals["VARIANT_ID"]
	p.IDLike = strings.ToLower(vals["ID_LIKE"])
	p.Family = distroFamily(p.ID, p.IDLike)
	p.Immutable = immutableDistro(p.ID, p.Variant) || fileExists("/run/ostree-booted") || fileExists("/run/transactional-update")
	return p
}

func distroFamily(id, idLike string) string {
	id = strings.ToLower(id)
	like := " " + strings.ToLower(idLike) + " "
	containsLike := func(v string) bool { return strings.Contains(like, " "+v+" ") }
	switch id {
	case "debian", "ubuntu", "linuxmint", "pop", "elementary", "zorin", "kali", "neon", "tuxedo":
		return "debian"
	case "fedora", "nobara", "rhel", "centos", "rocky", "almalinux", "ultramarine":
		return "fedora"
	case "arch", "manjaro", "endeavouros", "cachyos", "garuda", "steamos":
		return "arch"
	case "opensuse", "opensuse-tumbleweed", "opensuse-leap", "opensuse-slowroll", "sles", "sled":
		return "suse"
	}
	switch {
	case containsLike("debian") || containsLike("ubuntu"):
		return "debian"
	case containsLike("fedora") || containsLike("rhel") || containsLike("centos"):
		return "fedora"
	case containsLike("arch"):
		return "arch"
	case containsLike("suse") || containsLike("opensuse"):
		return "suse"
	default:
		return "other"
	}
}

func immutableDistro(id, variant string) bool {
	id = strings.ToLower(id)
	variant = strings.ToLower(variant)
	if id == "steamos" || strings.Contains(id, "microos") || strings.Contains(id, "aeon") || strings.Contains(id, "kalpa") {
		return true
	}
	switch variant {
	case "silverblue", "kinoite", "sericea", "onyx", "atomic", "coreos", "steamdeck":
		return true
	}
	return false
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
