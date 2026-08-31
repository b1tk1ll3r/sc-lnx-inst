package main

import (
	"bufio"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

type SystemReadiness struct {
	State          string `json:"state"`
	MapCount       int64  `json:"vm_max_map_count"`
	MapCountOK     bool   `json:"vm_max_map_count_ok"`
	NoFileSoft     uint64 `json:"nofile_soft"`
	NoFileHard     uint64 `json:"nofile_hard"`
	NoFileHardOK   bool   `json:"nofile_hard_ok"`
	Filesystem     string `json:"filesystem,omitempty"`
	MountOptions   string `json:"mount_options,omitempty"`
	FilesystemOK   bool   `json:"filesystem_ok"`
	StorageFreeGiB int    `json:"storage_free_gib"`
	Reason         string `json:"reason,omitempty"`
}

func (a *App) systemReadiness(prefix string) SystemReadiness {
	r := SystemReadiness{State: "ready", FilesystemOK: true}
	if b, err := os.ReadFile("/proc/sys/vm/max_map_count"); err == nil {
		r.MapCount, _ = strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
	}
	r.MapCountOK = r.MapCount >= requiredMapCount

	var lim syscall.Rlimit
	if syscall.Getrlimit(syscall.RLIMIT_NOFILE, &lim) == nil {
		r.NoFileSoft, r.NoFileHard = lim.Cur, lim.Max
	}
	r.NoFileHardOK = r.NoFileHard >= requiredNoFile

	existing := nearestExistingPath(prefix)
	if existing != "" {
		if fs, opts, err := mountInfo(existing); err == nil {
			r.Filesystem, r.MountOptions = fs, opts
			low := strings.ToLower(fs)
			if low == "ntfs" || low == "ntfs3" || low == "fuseblk" || low == "exfat" || optionPresent(opts, "noexec") {
				r.FilesystemOK = false
				if optionPresent(opts, "noexec") {
					r.Reason = "Der Installationsdatenträger ist mit noexec eingehängt. Wine-Runner können dort nicht zuverlässig ausgeführt werden."
				} else {
					r.Reason = "Star Citizen sollte nicht auf NTFS/exFAT installiert werden. Bitte einen Linux-Datenträger verwenden."
				}
			}
		}
	}
	var s syscallStatfs
	if statfs(prefix, &s) != nil {
		_ = statfs(existing, &s)
	}
	if s.Bsize > 0 {
		r.StorageFreeGiB = int((s.Bavail * uint64(s.Bsize)) / (1024 * 1024 * 1024))
	}

	switch {
	case !r.FilesystemOK:
		r.State = "blocked"
	case !r.MapCountOK || !r.NoFileHardOK:
		r.State = "prepare"
	}
	return r
}

func nearestExistingPath(path string) string {
	p := path
	for p != "" && p != string(filepath.Separator) {
		if _, err := os.Stat(p); err == nil {
			return p
		}
		next := filepath.Dir(p)
		if next == p {
			break
		}
		p = next
	}
	if _, err := os.Stat(string(filepath.Separator)); err == nil {
		return string(filepath.Separator)
	}
	return ""
}

func mountInfo(path string) (string, string, error) {
	if commandExists("findmnt") {
		out, err := exec.Command("findmnt", "-T", path, "-n", "-o", "FSTYPE,OPTIONS").CombinedOutput()
		if err == nil {
			fields := strings.Fields(strings.TrimSpace(string(out)))
			if len(fields) >= 2 {
				return fields[0], fields[1], nil
			}
		}
	}
	// /proc/mounts fallback: choose the longest matching mountpoint.
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return "", "", err
	}
	defer f.Close()
	bestMount, bestFS, bestOpts := "", "", ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 4 {
			continue
		}
		mount := strings.ReplaceAll(fields[1], `\040`, " ")
		if path == mount || strings.HasPrefix(path, strings.TrimRight(mount, "/")+"/") {
			if len(mount) > len(bestMount) {
				bestMount, bestFS, bestOpts = mount, fields[2], fields[3]
			}
		}
	}
	if bestMount == "" {
		return "", "", errors.New("mount point not found")
	}
	return bestFS, bestOpts, nil
}

func optionPresent(opts, want string) bool {
	for _, o := range strings.Split(opts, ",") {
		if o == want {
			return true
		}
	}
	return false
}
