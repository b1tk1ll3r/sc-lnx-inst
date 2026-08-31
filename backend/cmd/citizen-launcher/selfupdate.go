package main

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type SelfUpdateStatus struct {
	Current       string `json:"current"`
	Installed     string `json:"installed,omitempty"`
	Latest        string `json:"latest,omitempty"`
	State         string `json:"state"`
	Mode          string `json:"mode"`
	Repository    string `json:"repository"`
	Asset         string `json:"asset,omitempty"`
	Digest        string `json:"digest,omitempty"`
	RestartNeeded bool   `json:"restart_needed"`
	CheckedAt     string `json:"checked_at,omitempty"`
	Error         string `json:"error,omitempty"`
}

func packageAutoUpdateActive() bool {
	if !commandExists("systemctl") {
		return false
	}
	return exec.Command("systemctl", "is-enabled", "citizen-launcher-self-update.timer").Run() == nil
}

func effectiveReleaseRepo() string {
	if v := strings.TrimSpace(os.Getenv("CITIZEN_LAUNCHER_RELEASE_REPO")); v != "" {
		return v
	}
	if data, err := os.ReadFile("/etc/citizen-launcher/release-repo"); err == nil {
		if v := strings.TrimSpace(string(data)); v != "" {
			return v
		}
	}
	return releaseRepo
}

func (a *App) selfUpdateStatus(fetch bool) (SelfUpdateStatus, error) {
	installed := installedPackageVersion()
	st := SelfUpdateStatus{
		Current:    appVersion,
		Installed:  installed,
		State:      "not-checked",
		Mode:       a.installMode(),
		Repository: effectiveReleaseRepo(),
	}
	if installed != "" && compareVersions(installed, appVersion) > 0 {
		st.RestartNeeded = true
	}
	if !fetch {
		if st.RestartNeeded {
			st.State = "restart-required"
		}
		return st, nil
	}

	release, err := githubLatest(effectiveReleaseRepo())
	st.CheckedAt = time.Now().Format(time.RFC3339)
	if err != nil {
		st.State = "check-failed"
		st.Error = err.Error()
		return st, err
	}
	latest := strings.TrimPrefix(strings.TrimSpace(release.TagName), "v")
	if latest == "" {
		err := errors.New("latest release has no usable version tag")
		st.State, st.Error = "check-failed", err.Error()
		return st, err
	}
	st.Latest = latest

	asset, err := a.releaseAssetForMode(release, latest, st.Mode)
	if err != nil {
		st.State = "asset-missing"
		st.Error = err.Error()
		return st, err
	}
	st.Asset = asset.Name
	st.Digest = asset.Digest

	current := appVersion
	if installed != "" && compareVersions(installed, current) > 0 {
		current = installed
	}
	cmp := compareVersions(latest, current)
	switch {
	case cmp > 0:
		st.State = "available"
	case st.RestartNeeded:
		st.State = "restart-required"
	default:
		st.State = "current"
	}
	return st, nil
}

func (a *App) installMode() string {
	if installedPackageVersion() != "" && commandExists("dpkg-deb") {
		return "deb"
	}
	return "user"
}

func installedPackageVersion() string {
	if !commandExists("dpkg-query") {
		return ""
	}
	cmd := exec.Command("dpkg-query", "-W", "-f=${Status}\n${Version}", "citizen-launcher")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 || !strings.Contains(lines[0], "install ok installed") {
		return ""
	}
	return strings.TrimSpace(lines[len(lines)-1])
}

func (a *App) releaseAssetForMode(release githubRelease, version, mode string) (githubAsset, error) {
	if mode == "deb" {
		want := "citizen-launcher_" + version + "_amd64.deb"
		for _, asset := range release.Assets {
			if asset.Name == want {
				return asset, nil
			}
		}
		// Accept Debian revisions such as 0.9.2-1 while keeping the package name strict.
		re := regexp.MustCompile(`^citizen-launcher_` + regexp.QuoteMeta(version) + `(?:-[0-9]+)?_amd64\.deb$`)
		for _, asset := range release.Assets {
			if re.MatchString(asset.Name) {
				return asset, nil
			}
		}
		return githubAsset{}, fmt.Errorf("release %s has no amd64 Debian package", release.TagName)
	}
	want := "citizen-launcher-" + version + "-linux-amd64.tar.gz"
	for _, asset := range release.Assets {
		if asset.Name == want {
			return asset, nil
		}
	}
	return githubAsset{}, fmt.Errorf("release %s has no generic amd64 tarball", release.TagName)
}

func (a *App) applySelfUpdate(system, quiet bool) error {
	st, err := a.selfUpdateStatus(true)
	if err != nil {
		return err
	}
	if st.State == "current" || st.State == "restart-required" {
		if !quiet {
			fmt.Printf("Citizen Launcher %s ist bereits installiert.\n", st.Installed)
		}
		return nil
	}
	if st.State != "available" {
		return fmt.Errorf("self-update is not applicable in state %q", st.State)
	}

	release, err := githubLatest(effectiveReleaseRepo())
	if err != nil {
		return err
	}
	asset, err := a.releaseAssetForMode(release, st.Latest, st.Mode)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(strings.ToLower(asset.Digest), "sha256:") {
		return errors.New("release asset has no GitHub SHA-256 digest; refusing automatic update")
	}

	if st.Mode == "deb" {
		if !system || os.Geteuid() != 0 {
			if commandExists("pkexec") {
				self := a.selfPath
				if self == "" {
					self = "/usr/bin/citizen-launcher"
				}
				cmd := exec.Command("pkexec", self, "self-update", "apply", "--system")
				cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
				return cmd.Run()
			}
			return errors.New("Debian package update needs root privileges and pkexec is unavailable")
		}
		return a.applyDebUpdate(asset, st.Latest, quiet)
	}
	return a.applyUserUpdate(asset, st.Latest, quiet)
}

func (a *App) applyDebUpdate(asset githubAsset, version string, quiet bool) error {
	cache := "/var/cache/citizen-launcher"
	if err := os.MkdirAll(cache, 0o755); err != nil {
		return err
	}
	path := filepath.Join(cache, asset.Name)
	tmp := path + ".new"
	_ = os.Remove(tmp)
	if err := download(asset.BrowserDownloadURL, tmp); err != nil {
		return err
	}
	if err := verifyReleaseDigest(tmp, asset.Digest); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := verifyDebPackage(tmp, version); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}

	cmd := exec.Command("apt-get",
		"-o", "DPkg::Lock::Timeout=120",
		"-o", "Dpkg::Options::=--force-confold",
		"install", "-y", "--no-install-recommends", path,
	)
	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	if quiet {
		cmd.Stdout = a.logWriter()
		cmd.Stderr = a.logWriter()
	} else {
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("APT package update failed: %w", err)
	}
	installed := installedPackageVersion()
	if compareVersions(installed, version) < 0 {
		return fmt.Errorf("package manager returned success but installed version is %q, expected at least %q", installed, version)
	}
	if !quiet {
		fmt.Printf("Citizen Launcher wurde auf %s aktualisiert. Ein laufendes GUI verwendet die neue Version nach dem nächsten Neustart.\n", installed)
	}
	return nil
}

func verifyDebPackage(path, version string) error {
	if !commandExists("dpkg-deb") {
		return errors.New("dpkg-deb is unavailable")
	}
	cmd := exec.Command("dpkg-deb", "-f", path, "Package", "Version", "Architecture")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("cannot inspect Debian package: %s", formatCommandFailure(err, out))
	}
	values := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		k, v, ok := strings.Cut(line, ":")
		if ok {
			values[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	pkg, pkgVersion, arch := values["Package"], values["Version"], values["Architecture"]
	if pkg == "" || pkgVersion == "" || arch == "" {
		return fmt.Errorf("unexpected Debian package metadata: %q", strings.TrimSpace(string(out)))
	}
	if pkg != "citizen-launcher" {
		return fmt.Errorf("refusing package %q", pkg)
	}
	if compareVersions(pkgVersion, version) < 0 {
		return fmt.Errorf("package version %q is older than release %q", pkgVersion, version)
	}
	if arch != "amd64" {
		return fmt.Errorf("refusing architecture %q", arch)
	}
	return nil
}

func (a *App) applyUserUpdate(asset githubAsset, version string, quiet bool) error {
	if a.selfPath == "" {
		return errors.New("cannot locate running Citizen Launcher executable")
	}
	if !strings.HasPrefix(strings.ToLower(asset.Digest), "sha256:") {
		return errors.New("release asset has no GitHub SHA-256 digest; refusing automatic update")
	}
	if err := os.MkdirAll(a.cacheDir, 0o755); err != nil {
		return err
	}
	archive := filepath.Join(a.cacheDir, asset.Name)
	if err := download(asset.BrowserDownloadURL, archive); err != nil {
		return err
	}
	if err := verifyReleaseDigest(archive, asset.Digest); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(a.cacheDir, "self-update-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if err := extractSingleBinaryTarGz(archive, stage); err != nil {
		return err
	}
	candidate := filepath.Join(stage, "citizen-launcher")
	out, err := exec.Command(candidate, "--version").CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != version {
		return fmt.Errorf("downloaded launcher version check failed: %s", formatCommandFailure(err, out))
	}
	tmp := a.selfPath + ".new"
	if err := copyFile(candidate, tmp, 0o755); err != nil {
		return err
	}
	if err := os.Rename(tmp, a.selfPath); err != nil {
		return err
	}
	if !quiet {
		fmt.Printf("Citizen Launcher wurde auf %s aktualisiert.\n", version)
	}
	return nil
}

func extractSingleBinaryTarGz(path, dir string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if filepath.Base(h.Name) != "citizen-launcher" || h.Typeflag != tar.TypeReg {
			continue
		}
		target := filepath.Join(dir, "citizen-launcher")
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, tr)
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	}
	return errors.New("tarball does not contain citizen-launcher")
}

func compareVersions(a, b string) int {
	parse := func(v string) []int {
		v = strings.TrimPrefix(strings.TrimSpace(v), "v")
		v = strings.SplitN(v, "-", 2)[0]
		parts := strings.Split(v, ".")
		out := make([]int, 3)
		for i := 0; i < len(out) && i < len(parts); i++ {
			n, _ := strconv.Atoi(parts[i])
			out[i] = n
		}
		return out
	}
	aa, bb := parse(a), parse(b)
	for i := range aa {
		if aa[i] < bb[i] {
			return -1
		}
		if aa[i] > bb[i] {
			return 1
		}
	}
	return 0
}

func printSelfUpdateKV(s SelfUpdateStatus) {
	fmt.Printf("current=%s\n", s.Current)
	fmt.Printf("installed=%s\n", s.Installed)
	fmt.Printf("latest=%s\n", s.Latest)
	fmt.Printf("state=%s\n", s.State)
	fmt.Printf("mode=%s\n", s.Mode)
	fmt.Printf("repository=%s\n", s.Repository)
	fmt.Printf("asset=%s\n", s.Asset)
	fmt.Printf("restart_needed=%t\n", s.RestartNeeded)
	if s.Error != "" {
		fmt.Printf("error=%s\n", s.Error)
	}
}
