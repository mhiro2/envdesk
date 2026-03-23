# CI integration

This document shows how to run `envdesk` checks in **GitHub Actions** and **GitLab CI**.

For local hooks and the convention script that rejects plaintext tracked env files, see [docs/guide.md](./guide.md#ci-and-hooks).

## What to run in CI

Typical gates:

```bash
bash scripts/check-env-conventions.sh   # optional; no decryption
envdesk lint
envdesk check-sync --strict-required-only --json
```

`check-env-conventions.sh` only inspects whether tracked `*.env` files look encrypted (SOPS markers). It does **not** need age keys.

`envdesk lint` and `envdesk check-sync` **decrypt** env files through `sops`, so the job must have an age **identity** that matches a recipient in `.sops.yaml` (same as local development).

## Secrets and safety

- Store the age **private** key in CI secrets (GitHub **Actions** secrets / GitLab **CI/CD variables**). Never commit it.
- Use a dedicated CI age identity if you want to revoke automation without rotating human keys.
- **GitHub**: secrets are not available to workflows triggered from fork pull requests by default. If you skip envdesk on fork PRs, document that limitation for contributors.

## GitHub Actions

Example job: install tools, expose the age key to SOPS, then run checks.

Adjust `SOPS_VERSION` and paths to match your policy. The snippet below follows the same sops install style as this repository’s E2E workflow.

```yaml
name: envdesk

on:
  pull_request:
  push:
    branches:
      - main

permissions:
  contents: read

jobs:
  envdesk:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v6
        with:
          go-version: "1.26"

      - name: Install envdesk
        run: |
          go install github.com/mhiro2/envdesk/cmd/envdesk@latest
          echo "$(go env GOPATH)/bin" >> "$GITHUB_PATH"

      - name: Install sops
        env:
          SOPS_VERSION: v3.10.2
        run: |
          curl -fsSL "https://github.com/getsops/sops/releases/download/${SOPS_VERSION}/sops-${SOPS_VERSION}.linux.amd64" -o /usr/local/bin/sops
          chmod +x /usr/local/bin/sops

      - name: Install age
        run: |
          sudo apt-get update
          sudo apt-get install -y age

      - name: Configure SOPS age key
        env:
          SOPS_AGE_KEY: ${{ secrets.SOPS_AGE_KEY }}
        run: |
          keyfile="${RUNNER_TEMP}/sops-age.key"
          umask 077
          echo "$SOPS_AGE_KEY" > "$keyfile"
          chmod 600 "$keyfile"
          echo "SOPS_AGE_KEY_FILE=$keyfile" >> "$GITHUB_ENV"

      - name: Check tracked env files look encrypted
        run: bash scripts/check-env-conventions.sh

      - name: envdesk lint
        run: envdesk lint

      - name: envdesk check-sync
        run: envdesk check-sync --strict-required-only --json
```

Create a repository secret named `SOPS_AGE_KEY`. Store the same material you would put in an age identity file: usually the `AGE-SECRET-KEY-...` line, or the full `keys.txt` content your team uses locally (including comment lines), as long as `sops` can decrypt with `SOPS_AGE_KEY_FILE` pointing at that file.

### Go module repositories

If the workflow already uses `go-version-file: go.mod`, you can install `envdesk` with the same Go toolchain instead of pinning `go-version` only for this job.

### Releases instead of `go install`

If you do not want Go in CI, download a release binary from [GitHub Releases](https://github.com/mhiro2/envdesk/releases) for `envdesk` and add it to `PATH`.

## GitLab CI

Example `.gitlab-ci.yml` fragment. Use a **masked** CI/CD variable; for multiline private keys, prefer a [file-type variable](https://docs.gitlab.com/ee/ci/variables/#cicd-variable-types) if your GitLab version supports it.

```yaml
stages: [check]

variables:
  SOPS_VERSION: "v3.10.2"
  # When using a "File" variable, GitLab sets the path automatically:
  # SOPS_AGE_KEY_FILE: "$SOPS_AGE_KEY"  # if variable type is File and named SOPS_AGE_KEY

.envdesk:
  image: golang:1.26-bookworm
  before_script:
    - apt-get update && apt-get install -y --no-install-recommends curl ca-certificates age
    - |
      curl -fsSL "https://github.com/getsops/sops/releases/download/${SOPS_VERSION}/sops-${SOPS_VERSION}.linux.amd64" -o /usr/local/bin/sops
      chmod +x /usr/local/bin/sops
    - go install github.com/mhiro2/envdesk/cmd/envdesk@latest
    - export PATH="$(go env GOPATH)/bin:$PATH"
    # If SOPS_AGE_KEY is a string variable (not File), write it to a temp file:
    - |
      keyfile="$(mktemp)"
      umask 077
      echo "$SOPS_AGE_KEY" > "$keyfile"
      chmod 600 "$keyfile"
      export SOPS_AGE_KEY_FILE="$keyfile"

envdesk:
  stage: check
  extends: .envdesk
  script:
    - bash scripts/check-env-conventions.sh
    - envdesk lint
    - envdesk check-sync --strict-required-only --json
```

Set `SOPS_AGE_KEY` in GitLab CI/CD variables (masked, protected if needed). If you use a **File** variable, replace the `mktemp` block with exporting the path GitLab provides for that variable.

## Reference in this repository

- [`.github/workflows/e2e.yaml`](../.github/workflows/e2e.yaml) — installs sops and age for automated tests
- [`scripts/check-env-conventions.sh`](../scripts/check-env-conventions.sh) — plaintext guard for tracked env files
- [`hooks/pre-push.sample`](../hooks/pre-push.sample) — local example invoking `envdesk check-sync`
