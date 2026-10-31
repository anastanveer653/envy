# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/).

## [Unreleased]

## [1.0.0] - 2026-10-31

This is a ground-up hardening release. Existing stores are upgraded
automatically the first time they are opened.

### Security
- Derive keys with **Argon2id** (RFC 9106, 64 MiB, t=3) instead of
  PBKDF2-SHA256 with 100k iterations.
- New versioned store format (`envy/v2`). The header with the KDF parameters
  is authenticated as GCM associated data, so it cannot be tampered with or
  downgraded.
- Remove the unsalted SHA-256 password hash from the store. GCM
  authentication already detects a wrong password.
- Write the store atomically (temp file, fsync, rename) so a crash cannot
  corrupt it.
- `envy set KEY` with no value reads from a hidden prompt or stdin, which
  keeps secrets out of shell history.
- Validate key and environment names. Environment names were previously used
  unchecked in file names such as `.env.<name>`; names containing path
  separators are now rejected.
- `envy audit` redacts matches instead of printing the leaked secret again.

### Added
- `envy run -- <cmd>` injects secrets into a child process without writing them to disk.
- `envy passwd` changes the master password.
- `envy envs`, `envy envs add [--from ENV]`, `envy envs remove`.
- `envy export --template` generates an up-to-date `.env.example`.
- `envy diff --exit-code` and `envy audit` exit with status 1 on findings, for use in CI.
- `envy list --json` and `envy audit --json`.
- `ENVY_PASSWORD` and `ENVY_NEW_PASSWORD` for non-interactive use.
- `envy audit` detects GitHub fine-grained, GitLab, Slack, Google, OpenAI,
  Anthropic, npm and Twilio tokens, plus connection strings with passwords.
  Lines marked `envy:allow` are skipped.
- Shell completion: `envy completion bash|zsh|fish|powershell`.
- Cross-platform release binaries with checksums, and an installer that verifies them.
- CI on Linux, macOS and Windows with the race detector, golangci-lint,
  govulncheck, and envy auditing its own history.

### Changed
- **Breaking:** `envy get` prints only the value, so `$(envy get KEY)` works.
- **Breaking:** `push`/`pull` are replaced by `export -o` and `import FILE`.
- **Breaking:** `export` writes to stdout unless `-o` is given, and refuses to
  overwrite a file without `--force`.
- **Breaking:** the environment comes from `--env`, then `$ENVY_ENV`, then
  `development`. `NODE_ENV` and `APP_ENV` are no longer read, because a stray
  `NODE_ENV=production` silently redirected writes to production.
- Commands fail with a hint when an environment does not exist, instead of
  silently creating it on a typo.
- `envy list` no longer prints partial values.
- `envy init` no longer seeds a placeholder `APP_NAME` secret.
- Exported `.env` values are quoted so they survive a parse round trip.

### Fixed
- `envy audit` scans the full history of every branch, not only the last 50 commits.
- `envy audit` reported every `.env` file as not ignored, because it read the
  output of `git check-ignore -q`, which is always empty.
- `envy diff` output is sorted and stable between runs.
- `envy delete` reports an error for keys that do not exist.
- `envy import` saves once, instead of re-encrypting the store for every key.
- The test suite compiles. It had been referencing an undefined helper.

## [0.1.0] - 2026-04-07

- Initial release.

[Unreleased]: https://github.com/anastanveer653/envy/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/anastanveer653/envy/compare/v0.1.0...v1.0.0
[0.1.0]: https://github.com/anastanveer653/envy/releases/tag/v0.1.0
