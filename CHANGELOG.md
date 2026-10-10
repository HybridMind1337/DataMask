# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-10-10

### Added
- Core CLI commands: `scan`, `plan`, `mask`, `verify`, `inspect`, `rules list`, `version`.
- PostgreSQL and MySQL/MariaDB database driver support.
- Streamed table processing with server-side cursors and chunked bulk inserts.
- Sensitive PII detection engine with risk scoring and confidence levels.
- HMAC-backed deterministic masking preserving cross-table logical consistency.
- Collision detection with automated retry loop for unique column constraints.
- Topological dependency sorting respecting foreign keys and cyclic graph detection.
- Structured logger with automatic PII redacting and error scrubbing.
- Strict read-only enforcement on source database sessions.
- Protected environment guardrails stopping accidental writes to production targets.
- Comprehensive synthetic test fixtures and Docker Compose environment.
