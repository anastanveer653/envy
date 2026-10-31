# Security Policy

## Reporting a vulnerability

Please **do not** open a public issue for security problems.

Report privately through GitHub's
[private vulnerability reporting](https://github.com/anastanveer653/envy/security/advisories/new).
You should get an acknowledgement within 72 hours. Once a fix is released,
you will be credited in the advisory unless you prefer otherwise.

## Supported versions

Only the latest release receives security fixes.

## Threat model

envy protects secrets **at rest** in a project directory.

| Threat | Protected? | How |
|---|---|---|
| Someone reads `.envy/store.enc` (stolen laptop backup, leaked repo, shared drive) | Yes | AES-256-GCM, key derived from the master password with Argon2id |
| Someone edits the store file, including its header (e.g. to weaken the KDF) | Yes | GCM authenticates the ciphertext **and** the header (associated data); KDF parameters below safe minimums are rejected |
| Crash or power loss during a write | Yes | Writes go to a temp file that is fsynced and atomically renamed |
| Secret leaks into shell history | Yes, if you use it | `envy set KEY` reads the value from a hidden prompt or stdin |
| Secret written to disk to run an app | Yes, if you use it | `envy run` injects secrets into the child's environment only |
| Weak master password | **No** | An attacker with the file can guess offline. Argon2id (64 MiB, t=3) makes each guess expensive, but cannot save a guessable password. Use a long passphrase. |
| Malware or another user running as you | **No** | Anything that can read your process memory, keystrokes or environment can read your secrets. |
| Secrets inside a running process | **No** | Child processes started with `envy run` see secrets as ordinary environment variables. |

## Cryptographic design

```
password ──Argon2id(salt, t=3, m=64 MiB, p=4)──▶ 256-bit key
                                                     │
header = {format, kdf params, salt}  ──(AAD)──┐      ▼
payload (JSON of all environments) ──────────▶ AES-256-GCM ──▶ nonce ‖ ciphertext ‖ tag
```

- **One file, one key.** The whole store is sealed as one unit, so the file
  does not reveal key names, environment names or how many secrets you have.
- **Fresh 96-bit random nonce** on every save. The salt changes when you run
  `envy passwd`.
- **No password hash is stored.** A wrong password is detected because GCM
  authentication fails.
- **Versioned format.** The header records the format version and KDF
  parameters, so they can be strengthened later without breaking old stores.
- **Legacy stores** (envy 0.x, PBKDF2-SHA256 with 100k iterations) can still be
  read, and are rewritten in the current format the first time they are opened.

## Should I commit `.envy/store.enc`?

By default `envy init` adds `.envy/` to `.gitignore`. The file is safe to
publish **only if** the master password is strong, because anyone who can
read the repository can attempt offline guessing indefinitely. For teams,
prefer sharing the password through a password manager and keeping the store
out of version control, or use a KMS-backed tool such as SOPS.
