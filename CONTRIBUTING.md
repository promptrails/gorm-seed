# Contributing

Thank you for considering contributing to gorm-seed!

## Getting Started

1. Fork the repository
2. Clone your fork
3. Run `make build` to verify the project builds
4. Run `make test` to verify tests pass

## Development Workflow

- Run `make check` before committing (runs fmt, vet, lint, test, build)
- Add tests for new functionality
- Update documentation (README, godoc) for API changes
- Keep the API backward compatible when possible

## Code Style

- Follow standard Go conventions
- Use `gofmt -s` and `goimports` for formatting
- Run `golangci-lint` for additional checks
- Document all exported symbols with godoc comments

## Commit Messages

Write clear, concise commit messages that explain what and why.

## Pull Requests

- Create a feature branch from `main`
- Open a PR with a clear description of the changes
- Ensure CI passes on your PR
- Request review from the maintainers

## Testing

- All new features should have corresponding tests
- Run `make coverage` to verify code coverage
- Run `make bench` for performance-sensitive changes

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
