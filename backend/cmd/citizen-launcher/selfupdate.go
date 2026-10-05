package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"citizen-launcher/backend/internal/signing"
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

type installedPackage struct {
	Mode    string
	Version string
}

func packageAutoUpdateActive() bool {
	if !commandExists("systemctl") {
		return false
	}
	return exec.Command("systemctl", "is-enabled", "citizen-launcher-self-update.timer").Run() == nil
}

var stableVersionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

var releaseRepoPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*/[A-Za-z0-9][A-Za-z0-9_.-]*$`)

type launcherReleaseSource struct {
	Kind       string
	Display    string
	GitHubRepo string
	APIRepoURL string
}

func parseLauncherReleaseSource(v string) (launcherReleaseSource, bool) {
	v = strings.TrimSpace(v)
	if releaseRepoPattern.MatchString(v) {
		return launcherReleaseSource{Kind: "github", Display: v, GitHubRepo: v}, true
	}
	if strings.HasPrefix(v, "github:") {
		repo := strings.TrimSpace(strings.TrimPrefix(v, "github:"))
		if releaseRepoPattern.MatchString(repo) {
			return launcherReleaseSource{Kind: "github", Display: "github:" + repo, GitHubRepo: repo}, true
		}
		return launcherReleaseSource{}, false
	}
	if strings.HasPrefix(v, "gitea:") {
		raw := strings.TrimSpace(strings.TrimPrefix(v, "gitea:"))
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return launcherReleaseSource{}, false
		}
		clean := strings.TrimRight(u.String(), "/")
		// Store the exact Gitea API repository endpoint so installations also work
		// when the Gitea instance itself is hosted below a URL sub-path.
		if !strings.Contains(u.Path, "/api/v1/repos/") {
			return launcherReleaseSource{}, false
		}
		tail := strings.Trim(strings.SplitN(u.Path, "/api/v1/repos/", 2)[1], "/")
		if !releaseRepoPattern.MatchString(tail) {
			return launcherReleaseSource{}, false
		}
		return launcherReleaseSource{Kind: "gitea", Display: "gitea:" + clean, APIRepoURL: clean}, true
	}
	return launcherReleaseSource{}, false
}

func validReleaseRepo(v string) bool {
	_, ok := parseLauncherReleaseSource(v)
	return ok
}

func effectiveReleaseRepo() string {
	// Never let a user-controlled environment variable redirect the privileged
	// system updater. Root trusts only the root-owned config file or compiled repo.
	if os.Geteuid() != 0 {
		if v := strings.TrimSpace(os.Getenv("CITIZEN_LAUNCHER_RELEASE_REPO")); validReleaseRepo(v) {
			return v
		}
	}
	if data, err := os.ReadFile("/etc/citizen-launcher/release-repo"); err == nil {
		trusted := true
		if os.Geteuid() == 0 {
			if fi, statErr := os.Stat("/etc/citizen-launcher/release-repo"); statErr != nil {
				trusted = false
			} else if st, ok := fi.Sys().(*syscall.Stat_t); !ok || st.Uid != 0 || fi.Mode().Perm()&0o022 != 0 {
				trusted = false
			}
		}
		if trusted {
			if v := strings.TrimSpace(string(data)); validReleaseRepo(v) {
				return v
			}
		}
	}
	if validReleaseRepo(releaseRepo) {
		return releaseRepo
	}
	return "github:b1tk1ll3r/sc-lnx-inst"
}

func launcherLatestRelease(spec string) (githubRelease, error) {
	source, ok := parseLauncherReleaseSource(spec)
	if !ok {
		return githubRelease{}, fmt.Errorf("invalid launcher release source %q", spec)
	}
	var rel githubRelease
	var err error
	switch source.Kind {
	case "github":
		rel, err = githubLatest(source.GitHubRepo)
	case "gitea":
		req, reqErr := http.NewRequest("GET", source.APIRepoURL+"/releases/latest", nil)
		if reqErr != nil {
			return githubRelease{}, reqErr
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "citizen-launcher/"+appVersion)
		client := httpsClient(30 * time.Second)
		resp, doErr := client.Do(req)
		if doErr != nil {
			return githubRelease{}, doErr
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return githubRelease{}, fmt.Errorf("Gitea release API returned HTTP %d", resp.StatusCode)
		}
		err = json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&rel)
	default:
		err = fmt.Errorf("unsupported release source %q", source.Kind)
	}
	if err != nil {
		return githubRelease{}, err
	}
	// Every launcher release must carry SHA256SUMS.txt plus an Ed25519
	// signature by the project release key. Asset digests are taken only from
	// the signed manifest; GitHub's own API digest is merely cross-checked.
	if err := applySignedChecksums(&rel); err != nil {
		return githubRelease{}, err
	}
	return rel, nil
}

const (
	checksumAssetName  = "SHA256SUMS.txt"
	signatureAssetName = "SHA256SUMS.txt.sig"
)

func applySignedChecksums(rel *githubRelease) error {
	if rel == nil {
		return errors.New("nil release")
	}
	var sumsURL, sigURL string
	for _, asset := range rel.Assets {
		switch asset.Name {
		case checksumAssetName:
			sumsURL = asset.BrowserDownloadURL
		case signatureAssetName:
			sigURL = asset.BrowserDownloadURL
		}
	}
	if sumsURL == "" || sigURL == "" {
		return fmt.Errorf("release %s is not signed (%s/%s missing); refusing update", rel.TagName, checksumAssetName, signatureAssetName)
	}
	sums, err := fetchSmall(sumsURL, 2<<20)
	if err != nil {
		return fmt.Errorf("%s: %w", checksumAssetName, err)
	}
	sig, err := fetchSmall(sigURL, 4<<10)
	if err != nil {
		return fmt.Errorf("%s: %w", signatureAssetName, err)
	}
	return applyVerifiedChecksums(rel, sums, sig, signing.PublicKeys)
}

// applyVerifiedChecksums verifies the manifest signature and replaces every
// asset digest with the signed value. Assets missing from the manifest lose
// their digest and therefore can never be installed (fail closed).
func applyVerifiedChecksums(rel *githubRelease, sums, sig []byte, keys []string) error {
	if err := signing.VerifyWith(keys, sums, sig); err != nil {
		return fmt.Errorf("release %s: %w", rel.TagName, err)
	}
	checksums := parseSHA256SUMS(string(sums))
	for i := range rel.Assets {
		a := &rel.Assets[i]
		signed := checksums[a.Name]
		if signed == "" {
			a.Digest = ""
			continue
		}
		if api := strings.ToLower(strings.TrimSpace(a.Digest)); strings.HasPrefix(api, "sha256:") && api != "sha256:"+signed {
			return fmt.Errorf("release asset %s: API digest differs from signed checksum", a.Name)
		}
		a.Digest = "sha256:" + signed
	}
	return nil
}

func fetchSmall(rawURL string, limit int64) ([]byte, error) {
	if err := requireHTTPS(rawURL); err != nil {
		return nil, err
	}
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "citizen-launcher/"+appVersion)
	resp, err := httpsClient(30 * time.Second).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("download returned HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("file exceeds %d bytes", limit)
	}
	return data, nil
}

func parseSHA256SUMS(body string) map[string]string {
	out := map[string]string{}
	hex64 := regexp.MustCompile(`^[0-9a-fA-F]{64}$`)
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || !hex64.MatchString(fields[0]) {
			continue
		}
		name := strings.TrimPrefix(fields[len(fields)-1], "*")
		name = strings.TrimPrefix(name, "./")
		if filepath.Base(name) != name || name == "" {
			continue
		}
		out[name] = strings.ToLower(fields[0])
	}
	return out
}

func (a *App) selfUpdateStatus(fetch bool) (SelfUpdateStatus, error) {
	pkg := installedPackageInfo()
	mode := a.installMode()
	installed := pkg.Version
	if mode == "user" {
		installed = executableVersion(a.selfPath)
	}
	st := SelfUpdateStatus{
		Current:    appVersion,
		Installed:  installed,
		State:      "not-checked",
		Mode:       mode,
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

	release, err := launcherLatestRelease(effectiveReleaseRepo())
	st.CheckedAt = time.Now().Format(time.RFC3339)
	if err != nil {
		st.State = "check-failed"
		st.Error = err.Error()
		return st, err
	}
	latest := strings.TrimPrefix(strings.TrimSpace(release.TagName), "v")
	// Only plain X.Y.Z tags are update targets: the version is embedded in
	// asset regexes and cache paths, and compareVersions ignores suffixes.
	if !stableVersionPattern.MatchString(latest) {
		err := fmt.Errorf("latest release tag %q is not a stable X.Y.Z version", release.TagName)
		st.State, st.Error = "check-failed", err.Error()
		return st, err
	}
	st.Latest = latest

	current := appVersion
	if installed != "" && compareVersions(installed, current) > 0 {
		current = installed
	}
	cmp := compareVersions(latest, current)

	// Immutable system images must be updated by their host image/package layer.
	// A ~/.local installation on the same machine still uses mode=user and can
	// update itself atomically without touching the immutable base system.
	if mode == "system-managed" {
		if cmp > 0 {
			st.State = "external-update"
		} else if st.RestartNeeded {
			st.State = "restart-required"
		} else {
			st.State = "current"
		}
		return st, nil
	}

	asset, err := a.releaseAssetForMode(release, latest, mode)
	if err != nil {
		st.State = "asset-missing"
		st.Error = err.Error()
		return st, err
	}
	st.Asset = asset.Name
	st.Digest = asset.Digest

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
	pkg := installedPackageInfo()
	if pkg.Version == "" || !isSystemExecutable(a.selfPath) {
		return "user"
	}
	p := detectPlatform()
	if p.Immutable {
		return "system-managed"
	}
	return pkg.Mode
}

func isSystemExecutable(path string) bool {
	if path == "" {
		return false
	}
	clean, _ := filepath.EvalSymlinks(path)
	if clean == "" {
		clean = filepath.Clean(path)
	}
	return clean == "/usr/bin/citizen-launcher" || clean == "/bin/citizen-launcher"
}

func executableVersion(path string) string {
	if path == "" {
		return ""
	}
	out, err := exec.Command(path, "--version").CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func installedPackageVersion() string { return installedPackageInfo().Version }

func installedPackageInfo() installedPackage {
	// Prefer the package database native to /etc/os-release. This prevents a
	// developer-installed foreign package tool from taking ownership of updates.
	vals := readOSRelease("/etc/os-release")
	family := distroFamily(strings.ToLower(vals["ID"]), strings.ToLower(vals["ID_LIKE"]))
	order := []string{"deb", "rpm", "arch"}
	switch family {
	case "debian":
		order = []string{"deb", "rpm", "arch"}
	case "fedora", "suse":
		order = []string{"rpm", "deb", "arch"}
	case "arch":
		order = []string{"arch", "rpm", "deb"}
	}
	for _, mode := range order {
		if pkg := queryInstalledPackage(mode); pkg.Version != "" {
			return pkg
		}
	}
	return installedPackage{}
}

func queryInstalledPackage(mode string) installedPackage {
	switch mode {
	case "deb":
		if !commandExists("dpkg-query") {
			return installedPackage{}
		}
		cmd := exec.Command("dpkg-query", "-W", "-f=${Status}\n${Version}", "citizen-launcher")
		if out, err := cmd.CombinedOutput(); err == nil {
			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			if len(lines) >= 2 && strings.Contains(lines[0], "install ok installed") {
				return installedPackage{Mode: "deb", Version: strings.TrimSpace(lines[len(lines)-1])}
			}
		}
	case "rpm":
		if !commandExists("rpm") {
			return installedPackage{}
		}
		cmd := exec.Command("rpm", "-q", "--qf", "%{VERSION}\n", "citizen-launcher")
		if out, err := cmd.CombinedOutput(); err == nil {
			if v := strings.TrimSpace(string(out)); v != "" {
				return installedPackage{Mode: "rpm", Version: v}
			}
		}
	case "arch":
		if !commandExists("pacman") {
			return installedPackage{}
		}
		cmd := exec.Command("pacman", "-Q", "citizen-launcher")
		if out, err := cmd.CombinedOutput(); err == nil {
			fields := strings.Fields(strings.TrimSpace(string(out)))
			if len(fields) >= 2 && fields[0] == "citizen-launcher" {
				return installedPackage{Mode: "arch", Version: fields[1]}
			}
		}
	}
	return installedPackage{}
}

func (a *App) releaseAssetForMode(release githubRelease, version, mode string) (githubAsset, error) {
	patterns := map[string][]*regexp.Regexp{
		"deb": {
			regexp.MustCompile(`^citizen-launcher_` + regexp.QuoteMeta(version) + `_amd64\.deb$`),
			regexp.MustCompile(`^citizen-launcher_` + regexp.QuoteMeta(version) + `-[0-9]+_amd64\.deb$`),
		},
		"rpm": {
			regexp.MustCompile(`^citizen-launcher-` + regexp.QuoteMeta(version) + `-[0-9]+(?:\.[A-Za-z0-9_.-]+)?\.x86_64\.rpm$`),
		},
		"arch": {
			regexp.MustCompile(`^citizen-launcher-` + regexp.QuoteMeta(version) + `-[0-9]+-x86_64\.pkg\.tar\.(?:zst|xz|gz)$`),
		},
		"user": {
			regexp.MustCompile(`^citizen-launcher-` + regexp.QuoteMeta(version) + `-linux-amd64\.tar\.gz$`),
		},
	}
	res, ok := patterns[mode]
	if !ok {
		return githubAsset{}, fmt.Errorf("release updates are not supported for install mode %q", mode)
	}
	for _, re := range res {
		for _, asset := range release.Assets {
			// The name becomes a path below the (root-owned) cache directory.
			if filepath.Base(asset.Name) != asset.Name || strings.ContainsAny(asset.Name, "/\\") {
				continue
			}
			if re.MatchString(asset.Name) {
				return asset, nil
			}
		}
	}
	return githubAsset{}, fmt.Errorf("release %s has no %s package for x86-64", release.TagName, mode)
}

func (a *App) applySelfUpdate(system, quiet bool) error {
	st, err := a.selfUpdateStatus(true)
	if err != nil {
		return err
	}
	if st.State == "current" || st.State == "restart-required" {
		if !quiet {
			shown := st.Installed
			if shown == "" {
				shown = appVersion
			}
			fmt.Printf("Citizen Launcher %s ist bereits installiert.\n", shown)
		}
		return nil
	}
	if st.State == "external-update" || st.Mode == "system-managed" {
		return errors.New("Dieses immutable Linux-System verwaltet /usr über sein System-Image. Bitte Citizen Launcher über rpm-ostree/transactional-update bzw. die Distribution aktualisieren; eine ~/.local-Installation kann sich weiterhin selbst aktualisieren")
	}
	if st.State != "available" {
		return fmt.Errorf("self-update is not applicable in state %q", st.State)
	}

	release, err := launcherLatestRelease(effectiveReleaseRepo())
	if err != nil {
		return err
	}
	if latestNow := strings.TrimPrefix(strings.TrimSpace(release.TagName), "v"); latestNow != st.Latest {
		return fmt.Errorf("Release änderte sich während der Update-Prüfung (%s → %s); Update wird beim nächsten Lauf erneut geprüft", st.Latest, latestNow)
	}
	asset, err := a.releaseAssetForMode(release, st.Latest, st.Mode)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(strings.ToLower(asset.Digest), "sha256:") {
		return errors.New("release asset has no SHA-256 digest/checksum; refusing automatic update")
	}

	if st.Mode == "deb" || st.Mode == "rpm" || st.Mode == "arch" {
		if !system || os.Geteuid() != 0 {
			if commandExists("pkexec") {
				self := a.stableExecutable()
				if self == "" {
					self = "/usr/bin/citizen-launcher"
				}
				cmd := exec.Command("pkexec", self, "self-update", "apply", "--system")
				cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
				return cmd.Run()
			}
			return fmt.Errorf("%s package update needs root privileges and pkexec is unavailable", st.Mode)
		}
		return a.applyNativePackageUpdate(st.Mode, asset, st.Latest, quiet)
	}
	return a.applyUserUpdate(asset, st.Latest, quiet)
}

func (a *App) applyNativePackageUpdate(mode string, asset githubAsset, version string, quiet bool) error {
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
	switch mode {
	case "deb":
		if err := verifyDebPackage(tmp, version); err != nil {
			_ = os.Remove(tmp)
			return err
		}
	case "rpm":
		if err := verifyRPMPackage(tmp, version); err != nil {
			_ = os.Remove(tmp)
			return err
		}
	case "arch":
		if err := verifyArchPackage(tmp, version); err != nil {
			_ = os.Remove(tmp)
			return err
		}
	default:
		return fmt.Errorf("unsupported native package mode %q", mode)
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}

	var cmd *exec.Cmd
	switch mode {
	case "deb":
		cmd = exec.Command("apt-get", "-o", "DPkg::Lock::Timeout=120", "-o", "Dpkg::Options::=--force-confold", "install", "-y", "--no-install-recommends", path)
		cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	case "rpm":
		// The RPM has already been authenticated through the configured release
		// source's SHA-256 metadata and its package metadata is checked above. rpm still performs
		// dependency and scriptlet validation locally.
		cmd = exec.Command("rpm", "-Uvh", "--replacepkgs", path)
	case "arch":
		cmd = exec.Command("pacman", "-U", "--noconfirm", "--needed", path)
	}
	if quiet {
		if os.Geteuid() == 0 {
			cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		} else {
			cmd.Stdout, cmd.Stderr = a.logWriter(), a.logWriter()
		}
	} else {
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s package update failed: %w", mode, err)
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
	return verifyPackageIdentity(values["Package"], values["Version"], values["Architecture"], version, "amd64")
}

func verifyRPMPackage(path, version string) error {
	if !commandExists("rpm") {
		return errors.New("rpm is unavailable")
	}
	cmd := exec.Command("rpm", "-qp", "--qf", "%{NAME}\n%{VERSION}\n%{ARCH}\n", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("cannot inspect RPM package: %s", formatCommandFailure(err, out))
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 3 {
		return fmt.Errorf("unexpected RPM package metadata: %q", strings.TrimSpace(string(out)))
	}
	return verifyPackageIdentity(strings.TrimSpace(lines[0]), strings.TrimSpace(lines[1]), strings.TrimSpace(lines[2]), version, "x86_64")
}

func verifyArchPackage(path, version string) error {
	tool := ""
	if commandExists("bsdtar") {
		tool = "bsdtar"
	} else if commandExists("tar") {
		tool = "tar"
	} else {
		return errors.New("tar/bsdtar is unavailable")
	}
	out, err := exec.Command(tool, "-xOf", path, ".PKGINFO").CombinedOutput()
	if err != nil {
		return fmt.Errorf("cannot inspect Arch package: %s", formatCommandFailure(err, out))
	}
	vals := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		k, v, ok := strings.Cut(line, "=")
		if ok {
			vals[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return verifyPackageIdentity(vals["pkgname"], vals["pkgver"], vals["arch"], version, "x86_64")
}

func verifyPackageIdentity(name, pkgVersion, arch, releaseVersion, wantArch string) error {
	if name == "" || pkgVersion == "" || arch == "" {
		return fmt.Errorf("package metadata is incomplete (name=%q version=%q arch=%q)", name, pkgVersion, arch)
	}
	if name != "citizen-launcher" {
		return fmt.Errorf("refusing package %q", name)
	}
	if compareVersions(pkgVersion, releaseVersion) != 0 {
		return fmt.Errorf("package version %q does not match release %q", pkgVersion, releaseVersion)
	}
	if arch != wantArch {
		return fmt.Errorf("refusing architecture %q", arch)
	}
	return nil
}

func (a *App) applyUserUpdate(asset githubAsset, version string, quiet bool) error {
	if a.selfPath == "" {
		return errors.New("cannot locate running Citizen Launcher executable")
	}
	if !strings.HasPrefix(strings.ToLower(asset.Digest), "sha256:") {
		return errors.New("release asset has no SHA-256 digest/checksum; refusing automatic update")
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
		n, copyErr := io.Copy(out, io.LimitReader(tr, 512<<20+1))
		if copyErr == nil && n > 512<<20 {
			copyErr = errors.New("citizen-launcher binary in tarball exceeds 512 MiB")
		}
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
