# Contributing to DataMask

Thank you for contributing to DataMask!

## Development Setup

Requirements:
- Go 1.22+
- Docker & Docker Compose (for integration tests)

### Testing
Run the test suite locally:
```bash
go test -v ./...
go vet ./...
```

### Pull Request Guidelines
1. Ensure all tests pass.
2. Maintain zero raw PII logging rules. Errors and logs must never echo raw sensitive data.
3. Keep database driver operations strictly read-only against source databases.
