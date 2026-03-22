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

## ⚖️ Comparison

| Tool | Best fit | How `envdesk` differs |
| --- | --- | --- |
| `dotenvx` | sharing and syncing dotenv-style env files | `envdesk` is more opinionated about Git review workflows, schema validation, drift checks, and SOPS-based encryption |
| `git-crypt` | transparent file encryption for a broader set of files in a repo | `envdesk` keeps env operations explicit and adds env-aware diff, lint, sync, and example generation |

## 📦 Install

Install with:

```bash
go install github.com/mhiro2/envdesk/cmd/envdesk@latest
```

Release builds embed version metadata, so `envdesk --version` reports the build identity from the release workflow.

## 🚀 Getting Started

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
    age: []
```

The scaffolded schema starts with `APP_ENV`.

If you want the initial env file encrypted from the first write, add `--encrypt --age <recipient>`.
The `recipient` is your team's age public key such as `age1...`.

### 2. Edit the encrypted env file

Open one env file, edit it safely, and write it back encrypted.

```bash
envdesk edit api dev
```

`edit` decrypts through `sops`, opens the plaintext in your editor, validates the result, and re-encrypts it on success.
Set `EDITOR` or pass `--editor`; on Windows, values such as `notepad.exe` or `code.cmd --wait` work.

### 3. Export plaintext only when you need it locally

Export plaintext only for short-lived local use.

```bash
envdesk export api dev --out .env.local
```

Use `export` as the explicit plaintext escape hatch for short-lived local workflows.

### 4. Review structure before committing

Check that the file still matches schema.

```bash
envdesk lint --service api
```

Check whether required keys drifted across environments.

```bash
envdesk check-sync --strict-required-only
```

`lint` checks the env file against schema. `check-sync` catches key drift across environments, so it becomes more useful as you add `stg` and `prod`.
`envdesk check-sync --json` still exits non-zero when drift is present so CI can consume JSON without losing failure signals.

### 5. When the repository grows

After the first setup, the next common jobs are:

- compare environments with `envdesk diff api dev stg --value-mode public`
- align missing keys with `envdesk sync-keys api dev --to stg --placeholders`
- manage access with `envdesk member add alice.pub --scope api --dry-run` and `envdesk rekey`
- generate onboarding examples with `envdesk example generate`

For the full operational flow, see [docs/guide.md](./docs/guide.md).

## 🛠️ Typical Workflows

### Daily editing

Edit one environment safely.

```bash
envdesk edit api dev
```

Export plaintext for local app startup or debugging.

```bash
envdesk export api dev --out .env.local
```

### Review before merge

Validate one service against schema.

```bash
envdesk lint --service api
```

Check whether required keys drifted across environments.

```bash
envdesk check-sync --strict-required-only
```

Compare two environments without exposing secrets by default.

```bash
envdesk diff api dev stg --value-mode public
```

### Access and repository maintenance

Preview recipient changes before granting access.

```bash
envdesk member add alice.pub --scope api --dry-run
```

Re-encrypt files after recipient changes.

```bash
envdesk rekey --service api --env dev --dry-run
```

Generate a shareable example file for onboarding.

```bash
envdesk example generate --service api --out env/api/.env.example
```

Check local setup and repository safety.

```bash
envdesk doctor --json
```

See [docs/guide.md](./docs/guide.md) for the full command set and detailed workflows.

## ⚙️ Configuration

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

Schema files define key requirements, types, and whether a key is secret. `diff`, `check-sync`, `sync-keys`, and `example generate` all use that metadata for safer output, drift classification, placeholder generation, and commented examples.
See [docs/guide.md](./docs/guide.md) for the current schema model and command behavior.

## 📝 Env File Dialect

`envdesk` intentionally supports a small dotenv subset through `internal/envfile`.

- blank lines and full-line `#` comments are accepted
- `KEY=value` assignments use keys in `[A-Za-z_][A-Za-z0-9_]*`
- an `export` prefix is accepted
- unquoted values, double-quoted escape sequences, and single-quoted literals are accepted
- inline comments are recognized only for unquoted values when `#` is preceded by whitespace

`envdesk` does not implement shell expansion, command substitution, multiline values, heredocs, or full compatibility with every dotenv/export dialect.

`edit` now preserves the edited plaintext bytes after validation, so supported comments and quoting survive a round trip.
`sync-keys` and `example generate` still write normalized output, which means comments, blank lines, and original quoting are not preserved there.
`edit`, `sync-keys`, and `.sops.yaml` updates use atomic replacement writes to reduce the risk of partial file corruption.

## ✅ CI And Hooks

The repository includes hook examples under [`hooks/`](./hooks) and helper scripts under [`scripts/`](./scripts).
The core CLI supports Windows, but the sample hooks and helper script assume a Bash-compatible shell.

Recommended checks:

```bash
bash scripts/check-env-conventions.sh
envdesk lint
envdesk check-sync --json
```

`envdesk doctor` is the repository-level safety check for local setups. It detects missing tools, unsafe `.gitignore` rules, tracked plaintext env files, and permissive plaintext file modes.
It also reports whether the repository currently looks like `encrypted`, `plaintext`, or `mixed` mode.

## 📚 Learn More

- [docs/guide.md](./docs/guide.md): step-by-step workflows, command usage, CI hooks, onboarding, and operational guidance
- [ARCHITECTURE.md](./ARCHITECTURE.md): package layout, data flow, command responsibilities, and security model

## 📝 License

MIT
