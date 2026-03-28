# 🔐 envdesk

Git-friendly env operations for teams.

`envdesk` is a local-first CLI for managing encrypted env files across services and environments.

## ✨ Why envdesk?

- lighter than a full secrets platform when you want encrypted env files in Git
- safer than passing `.env` files by hand because edit and export stay explicit
- team-oriented because review, lint, diff, and drift checks are built into the CLI

## 🎯 Positioning

- `envdesk` is not a vault and does not replace a hosted secrets manager
- `envdesk` is a thin UX layer over SOPS + age for repository-local env workflows
- `envdesk` focuses on schema-aware editing, review, drift detection, and key sync for teams

Next to [dotenvx](https://dotenvx.com/) (encrypt dotenv-style files and keep an app-side `dotenv` workflow), `envdesk` is more opinionated about Git review workflows, schema validation, drift checks, and SOPS-based encryption. Next to [git-crypt](https://github.com/AGWA/git-crypt) (transparent encryption for selected paths in a Git repo), `envdesk` keeps env operations explicit and adds env-aware diff, lint, sync, and example generation.

## 📦 Install

Install with:

```bash
# Homebrew (macOS / Linux) — [third-party tap](https://github.com/mhiro2/homebrew-tap)
brew install --cask mhiro2/tap/envdesk
```

```bash
# Go toolchain
go install github.com/mhiro2/envdesk/cmd/envdesk@latest
```

Release builds embed version metadata, so `envdesk --version` reports the build identity from the release workflow.

## 📋 Prerequisites

- **Encrypted workflows:** `sops` and `age` on your `PATH`, plus keys your team can decrypt with. Install them with your package manager or from releases ([SOPS](https://github.com/getsops/sops/releases), [age](https://github.com/FiloSottile/age/releases)). With [Homebrew](https://brew.sh) (macOS or Linux):

  ```bash
  brew install sops age
  ```

  Keys, `SOPS_AGE_KEY_FILE`, and a full walkthrough live in [docs/getting-started.md](./docs/getting-started.md). Read that before `init --sops` or `edit` if you are new to the stack.
- **`edit`:** set `EDITOR` or pass `--editor` (on Windows, values such as `notepad.exe` or `code.cmd --wait` work).

## 🚀 Getting started

This is the shortest path from an empty repository to a reviewable encrypted env workflow.

### 1. Scaffold one service and one environment

```bash
envdesk init --services api --envs dev --sops
```

`init` creates a minimal repository layout:

```text
envdesk.yaml
.sops.yaml
env/
  api/
    dev.env
env.schema/
  api.yaml
```

Generated `envdesk.yaml`:

```yaml
version: 1

services:
  - name: api
    schema: env.schema/api.yaml
    files:
      dev: env/api/dev.env
```

Generated `.sops.yaml`:

```yaml
creation_rules:
  - path_regex: ^env/.*\.env$
    age: ""
```

The scaffolded schema starts with `APP_ENV`.

If you want the initial env file encrypted from the first write, add `--encrypt --age <recipient>`.
The `recipient` is your team's age public key such as `age1...`.

### 2. Edit the encrypted env file

```bash
envdesk edit api dev
```

`edit` decrypts through `sops`, opens the plaintext in your editor, validates the result, and re-encrypts it on success.

### 3. Export plaintext only when you need it locally

```bash
envdesk export api dev --out .env.local
```

Use `export` as the explicit plaintext escape hatch for short-lived local workflows.

### 4. Review structure before committing

```bash
envdesk lint --service api
envdesk check-sync --strict-required-only
```

`lint` checks env files against the schema. `check-sync` catches required-key drift across environments (more valuable once you add `stg` and `prod`).
`envdesk check-sync --json` still exits non-zero when drift is present so CI can consume JSON without losing failure signals.
`envdesk status` combines per-environment lint state, sync state, and last updated time into one dashboard.
`envdesk audit` shows the history view: who last changed a key, how schema metadata moved, and when the current drift state started.

Other review and maintenance commands are in the [quick reference](#command-quick-reference) below; the full flow is in [docs/guide.md](./docs/guide.md).

### 5. When the repository grows

After the first setup, common next steps include:

- compare environments with `envdesk diff api dev stg --value-mode public`
- align missing keys with `envdesk sync-keys api dev --to stg --placeholders`
- inspect when drift started with `envdesk audit --service api --env dev --key DATABASE_URL`
- manage access with `envdesk member add alice.pub --scope api --dry-run` and `envdesk rekey`
- generate onboarding examples with `envdesk example generate`

## 🛠️ Command quick reference

| Goal | Example |
| --- | --- |
| Edit one environment (decrypt → editor → validate → encrypt) | `envdesk edit api dev` |
| Export plaintext for local use | `envdesk export api dev --out .env.local` |
| Schema validation | `envdesk lint --service api` |
| Required-key drift across environments | `envdesk check-sync --strict-required-only` |
| Lint + sync + last-updated overview | `envdesk status --service api` |
| Trace key ownership, schema changes, and drift start dates | `envdesk audit --service api --env dev --key DATABASE_URL` |
| Compare two environments (secrets hidden by default) | `envdesk diff api dev stg --value-mode public` |
| Preview recipient / scope changes | `envdesk member add alice.pub --scope api --dry-run` |
| Re-encrypt after key changes | `envdesk rekey --service api --env dev --dry-run` |
| Generate `.env.example` for onboarding | `envdesk example generate --service api --out env/api/.env.example` |
| Local tool and repo safety | `envdesk doctor --json` |

For every subcommand, flags, and workflows, see [docs/guide.md](./docs/guide.md).

## ⚙️ Project config (`envdesk.yaml`)

This file maps **services** and **environments** to on-disk env paths and to a **schema file per service**. It does not define the syntax of lines inside `*.env` files (see **Env file syntax** below).

`envdesk.yaml` uses a strict schema:

```yaml
version: 1

services:
  - name: api
    schema: env.schema/api.yaml
    files:
      dev: env/api/dev.env
      stg: env/api/stg.env
      prod: env/api/prod.env
```

All configured `schema` and env file paths must stay inside the repository root that contains `envdesk.yaml`.
Paths that escape with absolute locations or `../` are rejected during config load.

Per-service schema files (`env.schema/*.yaml`) declare keys, types, requiredness, and secret-ness. Commands such as `diff`, `check-sync`, `status`, `audit`, `sync-keys`, and `example generate` use that metadata for safer output, drift classification, audit timelines, placeholders, and commented examples. See [docs/guide.md](./docs/guide.md) for the schema model and behavior.

## 📝 Env file syntax (`*.env` files)

This section is about **assignments inside env files** (for example `KEY=value` or `export KEY=value`), not about `envdesk.yaml` or schema YAML.

`envdesk` supports a **small** dotenv-style subset: blank lines and full-line `#` comments, `KEY=value` / `export KEY=value`, and common quoting rules. It does **not** implement shell expansion, command substitution, multiline values, or full compatibility with every dotenv variant.

`edit` keeps your validated plaintext formatting on save; `sync-keys` and `example generate` emit normalized output instead. Writes use atomic replacement where applicable.

Details: [Env file dialect and normalization](./docs/guide.md#env-file-dialect-and-normalization) in [docs/guide.md](./docs/guide.md).

## ✅ CI and hooks

Sample Git hooks live under [`hooks/`](./hooks); [`scripts/check-env-conventions.sh`](./scripts/check-env-conventions.sh) is a small guard for what gets committed. Those samples assume a Bash-compatible shell. The envdesk CLI itself supports Windows; adapt hooks or run the same checks in CI on runners that match your stack.

**Checks to run in CI (or locally before push)** — each step catches a different class of problem:

1. **No accidental plaintext env in Git** — `bash scripts/check-env-conventions.sh` walks tracked `*.env` paths (skipping `*.example`) and fails if a file lacks SOPS markers (`ENC[` or `sops:`), so plaintext secrets are not committed by mistake.
2. **Schema match** — `envdesk lint` validates every configured env file against its service schema (types, required keys, secrets metadata).
3. **Cross-environment drift** — `envdesk check-sync --strict-required-only --json` compares required keys across environments; JSON is easy to log in CI and the process still exits non-zero when drift exists.

```bash
bash scripts/check-env-conventions.sh
envdesk lint
envdesk check-sync --strict-required-only --json
```

Encrypted files need `sops` and an age identity in that environment (for example `SOPS_AGE_KEY_FILE`). Step-by-step jobs for **GitHub Actions** and **GitLab** are in [docs/ci-integration.md](./docs/ci-integration.md).

**Local developer sanity check:** `envdesk doctor` reports missing tools, weak `.gitignore` coverage for plaintext exports, tracked risky files, file modes, and whether the tree looks `encrypted`, `plaintext`, or `mixed`.

## 📚 Learn more

- [docs/getting-started.md](./docs/getting-started.md): SOPS and age from zero, first encrypted layout, and Git hygiene
- [docs/guide.md](./docs/guide.md): step-by-step workflows, command usage, local hooks, onboarding, and operational guidance
- [docs/ci-integration.md](./docs/ci-integration.md): GitHub Actions and GitLab CI job examples
- [ARCHITECTURE.md](./ARCHITECTURE.md): package layout, data flow, command responsibilities, and security model

## 📝 License

MIT
