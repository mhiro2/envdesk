# Guide

## Prerequisites

`envdesk` assumes a Git repository with encrypted env files managed by SOPS and age.

- **Supported platforms:** macOS, Linux, Windows
- **`sops` on `PATH`** for `edit`, `export`, and `rekey`
- **`age` or a local age key** for `doctor` and any flow that decrypts
- **`envdesk.yaml`**, or run `envdesk init` to create one

If SOPS and age are new to you, start with [docs/getting-started.md](./getting-started.md).

## Repository layout

The default layout is (see also [Project config (`envdesk.yaml`)](../README.md#-project-config-envdeskyaml) in the README):

```text
repo/
  envdesk.yaml
  .sops.yaml
  env/
    api/
      dev.env
      stg.env
      prod.env
  env.schema/
    api.yaml
```

`envdesk init` scaffolds the layout, and `--services` and `--envs` control the generated service and environment set.
Use `--encrypt --age <recipient>` when you want the initial `env/*.env` files written in encrypted form from the first scaffold.
The `recipient` is an age public key such as `age1...`.

`envdesk.yaml` is treated as repository-local configuration.
Configured `schema` and env file paths must remain under the directory that contains `envdesk.yaml`.
Absolute paths or `../` escapes outside that repository root are rejected.

## Typical workflow

1. Run `envdesk init` for a fresh repository.
2. Edit encrypted env files with `envdesk edit <service> <environment>`.
3. Export plaintext locally with `envdesk export <service> <environment> --out .env.local`.
4. Validate structure with `envdesk lint` and `envdesk check-sync`.
5. Reconcile recipients with `envdesk member add/remove` and `envdesk rekey`.

## Safe editing

`envdesk edit` decrypts a target env file through `sops`, opens the plaintext in `$EDITOR` or `--editor`, re-parses the edited content, validates it against schema unless `--no-lint` is set, and re-encrypts on success.
On Windows, editors such as `notepad.exe` and `code.cmd --wait` work through the native command shell.

The decrypted file is written to a temporary path and removed after the edit session ends.
Encrypted env writes and `.sops.yaml` updates use atomic replacement writes to reduce the chance of leaving a partially written file behind.

When the edited file still matches the supported env dialect, `envdesk edit` re-encrypts the edited plaintext as-is instead of normalizing it first.

## Exporting plaintext

`envdesk export` is the explicit plaintext escape hatch.

- `--out` writes a private file mode where the platform honors POSIX permission bits
- `--stdout` streams plaintext to standard output
- `--force` allows overwriting an existing output file

Use `export` only for short-lived local workflows such as creating `.env.local`.

## Validation

`envdesk lint` validates env files against their schemas.

- missing required keys are reported as errors
- invalid values are reported as errors
- undeclared keys are reported as warnings

`envdesk diff` compares environments by key set and can show additional detail with `--value-mode`.

- `hidden` prints change markers without exposing values
- `hash` prints short SHA-256 digests for value comparison
- `public` prints only schema-declared non-secret values
- `all` prints all values

`envdesk check-sync` reports key drift across environments and classifies each issue as `required`, `optional`, `undeclared`, or `untracked`.
Use `--strict-required-only` when CI should fail only on missing required schema keys.
`--json` keeps the same non-zero exit behavior when drift exists so machine-readable output still works in CI.

`envdesk status` gives a service-by-environment dashboard with lint state, sync state, and each env file's last updated time.
Use it when you want one review-oriented snapshot before a change or release.

`envdesk sync-keys --placeholders` inserts schema-aware defaults for missing keys.

- `bool` becomes `false`
- `int` becomes `0`
- `enum` prefers the target environment name when it is an allowed value, otherwise the first allowed value
- `secret` stays empty

`envdesk example generate` writes `.env.example` entries with schema metadata comments so requiredness, type, secret-ness, and enum values are visible in the generated file.

## Env file dialect and normalization

This section describes **syntax inside `*.env` files** (assignments and comments), not `envdesk.yaml` or schema YAML. For the latter, see [Project config (`envdesk.yaml`)](../README.md#-project-config-envdeskyaml).

`envdesk` uses a deliberately small parser for env files.

- supported: blank lines, full-line `#` comments, `export KEY=value`, unquoted values, double-quoted escapes, single-quoted literals
- limited: inline comments are recognized only for unquoted values with whitespace before `#`
- unsupported: shell expansion, command substitution, multiline values, heredocs, full dotenv dialect compatibility

`edit` preserves supported formatting on successful validation.
`sync-keys` and `example generate` emit normalized output, so comments, blank lines, and original quoting are not preserved there.

`envdesk doctor` validates repository safety and local readiness.

- detected repository mode (`encrypted`, `plaintext`, or `mixed`)
- missing `sops`
- missing `age` or a local age key
- tracked plaintext env files
- missing ignore rules for plaintext outputs
- permissive plaintext env file modes on platforms with POSIX permission bits

## CI and hooks

Sample hooks and scripts ship with this repository:

- `hooks/pre-commit.sample` and `hooks/pre-push.sample` — copy into `.git/hooks` to run checks before commit or push
- `scripts/check-env-conventions.sh` — fails when tracked `*.env` paths (except `*.example`) lack SOPS markers (`ENC[` or `sops:`)

They assume a Bash-compatible shell. On Windows, use Git Bash or adapt to PowerShell. The envdesk binary itself runs on Windows; CI runners often use Linux and the same three checks as below.

**Checks (same order as CI):**

1. **Plaintext guard** — `bash scripts/check-env-conventions.sh` (no decryption, no age key).
2. **Schema** — `envdesk lint` (needs `sops` + age identity to read encrypted files).
3. **Drift** — `envdesk check-sync --strict-required-only --json` (same prerequisites as lint).

Full job examples for **GitHub Actions** and **GitLab CI** (`sops`, `age`, `SOPS_AGE_KEY_FILE`) are in [docs/ci-integration.md](./ci-integration.md).

**Install the sample hooks locally:**

```bash
cp hooks/pre-commit.sample .git/hooks/pre-commit
cp hooks/pre-push.sample .git/hooks/pre-push
chmod +x .git/hooks/pre-commit .git/hooks/pre-push
```

## Onboarding

For a new teammate:

1. Add the teammate's age recipient with `envdesk member add <recipient>`.
2. Preview the affected files with `envdesk member add <recipient> --dry-run`.
3. Re-encrypt affected files with `--rekey` or `envdesk rekey`.
4. Share the repository workflow for `envdesk export` and `envdesk doctor`.
5. Point them at `README.md`, [docs/getting-started.md](./getting-started.md) if they are new to SOPS/age, and this guide.

## Common tasks

Use these commands for the most common follow-up tasks after initial setup:

- compare two environments with `envdesk diff <service> <from-env> <to-env> --value-mode public`
- align missing keys with `envdesk sync-keys <service> <source-env> --to <target-env> --placeholders`
- preview recipient changes with `envdesk member add <recipient> --dry-run`
- re-encrypt files after recipient changes with `envdesk rekey --service <service>`
- generate a shareable example file with `envdesk example generate --service <service> --out env/<service>/.env.example`
- check local setup and repository safety with `envdesk doctor`
