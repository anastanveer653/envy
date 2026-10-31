<div align="center">

```
  ███████╗███╗   ██╗██╗   ██╗██╗   ██╗
  ██╔════╝████╗  ██║██║   ██║╚██╗ ██╔╝
  █████╗  ██╔██╗ ██║██║   ██║ ╚████╔╝
  ██╔══╝  ██║╚██╗██║╚██╗ ██╔╝  ╚██╔╝
  ███████╗██║ ╚████║ ╚████╔╝    ██║
  ╚══════╝╚═╝  ╚═══╝  ╚═══╝     ╚═╝
```

**Encrypted, multi-environment secrets for your projects. One binary, no cloud.**

[![CI](https://github.com/anastanveer653/envy/actions/workflows/ci.yml/badge.svg)](https://github.com/anastanveer653/envy/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/anastanveer653/envy?color=00b4d8)](https://github.com/anastanveer653/envy/releases)
[![Go](https://img.shields.io/github/go-mod/go-version/anastanveer653/envy?color=00b4d8)](go.mod)
[![License: MIT](https://img.shields.io/badge/license-MIT-00b4d8)](LICENSE)

</div>

envy replaces the pile of plaintext `.env`, `.env.staging` and `.env.production`
files in your project with one authenticated-encrypted store. It runs your app
with the right secrets injected, tells you when production is missing a key, and
scans your git history for credentials that already leaked.

```console
$ envy init
Create master password: ••••••••••••
✓ Created .envy/store.enc
✓ Environments: development, staging, production
✓ Added .envy/ to .gitignore

$ envy import .env.production -e production
✓ Imported .env.production into production: 14 added, 0 updated, 0 skipped

$ envy run -e production -- ./server        # secrets go to the process, never to disk

$ envy diff staging production --exit-code
- FEATURE_FLAGS_URL only in staging
+ SENTRY_DSN only in production

1 key only in staging, 1 key only in production
12 shared keys with different values

$ envy audit
● AWS access key ID  [aws-access-key-id]
    config/deploy.yml:12
    commit 3f9c2a1b7d4e
    match  AKIA********
✗ 1 issue found
```

## Features

- **Strong encryption at rest.** AES-256-GCM with a key derived by Argon2id
  (RFC 9106). The file header is authenticated too, so the KDF cost can't be
  quietly downgraded. See the [threat model](SECURITY.md).
- **Environments built in.** Keep `development`, `staging`, `production`, or any
  others in one store. Clone one with `envy envs add qa --from staging`.
- **`envy run`.** Inject secrets into any command's environment instead of
  writing a `.env` file.
- **`envy diff`.** Catch drift between environments before you deploy. With
  `--exit-code` it fails CI.
- **`envy audit`.** Scan every commit on every branch for AWS, GitHub, GitLab,
  Stripe, OpenAI, Anthropic, Slack, Google, npm, Twilio and SendGrid tokens,
  private keys and connection strings, plus `.env` files that were committed or
  aren't ignored. Matches are redacted, and `--staged` makes it fast enough for a
  pre-commit hook.
- **Script-friendly.** `envy get` prints the bare value, and `--json` output and
  meaningful exit codes are available. `ENVY_PASSWORD` covers non-interactive use.
- **Safe defaults.** Hidden prompts keep values out of shell history, writes are
  atomic, names are validated, `export` won't overwrite a file without
  `--force`, and `run` strips `ENVY_PASSWORD` from the child process.
- **Single static binary** for Linux, macOS and Windows on amd64 and arm64.

## Install

**macOS / Linux** (verifies the SHA-256 checksum before installing):

```bash
curl -fsSL https://raw.githubusercontent.com/anastanveer653/envy/main/install.sh | sh
```

**With Go:**

```bash
go install github.com/anastanveer653/envy@latest
```

**Windows:** download `envy_<version>_windows_amd64.zip` from
[Releases](https://github.com/anastanveer653/envy/releases), extract `envy.exe`
and add it to your `PATH`.

## Quick start

```bash
cd my-project
envy init                                  # create the store and set a master password

envy set DATABASE_URL                      # prompts for the value (hidden)
envy set STRIPE_KEY -e production --note "live key, rotate quarterly"
envy set TLS_KEY -e production < server.key

envy list -e production                    # names and notes only, never values
envy get DATABASE_URL                      # prints the value
envy run -- npm run dev                    # start your app with secrets injected

envy export --template -o .env.example --force   # keep the example file honest
```

The environment comes from `--env/-e`, then `$ENVY_ENV`, then `development`.

## Commands

| Command | What it does |
|---|---|
| `envy init [--envs a,b,c]` | Create `.envy/store.enc` and add `.envy/` to `.gitignore` |
| `envy set KEY [VALUE] [-n note]` | Add or update a secret. Without VALUE it reads a hidden prompt or stdin |
| `envy get KEY` | Print a secret's value |
| `envy list [--json]` | List key names, notes and update times |
| `envy delete KEY...` | Delete secrets |
| `envy envs` / `envs add NAME [--from ENV]` / `envs remove NAME` | Manage environments |
| `envy diff A B [--exit-code]` | Compare the key sets of two environments |
| `envy run -- CMD [ARGS]` | Run a command with secrets in its environment |
| `envy import [FILE] [--skip-existing]` | Import a `.env` file (default `.env`) |
| `envy export [-o FILE] [--template] [--force]` | Write `.env` format to stdout or a file |
| `envy passwd` | Change the master password |
| `envy audit [PATH] [--staged] [--json]` | Scan a repository for leaked secrets |
| `envy completion bash\|zsh\|fish\|powershell` | Generate shell completions |

Run `envy <command> --help` for every flag and more examples.

## Using envy in CI

```yaml
# .github/workflows/deploy.yml
- run: go install github.com/anastanveer653/envy@latest

- name: Fail if production is missing keys that staging has
  run: envy diff staging production --exit-code
  env:
    ENVY_PASSWORD: ${{ secrets.ENVY_PASSWORD }}

- name: Block leaked credentials
  run: envy audit
```

### Pre-commit hook

With [pre-commit](https://pre-commit.com):

```yaml
repos:
  - repo: https://github.com/anastanveer653/envy
    rev: v1.0.0
    hooks:
      - id: envy-audit
```

Or with a plain git hook:

```bash
printf '#!/bin/sh\nexec envy audit --staged\n' > .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
```

## How it works

```
master password ──Argon2id (64 MiB, t=3, p=4, random salt)──▶ 256-bit key
                                                                  │
{format, kdf params, salt} ──── authenticated as GCM AAD ───┐     ▼
all environments as JSON ───────────────────────────────────▶ AES-256-GCM ──▶ .envy/store.enc
```

- The whole store is sealed as one unit, so the file reveals neither key names
  nor how many secrets it holds.
- Each save uses a fresh random nonce and writes atomically: temp file, fsync,
  then rename.
- No password hash is stored. A wrong password simply fails GCM authentication.
- The format is versioned. Stores from envy 0.x (PBKDF2) are upgraded the first
  time you open them.

Read [SECURITY.md](SECURITY.md) for the threat model: what envy protects against
and what it doesn't.

## When to use something else

envy is built for individual developers and small teams who want strong local
encryption without running a service. Other tools fit better when:

- **Your team already uses a cloud KMS or age keys.** [SOPS](https://github.com/getsops/sops)
  encrypts values with per-recipient keys, which avoids a shared password.
- **You want public-key encrypted `.env` files in git.** [dotenvx](https://dotenvx.com)
  lets CI decrypt with a private key.
- **You need access control, rotation and audit logs across an organisation.**
  Use a secrets manager such as HashiCorp Vault, Infisical, Doppler or 1Password.

## Project layout

```
cmd/              CLI commands (cobra), with end-to-end tests
internal/crypto   Argon2id + AES-256-GCM, legacy decryption
internal/store    encrypted store format, atomic writes, validation
internal/dotenv   .env parsing and round-trip-safe quoting
internal/audit    secret detection rules and git history scanning
internal/ui       colors, prompts, TTY handling
```

## Contributing

Issues and pull requests are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for
setup, tests and how to add an audit rule. Report security issues privately as
described in [SECURITY.md](SECURITY.md).

## License

[MIT](LICENSE) © [Anas Tanveer](https://github.com/anastanveer653)
