# Architecture

## Overview

`envdesk` is organized as a thin CLI layer over a small set of domain packages.

- `cmd/envdesk` wires the executable.
- `internal/cli` handles Cobra commands, flags, and user-facing output.
- `internal/app` contains the command workflows and business rules.
- `internal/config` loads and validates `envdesk.yaml`.
- `internal/schema` loads and validates schema YAML.
- `internal/envfile` parses and serializes dotenv-style files.
- `internal/crypto` wraps the `sops` CLI.
- `internal/atomicwrite` centralizes replacement writes for env files and `.sops.yaml`.

The design is intentionally local-first. `envdesk` reads repository files, validates them, and invokes `sops` for encryption and decryption. It does not implement its own cryptography or depend on a remote secrets service.

## Data flow

1. The CLI resolves the repository config path.
2. `internal/config` loads the service layout and file mappings.
3. Command handlers call into `internal/app`.
4. `internal/app` loads env files, schemas, and SOPS configuration as needed.
5. `internal/crypto` shells out to `sops` for decrypt, encrypt, and rekey operations.
6. `edit` launches the configured editor through the platform shell (`/bin/sh` on Unix-like systems, `cmd.exe` on Windows).

This keeps command implementations small and makes the command behavior easy to test with fakes.

## Security model

`envdesk` follows a narrow security boundary:

- decrypted content exists only in explicit export and edit flows
- the repository copy remains encrypted
- `doctor` checks for plaintext env files, weak permissions, missing ignore rules, and tool availability
- `check-env-conventions.sh` is a lightweight repository check for tracked env files that do not look encrypted

The tool assumes the team uses age recipients through `.sops.yaml` and manages recipient changes with `member` and `rekey`.

## Command responsibilities

- `init` scaffolds repository layout and config files, and can encrypt the initial env files when recipients are provided.
- `edit` decrypts, opens, validates, and re-encrypts a single env file.
- `export` writes decrypted content for local use.
- `diff` compares structure between two environments and supports secret-safe detail modes.
- `lint` validates env files against schema.
- `check-sync` reports schema-aware key drift across environments.
- `status` aggregates per-environment lint, drift, and file update signals into a dashboard view.
- `sync-keys` normalizes target key sets from a source environment and can generate schema-aware placeholders.
- `doctor` validates the repository, local crypto prerequisites, and detected repository mode.
- `example generate` produces non-secret example env files with schema comments.
- `member add/remove` manages `.sops.yaml` recipients and supports dry-run previews.
- `rekey` re-encrypts selected env files with the current recipient set and surfaces partial failures.

## Release flow

Tagged releases are expected to be built from GitHub Actions. The release workflow embeds version metadata into the binary so `envdesk --version` can report the packaged build identity, and it ships macOS, Linux, and Windows artifacts.
