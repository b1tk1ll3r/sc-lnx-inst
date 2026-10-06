# Self-hosted GitHub Runner fix (Debian 13)

This archive fixes the two runner-specific problems seen in the workflow logs:

1. **Root-owned files in `GITHUB_WORKSPACE`**
   - The old Fedora, openSUSE, Arch and release E2E jobs used GitHub Actions
     job-level `container:` execution.
   - On a persistent self-hosted runner this can leave files in the mounted
     workspace owned by root. A later `actions/checkout` then fails at
     `git clean -ffdx` with `Permission denied`.
   - The corrected workflows run package/test containers explicitly with
     `docker run`/`docker create` and keep the checkout read-only inside those
     containers.
   - Package files are copied back with `docker cp`, so they are created as the
     runner user on the host.
   - Before each checkout, a small recovery step also normalizes ownership of
     an existing workspace. This allows the first run of the corrected workflow
     to recover from the root-owned files left by previous runs.

2. **`sudo apt-get` inside a non-interactive workflow**
   - The workflow no longer tries to install host dependencies using `sudo`.
   - It verifies that the dependencies are present and prints an actionable
     error if they are missing.

Install the verification dependencies once on the Debian 13 runner host from an
interactive SSH/session:

```bash
sudo apt-get update
sudo apt-get install -y --no-install-recommends \
  python3-yaml dpkg-dev zstd jq shellcheck openssl
```

The runner account must also be able to access Docker without `sudo`. For a
runner started as user `groot`, for example:

```bash
sudo usermod -aG docker groot
```

Then restart the runner process/service so it receives the updated supplementary
group membership.

## Files

Copy/extract the archive into the repository root so these files replace the
existing workflows:

- `.github/workflows/ci.yml`
- `.github/workflows/packages.yml`
- `.github/workflows/release.yml`
