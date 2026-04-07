# Contributing to envy

Thank you for your interest in contributing! Here's how to get started.

## Development Setup

```bash
git clone https://github.com/user/envy
cd envy
go mod download
go build .
./envy --help
```

## Running Tests

```bash
go test ./...
go test -race ./...       # race condition detection
go test -cover ./...      # coverage report
```

## Project Structure

```
envy/
├── main.go               # Entry point
├── cmd/                  # CLI commands (cobra)
│   ├── root.go           # Root command + logo
│   ├── init.go           # envy init
│   ├── secrets.go        # set, get, list, delete
│   ├── envops.go         # diff, push, pull, import, export
│   └── audit.go          # git history scanner
├── internal/
│   ├── crypto/           # AES-256-GCM encryption
│   └── env/              # Store management + helpers
└── .github/
    └── workflows/        # CI/CD
```

## Submitting Changes

1. Fork the repo
2. Create a branch: `git checkout -b feature/your-feature`
3. Make your changes
4. Add tests for new functionality
5. Run `go test ./...` and `go vet ./...`
6. Push and open a Pull Request

## Code Style

- Follow standard Go formatting (`gofmt`)
- Keep functions focused and small
- Add comments for exported functions
- Prefer explicit error handling

## Reporting Bugs

Open an issue with:
- Go version (`go version`)
- OS and architecture
- Steps to reproduce
- Expected vs actual behavior

## Feature Requests

Open an issue describing:
- The problem you're solving
- Your proposed solution
- Alternatives you considered
