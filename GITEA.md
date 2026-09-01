# Gitea Actions setup

Citizen Launcher 1.1.3 uses Gitea Actions natively. Workflows are in `.gitea/workflows/`; the old `.github/workflows/` directory is intentionally absent.

## Requirements

- Gitea Actions enabled for the repository.
- An `act_runner` with an `ubuntu-latest` label.
- Docker-capable runner execution, because Fedora and Arch package jobs use job containers.
- Outbound HTTPS access for Go modules, upstream package metadata and the referenced `actions/checkout@v4` / `actions/setup-go@v5` actions.

Gitea Actions is GitHub-Actions compatible enough to run those standard actions. By default Gitea resolves `actions/...` from GitHub. An installation that wants to avoid that external dependency can mirror the two actions into its own Gitea and configure the instance's default actions URL accordingly.

## CI

`.gitea/workflows/ci.yml` runs on pushes and pull requests:

- full regression/race/package verification on the Ubuntu runner;
- RPM build and inspection inside Fedora;
- pacman package build as an unprivileged user inside Arch Linux.

If your runner uses a different label, replace `runs-on: ubuntu-latest` in both workflow files.

## Releases

Push a version tag matching `VERSION`, for example:

```bash
git tag v1.1.3
git push origin v1.1.3
```

`.gitea/workflows/release.yml` then:

1. verifies tag ↔ `VERSION`;
2. creates (or reuses on rerun) the Gitea Release;
3. builds Debian/generic, Fedora RPM and Arch packages;
4. uploads each package directly to the Gitea Release;
5. downloads the release packages again and creates a deterministic `SHA256SUMS.txt`;
6. uploads the checksum manifest.

The workflow uses Gitea's built-in job token with:

```yaml
permissions:
  code: read
  releases: write
```

No personal access token and no `gh` CLI are needed.

## Self-update source

Release builds set:

```text
gitea:<GITEA_API_URL>/repos/<owner>/<repo>
```

inside `/etc/citizen-launcher/release-repo`. Example:

```text
gitea:https://git.example.org/api/v1/repos/games/citizen-launcher
```

Citizen Launcher resolves `/releases/latest`, reads the matching release package and uses `SHA256SUMS.txt` to obtain its SHA-256 before any native package update. Privileged Gitea update sources must be HTTPS.

Existing `github:owner/repo` and legacy `owner/repo` sources remain supported.

## Reruns

The release helper `scripts/gitea-release.sh` is intentionally rerun-safe. Existing same-named attachments are removed before a replacement is uploaded, so rerunning a failed release does not create duplicate package assets.

## Small runner / Fedora RPM jobs

The Fedora jobs are intentionally written for constrained `act_runner` Docker roots. Do not replace the staged dependency setup with one large command such as:

```text
dnf install git golang rpm-build systemd-rpm-macros curl python3
```

That pulls Fedora's full Go source package, weak RPM build dependencies and other tooling at the same time and can exceed a small container root filesystem before `rpmbuild` starts.

The 1.1.3 workflow instead:

1. installs only `git-core` for checkout;
2. stores DNF cache data on `${{ gitea.workspace }}` rather than `/var/cache/libdnf5`;
3. disables weak dependencies and docs in the disposable build container;
4. installs `golang-bin`, builds the static backend, then removes the compiler;
5. installs `rpm-build` only after Go has been removed;
6. uses `jq` instead of Python for the RPM release upload path.

The workflow prints `df -h /` after the major phases. If a runner still cannot fit the native Fedora build after this reduction, increase the Docker/container root storage rather than silently dropping RPM verification.
