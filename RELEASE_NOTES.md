# Citizen Launcher 1.1.1

## Gitea-native CI/CD release

1.1.1 keeps the confirmed playable multi-distribution 1.1.x runtime and migrates the project automation from GitHub Actions to Gitea Actions.

### Gitea workflows

- workflows now live exclusively under `.gitea/workflows/`
- CI keeps the full Go/race/package verification plus Fedora RPM and Arch package builds
- tag releases are created with Gitea's REST API and built-in `GITEA_TOKEN`
- release publishing no longer depends on the `gh` CLI
- release jobs upload `.deb`, `.rpm`, `.pkg.tar.zst`, generic tarball and one combined `SHA256SUMS.txt`
- upload is idempotent: a rerun replaces same-named release attachments instead of duplicating them

### Gitea-aware self-update

Packages produced by Gitea Actions embed an exact `gitea:<api-repository-url>` update source. The launcher can now resolve Gitea's latest-release API and hydrate per-asset SHA-256 values from the workflow-generated `SHA256SUMS.txt`. GitHub release sources remain supported for existing installations.

For privileged updates, a configured Gitea source must use HTTPS. This preserves the fail-closed package-update model.

### Runner portability

The release flow does not depend on cross-job `upload-artifact` compatibility. Each native build uploads its package directly to the Gitea Release, and the final job downloads those release attachments to create the checksum manifest. This works across a wider range of act_runner versions.

### Runtime

Wine, DXVK, RSI Launcher setup, hardware checks, single-instance protection, repair, support bundles and the already confirmed playable Star Citizen path are unchanged.
