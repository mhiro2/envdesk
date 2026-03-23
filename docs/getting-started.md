# Getting started

This guide is for teams that are new to **SOPS**, **age**, and **envdesk**. It walks from installing tools to an encrypted, reviewable env layout in Git.

If you already know SOPS and age, skim the concepts section and jump to [Scaffold with envdesk](#scaffold-with-envdesk).

## What you are building

You will keep environment variables in Git as **SOPS-encrypted** files. **age** provides the encryption keys. **envdesk** adds schema-aware editing, lint, drift checks, and team workflows on top of that stack.

## Concepts

### SOPS

[SOPS](https://github.com/getsops/sops) encrypts files in place so ciphertext can live in a repository. The `sops` CLI encrypts, decrypts, and re-encrypts files according to rules in `.sops.yaml`.

### age

[age](https://github.com/FiloSottile/age) is a simple modern file encryption format. SOPS uses age recipients (public keys) to decide who can decrypt. Each teammate (and each automation context, such as CI) needs an age **identity** (private key material) that matches a recipient listed in `.sops.yaml`.

### Recipients and identities

- **Recipient**: an age public key string such as `age1...`. It is safe to share and commit in `.sops.yaml`.
- **Identity**: the corresponding private key. Treat it like a password: store in a password manager, OS keychain, or CI secret—never commit it.

### envdesk

`envdesk` does not replace SOPS; it calls `sops` for crypto and focuses on env-specific workflows (edit, export, lint, sync, member/rekey). See [ARCHITECTURE.md](../ARCHITECTURE.md) for the data flow.

## Install tools

Install all of the following on your machine:

1. **age** — key generation and (optionally) encryption helpers.
2. **sops** — encrypt and decrypt env files.
3. **envdesk** — CLI for this repository layout.

Installation options vary by OS. Official sources:

- age: [FiloSottile/age releases](https://github.com/FiloSottile/age/releases)
- sops: [getsops/sops releases](https://github.com/getsops/sops/releases)
- envdesk: [README.md](../README.md#-install)

Verify:

```bash
age --version
sops --version
envdesk --version
```

## Create an age key pair

Generate a key file for your user (one-time per machine or per role):

```bash
age-keygen -o ~/.config/sops/age/keys.txt
```

The command prints your **public key** (recipient). Copy that line; you will pass it to `envdesk init` and share it with the team when they add you to the repo.

Keep `keys.txt` private. On Unix-like systems, restrict permissions:

```bash
chmod 600 ~/.config/sops/age/keys.txt
```

## Point SOPS at your age identity

SOPS discovers age keys in several ways. The most explicit option is `SOPS_AGE_KEY_FILE`:

```bash
export SOPS_AGE_KEY_FILE="$HOME/.config/sops/age/keys.txt"
```

Add that line to your shell profile if you want it for every session.

Alternatively, SOPS can read `SOPS_AGE_KEY` (inline key material). Prefer a file when possible to avoid keys in shell history.

## Scaffold with envdesk

Run `init` inside a Git repository (create one first if needed: `git init`).

Minimal layout with SOPS rules:

```bash
envdesk init --services api --envs dev --sops
```

To create **encrypted** env files immediately, pass your public recipient (the `age1...` line):

```bash
envdesk init --services api --envs dev --sops --encrypt --age age1yourpublickeyhere
```

You can repeat `--age` for multiple recipients (for example each teammate and a CI bot key).

What you get:

- `envdesk.yaml` — service and path mapping
- `.sops.yaml` — which paths SOPS encrypts and which age recipients apply
- `env/<service>/<env>.env` — env file (plaintext or encrypted depending on flags)
- `env.schema/<service>.yaml` — schema stub

## Day-to-day commands

Edit one environment (decrypt → editor → validate → re-encrypt):

```bash
envdesk edit api dev
```

Export plaintext only when you need it locally (short-lived files such as `.env.local`):

```bash
envdesk export api dev --out .env.local
```

Validate against schema and check cross-environment key drift:

```bash
envdesk lint --service api
envdesk check-sync --strict-required-only
```

Repository safety and tool readiness:

```bash
envdesk doctor
```

## Git hygiene

- Commit **encrypted** `env/**/*.env` and `.sops.yaml`. Do not commit age private keys or long-lived plaintext env files.
- Ignore local plaintext outputs. For example, in `.gitignore`:

  ```gitignore
  .env.local
  .env.*.local
  ```

- Before pushing, run `envdesk doctor` and the checks suggested in [docs/ci-integration.md](./ci-integration.md) and [docs/guide.md](./guide.md).

## Add a teammate

1. They generate an age key pair and send their **public** recipient to the team.
2. A maintainer runs `envdesk member add <recipient>` (use `--dry-run` first to preview).
3. Re-encrypt affected files with `envdesk rekey` so the new recipient can decrypt.

Details: [Onboarding](./guide.md#onboarding) in the guide.

## Where to go next

- [docs/guide.md](./guide.md) — full workflows, commands, hooks, and operational detail
- [docs/ci-integration.md](./ci-integration.md) — GitHub Actions and GitLab CI
- [README.md](../README.md) — install, quick start, and comparison with other tools
