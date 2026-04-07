<div align="center">

```
  ███████╗███╗   ██╗██╗   ██╗██╗   ██╗
  ██╔════╝████╗  ██║██║   ██║╚██╗ ██╔╝
  █████╗  ██╔██╗ ██║██║   ██║ ╚████╔╝ 
  ██╔══╝  ██║╚██╗██║╚██╗ ██╔╝  ╚██╔╝  
  ███████╗██║ ╚████║ ╚████╔╝    ██║   
  ╚══════╝╚═╝  ╚═══╝  ╚═══╝     ╚═╝   
```

**Smart `.env` manager for developers**

[![Release](https://img.shields.io/github/v/release/anastanveer653/envy?style=flat-square&color=00b4d8)](https://github.com/anastanveer653/envy/releases)
[![Go Version](https://img.shields.io/badge/go-1.21+-00b4d8?style=flat-square)](https://go.dev)
[![License](https://img.shields.io/badge/license-MIT-00b4d8?style=flat-square)](LICENSE)
[![Stars](https://img.shields.io/github/stars/anastanveer653/envy?style=flat-square&color=00b4d8)](https://github.com/anastanveer653/envy/stargazers)

</div>

---

## Why envy?

Every developer has been here:

- 😅 Accidentally committed `.env` to GitHub
- 😰 Sent API keys over Slack "just this once"
- 😤 Lost track of which secrets are in dev vs production
- 🤦 `.env.example` is 3 months out of date

**envy** fixes all of this. One CLI tool. AES-256 encryption. Zero config.

---

## Features

- 🔒 **AES-256-GCM encryption** — military-grade, password-derived keys
- 🌍 **Multi-environment** — dev, staging, prod, any custom environment
- 🔍 **Git audit** — scan your entire git history for leaked secrets
- 📋 **Smart diff** — compare environments to spot missing/mismatched keys
- 📥 **Import/Export** — seamlessly convert from/to plain `.env` files
- 🚀 **Single binary** — no runtime, no Docker, no dependencies
- ⚡ **Fast** — written in Go, instant startup

---

## Install

**macOS / Linux (one line):**
```bash
curl -fsSL https://raw.githubusercontent.com/anastanveer653/envy/main/install.sh | bash
```

**Homebrew:**
```bash
brew install user/tap/envy
```

**Go:**
```bash
go install github.com/anastanveer653/envy@latest
```

**Windows:** Download from [releases page](https://github.com/anastanveer653/envy/releases)

---

## Quick Start

```bash
# 1. Initialize in your project
cd my-project
envy init

# 2. Add your secrets
envy set DATABASE_URL postgres://localhost/mydb
envy set API_KEY sk-abc123 --env production

# 3. List secrets (values safely masked)
envy list
envy list --env production

# 4. Get a secret
envy get DATABASE_URL

# 5. Export to .env file when needed
envy export --env production --output .env
```

---

## Commands

| Command | Description |
|---------|-------------|
| `envy init` | Initialize envy in your project |
| `envy set KEY value` | Store an encrypted secret |
| `envy get KEY` | Retrieve a secret value |
| `envy list` | List all keys (values masked) |
| `envy delete KEY` | Delete a secret |
| `envy diff dev prod` | Compare two environments |
| `envy push <env>` | Export secrets to `.env.<env>` file |
| `envy pull <env>` | Import from `.env.<env>` file |
| `envy import` | Import from existing `.env` file |
| `envy export` | Export to plain `.env` file |
| `envy audit` | Scan git history for leaked secrets |

---

## How It Works

```
Your Secret → PBKDF2 Key Derivation → AES-256-GCM Encryption → .envy/store.enc
                    ↑
              Master Password
              (never stored)
```

1. Your master password is never stored — only a hash for verification
2. Each secret is encrypted using a unique salt + PBKDF2 key derivation
3. AES-256-GCM provides both encryption and authentication
4. The store file is safe to commit to version control (optional)

---

## vs. alternatives

| Feature | envy | dotenv | direnv | 1Password CLI |
|---------|------|--------|--------|---------------|
| Encryption | ✅ AES-256 | ❌ | ❌ | ✅ |
| Multi-environment | ✅ | ❌ | ✅ | ✅ |
| Git audit | ✅ | ❌ | ❌ | ❌ |
| Env diff | ✅ | ❌ | ❌ | ❌ |
| Single binary | ✅ | ❌ | ✅ | ✅ |
| Free & open source | ✅ | ✅ | ✅ | ❌ |
| No cloud required | ✅ | ✅ | ✅ | ❌ |

---

## Security

- **AES-256-GCM** — authenticated encryption, detects tampering
- **PBKDF2** — 100,000 iterations, makes brute force infeasible
- **Unique salt per store** — prevents rainbow table attacks
- **Zero network requests** — everything stays on your machine
- **File permissions** — store written as `0600` (owner read/write only)

To report a security vulnerability, please email security@example.com (do not open a public issue).

---

## Contributing

Contributions are welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md) first.

```bash
git clone https://github.com/anastanveer653/envy
cd envy
go mod download
go build .
./envy --help
```

---

## License

MIT © [Anas Tanveer](https://github.com/anastanveer653)

---

<div align="center">
  <sub>If envy saves you from a secret leak, consider giving it a ⭐</sub>
</div>
