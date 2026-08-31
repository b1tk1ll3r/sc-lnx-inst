package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testApp(t *testing.T) *App {
	t.Helper()
	root := t.TempDir()
	self := filepath.Join(root, "bin", "citizen-launcher")
	if err := os.MkdirAll(filepath.Dir(self), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(self, []byte("test"), 0o755); err != nil {
		t.Fatal(err)
	}
	a := &App{
		home:      root,
		configDir: filepath.Join(root, ".config", "citizen-launcher"),
		stateDir:  filepath.Join(root, ".local", "state", "citizen-launcher"),
		dataDir:   filepath.Join(root, ".local", "share", "citizen-launcher"),
		cacheDir:  filepath.Join(root, ".cache", "citizen-launcher"),
		libDir:    filepath.Join(root, ".local", "lib", "citizen-launcher"),
		selfPath:  self,
	}
	a.vendorDir = filepath.Join(a.dataDir, "vendor")
	return a
}

func TestDesktopEntryHasRealNewlinesAndStableLaunch(t *testing.T) {
	a := testApp(t)
	gc := GameConfig{Prefix: filepath.Join(a.home, "Games", "star-citizen")}
	if err := a.writeOwnedLaunchFiles(gc); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(a.starDesktopPath())
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if strings.Contains(s, `\n`) {
		t.Fatalf("desktop contains literal \\n: %q", s)
	}
	if !strings.Contains(s, "Exec=") || !strings.Contains(s, " game-launch\n") {
		t.Fatalf("invalid Exec line: %q", s)
	}
	wrapper := filepath.Join(a.dataDir, "bin", "star-citizen-launch")
	fi, err := os.Stat(wrapper)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&0o111 == 0 {
		t.Fatal("launch wrapper is not executable")
	}
	wb, _ := os.ReadFile(wrapper)
	if strings.Contains(string(wb), `\n`) {
		t.Fatalf("wrapper contains literal newline escape: %q", string(wb))
	}
}

func TestDXVKStateRequiresDLLsAndOverrideMarker(t *testing.T) {
	a := testApp(t)
	prefix := filepath.Join(a.home, "Games", "star-citizen")
	if err := os.MkdirAll(filepath.Join(prefix, "drive_c", "windows", "system32"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"system.reg", "user.reg"} {
		if err := os.WriteFile(filepath.Join(prefix, f), []byte("reg"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range dxvkRequiredDLLs {
		if err := os.WriteFile(filepath.Join(prefix, "drive_c", "windows", "system32", f), make([]byte, 2048), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got := a.dxvkState(prefix, "v3.1"); got != "repair" {
		t.Fatalf("without override marker got %q", got)
	}
	if err := atomicWriteFile(a.dxvkMarkerPath(), []byte("v3.1\n"+prefix+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := a.dxvkState(prefix, "v3.1"); got != "ready" {
		t.Fatalf("complete DXVK got %q", got)
	}
	if err := os.Remove(filepath.Join(prefix, "drive_c", "windows", "system32", "dxgi.dll")); err != nil {
		t.Fatal(err)
	}
	if got := a.dxvkState(prefix, "v3.1"); got != "repair" {
		t.Fatalf("missing DLL got %q", got)
	}
}

func TestEnvironmentReplacementKeepsManagedWinePath(t *testing.T) {
	env := []string{"HOME=/tmp/x", "PATH=/managed/wine/bin:/usr/bin", "LD_LIBRARY_PATH=/managed/lib"}
	env = setEnvValue(env, "PATH", "/toolbox/bin:"+envValue(env, "PATH"))
	if got := envValue(env, "PATH"); got != "/toolbox/bin:/managed/wine/bin:/usr/bin" {
		t.Fatalf("PATH lost runner: %q", got)
	}
	count := 0
	for _, v := range env {
		if strings.HasPrefix(v, "PATH=") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("duplicate PATH entries: %#v", env)
	}
}

func TestSingleInstanceAndStackLocks(t *testing.T) {
	a := testApp(t)
	first, ok, err := a.acquireGUILock()
	if err != nil || !ok {
		t.Fatalf("first GUI lock: ok=%v err=%v", ok, err)
	}
	defer releaseFileLock(first)
	second, ok, err := a.acquireGUILock()
	if err != nil || ok || second != nil {
		t.Fatalf("second GUI lock must be refused: ok=%v err=%v", ok, err)
	}

	stack, err := a.stackOperationLock(20 * time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	defer releaseFileLock(stack)
	if other, err := a.stackOperationLock(20 * time.Millisecond); err == nil || other != nil {
		t.Fatal("second stack lock unexpectedly succeeded")
	}
}

func TestSupportSanitizer(t *testing.T) {
	in := []byte("/home/tester/Games mail=a@example.com ip=192.168.1.4 MAC=aa:bb:cc:dd:ee:ff token=supersecret Bearer abc.def.ghi")
	out := string(sanitizeSupport(in, "/home/tester"))
	for _, secret := range []string{"/home/tester", "a@example.com", "192.168.1.4", "aa:bb:cc:dd:ee:ff", "supersecret", "Bearer abc.def.ghi"} {
		if strings.Contains(out, secret) {
			t.Fatalf("sanitizer leaked %q in %q", secret, out)
		}
	}
}

func TestReleaseRepoValidation(t *testing.T) {
	for _, good := range []string{"owner/repo", "a-b/c_d", "org.name/project.name"} {
		if !validReleaseRepo(good) {
			t.Fatalf("valid repo rejected: %q", good)
		}
	}
	for _, bad := range []string{"", "owner", "a/b/c", "https://evil/x", "a/../b", "a/b;cmd"} {
		if validReleaseRepo(bad) {
			t.Fatalf("invalid repo accepted: %q", bad)
		}
	}
}

func TestRootUpdaterIgnoresEnvironmentRepository(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("root-only trust-boundary test")
	}
	t.Setenv("CITIZEN_LAUNCHER_RELEASE_REPO", "evil/redirect")
	if got := effectiveReleaseRepo(); got == "evil/redirect" {
		t.Fatal("privileged updater trusted user environment")
	}
}

func TestFrontendDoesNotDoubleBindPrimaryAction(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("web", "app.js"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if strings.Contains(s, "$('primary').onclick") {
		t.Fatal("primary action has a second direct click handler")
	}
	if !strings.Contains(s, "document.addEventListener('click'") {
		t.Fatal("delegated action handler missing")
	}
}

func TestDXVKNativeOverrideSetIsComplete(t *testing.T) {
	want := []string{"d3d8", "d3d9", "d3d10core", "d3d11", "dxgi"}
	if strings.Join(dxvkOverrideNames, ",") != strings.Join(want, ",") {
		t.Fatalf("DXVK override set=%v", dxvkOverrideNames)
	}
}

func TestProcessEnvMatchingIsExact(t *testing.T) {
	prefix := "/home/user/Games/star-citizen"
	env := []byte("HOME=/home/user\x00WINEPREFIX=" + prefix + "-old\x00PATH=/usr/bin\x00")
	if processHasEnvValue(env, "WINEPREFIX", prefix) {
		t.Fatal("prefix matcher accepted a longer different prefix")
	}
	env = append(env, []byte("WINEPREFIX="+prefix+"\x00")...)
	if !processHasEnvValue(env, "WINEPREFIX", prefix) {
		t.Fatal("prefix matcher missed exact WINEPREFIX")
	}
}

func TestPinnedWinetricksIsIntegrityCheckedAndMinimal(t *testing.T) {
	if winetricksVersion != "20260125" {
		t.Fatalf("unexpected pinned Winetricks version %q", winetricksVersion)
	}
	if len(winetricksSHA256) != 64 {
		t.Fatalf("Winetricks checksum length=%d", len(winetricksSHA256))
	}
	for _, v := range basePrefixWinetricksVerbs {
		if strings.EqualFold(v, "powershell") || strings.EqualFold(v, "dxvk") {
			t.Fatalf("fragile/duplicate Winetricks verb must not be in base prefix: %s", v)
		}
	}
}

func TestDebMigrationRemovesShadowBinaryAndLegacyDesktop(t *testing.T) {
	a := testApp(t)
	fakeBin := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatal(err)
	}
	dpkg := filepath.Join(fakeBin, "dpkg-query")
	if err := os.WriteFile(dpkg, []byte("#!/bin/sh\nprintf 'install ok installed\\n1.0.0\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	oldBin := filepath.Join(a.home, ".local", "bin", "citizen-launcher")
	if err := os.MkdirAll(filepath.Dir(oldBin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldBin, []byte("#!/bin/sh\necho 0.9.2\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(a.mainDesktopPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.mainDesktopPath(), []byte("[Desktop Entry]\nName=Citizen Launcher\nExec="+oldBin+" gui\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	a.migrateUserInstallUnlocked()
	if _, err := os.Stat(oldBin); !os.IsNotExist(err) {
		t.Fatalf("shadow binary survived migration: %v", err)
	}
	if _, err := os.Stat(a.mainDesktopPath()); !os.IsNotExist(err) {
		t.Fatalf("legacy desktop survived migration: %v", err)
	}
}

func TestRepairDesktopIntegrationFixesLiteralNewlineRegression(t *testing.T) {
	a := testApp(t)
	gc := a.loadGameConfig()
	if err := os.MkdirAll(filepath.Dir(gc.LauncherEXE), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gc.LauncherEXE, []byte("fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(a.starDesktopPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	bad := "[Desktop Entry]\\nName=Star Citizen\\nExec=/old/star-citizen-launch\\n"
	if err := os.WriteFile(a.starDesktopPath(), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	a.repairDesktopIntegrationUnlocked()
	b, err := os.ReadFile(a.starDesktopPath())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), `\\n`) {
		t.Fatalf("literal newline regression was not repaired: %q", b)
	}
	if !strings.Contains(string(b), "Exec=") || !strings.Contains(string(b), " game-launch\n") {
		t.Fatalf("repaired desktop invalid: %q", b)
	}
}

func TestCopyFileEnforcesModeOnExistingDestination(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	if err := os.WriteFile(src, []byte("payload"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := copyFile(src, dst, 0o755); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o755 {
		t.Fatalf("copy mode=%o want 755", fi.Mode().Perm())
	}
}
