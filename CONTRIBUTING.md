# Contributing to PipeRelay

Thanks for your interest in contributing!

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/<your-username>/PipeRelay.git`
3. Create a branch: `git checkout -b my-feature`
4. Make your changes
5. Run tests: `make test`
6. Build: `make build`
7. Commit and push
8. Open a pull request

## Development

```bash
# Run in development mode
make dev

# Build binary
make build

# Run tests
make test
```

## Requirements

- Go 1.22+
- CGO enabled (for SQLite)

## Guidelines

- Keep PRs focused — one feature or fix per PR
- Add tests for new functionality
- Follow existing code style
- Update documentation if behavior changes

## Reporting Issues

Open an issue on GitHub with:
- What you expected
- What happened
- Steps to reproduce
- Go version and OS
