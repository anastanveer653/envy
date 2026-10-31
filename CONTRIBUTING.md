# Contributing to envy

Thanks for helping out. Bug reports, docs fixes and new audit rules are all welcome.

## Development setup

You need Go (the version in `go.mod`) and git.

```bash
git clone https://github.com/anastanveer653/envy
cd envy
go build -o envy .
./envy --help
```

## Tests and checks

```bash
go test ./...                 # all tests
go test -race ./...           # with the race detector (needs cgo)
make cover                    # coverage report in coverage.html
golangci-lint run             # lint, as in CI
go run . audit                # envy scans its own history
```

CI runs all of these on Linux, macOS and Windows. Please make sure they pass
before opening a pull request.

Tests lower the Argon2id cost through `store.KDFParams` so the suite stays fast.
Never lower it in production code.

## Project layout

```
envy/
├── main.go              entry point; version is stamped by GoReleaser
├── cmd/                 cobra commands; one file per command group
│   └── cmd_test.go      end-to-end tests that drive the real command tree
└── internal/
    ├── crypto/          Argon2id + AES-256-GCM primitives, legacy decryption
    ├── store/           encrypted store format, atomic writes, validation
    ├── dotenv/          .env parsing and safe quoting
    ├── audit/           secret detection rules and git history scanning
    └── ui/              colors, prompts, TTY handling
```

Commands get their I/O from the `app` struct rather than from `os.Stdin` and
`os.Stdout` directly. That lets `cmd_test.go` run every command against
in-memory buffers.

## Adding an audit rule

1. Add a `Rule` to `DefaultRules` in `internal/audit/rules.go`. Prefer
   distinctive prefixes such as `ghp_` or `sk_live_` over generic patterns.
2. Add a positive and a negative case to `internal/audit/audit_test.go`. Build
   fixture secrets by concatenating strings (`"ghp" + "_" + "..."`) so that
   envy's own audit does not flag the test file.

## Commits and pull requests

- Use [Conventional Commits](https://www.conventionalcommits.org/):
  `feat:`, `fix:`, `docs:`, `test:`, `refactor:`, `ci:`, `chore:`.
- Keep each PR to one change, and add tests for any behaviour change.
- Update `CHANGELOG.md` under **Unreleased**.

## Releasing (maintainers)

```bash
git tag -a v1.2.3 -m "v1.2.3"
git push origin v1.2.3
```

The release workflow runs GoReleaser, which builds binaries for
Linux, macOS and Windows on amd64 and arm64 and publishes `checksums.txt`.
