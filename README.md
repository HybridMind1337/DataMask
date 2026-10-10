# DataMask

<p align="center">
  <a href="https://github.com/HybridMind1337/DataMask/actions"><img src="https://img.shields.io/github/actions/workflow/status/HybridMind1337/DataMask/ci.yml?branch=main&label=CI&style=flat-square" alt="CI Status"></a>
  <a href="https://github.com/HybridMind1337/DataMask/releases"><img src="https://img.shields.io/github/v/release/HybridMind1337/DataMask?style=flat-square&color=007ec6" alt="Latest Release"></a>
  <a href="https://golang.org"><img src="https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat-square&logo=go" alt="Go Version"></a>
  <a href="./LICENSE"><img src="https://img.shields.io/badge/License-MIT-green?style=flat-square" alt="License: MIT"></a>
</p>

> **Production-like data. Without production identities.**

DataMask creates privacy-safe copies of production databases for development, automated testing, and staging environments while preserving relational integrity, schema structure, and realistic data characteristics.

```mermaid
flowchart TD
    subgraph Source ["Source Database (Protected Prod)"]
        S1[(PostgreSQL / MySQL / SQLite)]
    end

    subgraph DiscoveryEngine ["1. Inspection & Intelligence"]
        D1[Driver Read-Only Session]
        D2[Schema & Column Inspector]
        D3[Multi-Signal PII Detector]
        D4[Relational DAG & Cycle Resolver]
    end

    subgraph TransformationEngine ["2. Masking Core (Streaming)"]
        T1[Deterministic Key Deriver / HMAC-SHA256]
        T2[Keyset / Cursor Batch Reader]
        T3[Transformer Registry - 25+ Strategies]
        T4[JSON / JSONB Deep Scrubber]
        T5[ZeroMemory In-RAM Key Wiper]
    end

    subgraph SafeDestinations ["3. Output Destinations"]
        Tgt[(Target Dev / Staging DB)]
        Dump[Compressed SQL Stream .sql.gz]
    end

    subgraph AuditGate ["4. Verification & Audit"]
        V1[FK Integrity Validator]
        V2[Residual PII Leak Scanner]
        V3[Cryptographic SHA-256 Audit Report]
    end

    S1 -->|Enforced Read-Only| D1
    D1 --> D2
    D2 --> D3
    D2 --> D4
    D3 & D4 --> T2
    T1 -.->|Entropy Context| T3
    T2 --> T3
    T3 --> T4
    T4 -->|Bulk Copy / Batch Insert| Tgt
    T4 -->|SQL Insert Stream| Dump
    T4 -.->|Shutdown| T5
    Tgt --> V1 & V2
    V1 & V2 --> V3

    classDef source fill:#f9d5d5,stroke:#c0392b,stroke-width:2px,color:#900;
    classDef target fill:#d5f9d8,stroke:#27ae60,stroke-width:2px,color:#060;
    classDef engine fill:#e8f4fd,stroke:#2980b9,stroke-width:2px,color:#036;
    class S1 source;
    class Tgt,Dump target;
    class D1,D2,D3,D4,T1,T2,T3,T4,T5,V1,V2,V3 engine;
```

---

## Key Features

- **Multi-Engine Support:** Native driver adapters for **PostgreSQL**, **MySQL/MariaDB**, and **SQLite** (pure Go, CGO-free via `modernc.org/sqlite`).
- **Zero-Config Single-Command Run:** Mask directly from source to target without requiring a pre-existing config file (`datamask mask --source "..." --target "..." --auto`).
- **Interactive Onboarding Wizard (`datamask wizard`):** Step-by-step interactive CLI guide for first-time database connection, scanning, and safe copying.
- **Automated Configuration Generator (`datamask init`):** Introspects source schemas and writes an optimal, ready-to-use `datamask.yaml` with detected PII strategies.
- **Compressed Dump Streamer (`datamask dump`):** Direct SQL / `.sql.gz` streaming without requiring a secondary database.
- **Keyset (Seek) Pagination Streaming:** $O(1)$ fast indexed pagination on integer primary keys, avoiding $O(N^2)$ deep offset degradation.
- **In-Memory Key Zeroing:** Cryptographic secret keys are actively wiped from RAM upon execution finish (`ZeroMemory()`) to prevent dump leaks.
- **Deep JSON / JSONB Masking:** Recursively parses embedded JSON structures and sanitizes nested sensitive keys (`email`, `token`, `phone`, `password`, `name`).
- **Data Subsetting & Framework Presets:** Extract micro-datasets (`--subset-rows`, `--subset-percent`) with pre-configured rules for Laravel, Django, Rails, and WordPress.
- **CI/CD Quality Gate Fail-Safe (`datamask verify --ci`):** Strict verification mode exiting with error codes if residual PII or broken constraints are detected.
- **Shell Autocompletion:** Generates native autocompletion scripts for Bash, Zsh, Fish, and PowerShell (`datamask completion`).
- **Strict Read-Only Source Guarantee:** Enforces driver-level read-only transactions on source databases (`SET SESSION CHARACTERISTICS AS TRANSACTION READ ONLY`).
- **Production Guardrails:** Aborts immediately if the destination matches protected environment names (`prod`, `production`, `live`, etc.).
- **Deterministic Masking:** Keyed HMAC-SHA256 derivations ensure the same original value produces identical masked values across unrelated tables without exposing keys.
- **Relational Integrity Preservation:** Automatically discovers primary keys, foreign keys, and unique constraints. Computes topological DAG execution order and manages cyclic dependencies.
- **Collision Detection & Retries:** Guarantees unique constraints remain satisfied with automatic retry loops and internal source-to-destination mapping caches.
- **Zero Raw PII Leaks:** Error messages, stack traces, and structured logs scrub all values matching sensitive patterns before output.
- **Bounded Memory Streaming:** Streams large tables using server-side cursors in batches (default: 5,000 rows) and leverages bulk inserters (`pgx.CopyFrom` / chunked MySQL multi-inserts).
- **Post-Masking Verification:** Validates foreign key consistency and scans destination tables for residual PII or tokens.

---

## Installation

### One-Liner Installers (Recommended)

#### Linux & macOS
```bash
curl -sSL https://raw.githubusercontent.com/HybridMind1337/DataMask/main/scripts/install.sh | bash
```

#### Windows (PowerShell)
```powershell
iwr -useb https://raw.githubusercontent.com/HybridMind1337/DataMask/main/scripts/install.ps1 | iex
```

### From Source
```bash
go install github.com/HybridMind1337/DataMask/cmd/datamask@latest
```

### Via Docker
```bash
docker run --rm -v $(pwd)/datamask.yaml:/config/datamask.yaml hybridmind1337/datamask --help
```

---

## Quick Start

### 1. Initialize Configuration
Create `datamask.yaml` (see [`datamask.example.yaml`](./datamask.example.yaml) for full template):

```yaml
version: 1

source:
  driver: postgres
  host: ${DB_SOURCE_HOST:-localhost}
  port: 5432
  database: production_demo
  username: ${DB_SOURCE_USER:-postgres}
  password: ${DB_SOURCE_PASSWORD:-secret}

target:
  driver: postgres
  host: ${DB_TARGET_HOST:-localhost}
  port: 5432
  database: staging_safe
  username: ${DB_TARGET_USER:-postgres}
  password: ${DB_TARGET_PASSWORD:-secret}

settings:
  deterministic: true
  deterministic_key_env: DATAMASK_TRANSFORM_KEY
  preserve_primary_keys: true
  batch_size: 5000

tables:
  users:
    columns:
      id:
        strategy: preserve
      email:
        strategy: fake_email
        deterministic: true
      password:
        strategy: dev_auth
```

### 2. Scan Schema for Sensitive Data
```bash
datamask scan --config datamask.yaml
```

Output:
```text
DataMask
──────────────────────────────────────────
Scanning database...

Tables:             4
Columns:            21
Rows:               12

Potential sensitive fields:

CRITICAL users.password (strategy: dev_auth, confidence: 98%)
HIGH     users.email (strategy: fake_email, confidence: 95%)
HIGH     users.phone (strategy: fake_phone, confidence: 92%)
MEDIUM   users.first_name (strategy: fake_first_name, confidence: 85%)

Scan completed.
No source data was modified.
```

### 3. Review Masking Plan
```bash
datamask plan --config datamask.yaml
```

### 4. Execute Dry-Run
```bash
datamask mask --config datamask.yaml --dry-run
```

### 5. Execute Data Masking
```bash
export DATAMASK_TRANSFORM_KEY="your-high-entropy-secret-key"
datamask mask --config datamask.yaml
```

### 6. Or Stream Directly to Compressed SQL Dump
```bash
datamask dump \
  --source="postgres://postgres:secret@localhost:5432/production_demo" \
  --output=./safe_dev_dump.sql.gz \
  --preset=laravel \
  --subset-rows=5000
```

### 7. Verify Target Integrity & Privacy
```bash
datamask verify --config datamask.yaml
```

---

## Framework Presets

DataMask provides curated presets for standard web frameworks (`--preset`):
- `laravel`: Automatically handles `users.remember_token` (null), `users.password` (dev hash), `personal_access_tokens.token` (redact), and `failed_jobs.payload` (json_mask).
- `django`: Handles `auth_user` credentials, `django_session` scrubbing, and auth tokens.
- `rails`: Handles Devise `encrypted_password`, `reset_password_token`, and ActiveStorage identifiers.
- `wordpress`: Sanitizes `wp_users` (`user_pass`, `user_email`, `user_nicename`).

---

## Masking Strategies

| Strategy | Description | Sample Output |
| :--- | :--- | :--- |
| `preserve` | Keeps original value untouched | `481` |
| `null` | Sets column to SQL NULL | `NULL` |
| `redact` | Replaces with static indicator | `[REDACTED]` |
| `fake_email` | Reserved RFC domain (.test) email | `oliver.bennett@example.test` |
| `fake_name` | Realistic synthetic full name | `Oliver Bennett` |
| `fake_first_name` | Realistic first name | `Oliver` |
| `fake_last_name` | Realistic last name | `Bennett` |
| `fake_phone` | Non-routable dramatic phone | `+44 7700 900418` |
| `fake_address` | Synthetic street address | `42 High Street, Springfield` |
| `fake_postcode` | Format-preserving postal code | `TE14 2ST` |
| `fake_ipv4` | RFC 5737 documentation IP | `192.0.2.41` |
| `fake_ipv6` | RFC 3849 documentation IPv6 | `2001:db8::01a4` |
| `random_uuid` | Synthetic RFC 4122 v4 UUID | `a1b2c3d4-e5f6-47a8-b9c0-d1e2f3a4b5c6` |
| `pattern_scrub` | Scrubs emails/tokens from free text | `Contact [REDACTED_EMAIL]` |
| `dev_auth` | Known development password hash | `$2y$10$TKh8H1...` |

---

## Local Development & Synthetic Fixtures

Launch local PostgreSQL and MySQL test databases populated with synthetic e-commerce data:

```bash
docker compose up -d
```

Run test suite:
```bash
go test -v ./...
go vet ./...
```

---

## Security Model

1. **Source Isolation:** Source databases are treated as strictly read-only.
2. **Deterministic Entropy:** Derivations use HMAC-SHA256 with keys supplied via environment variables. Keys are never printed or saved in reports.
3. **No Raw Logging:** Logs and terminal outputs never echo raw column values.

---

## Legal & Compliance Disclaimer

DataMask is an automated data sanitisation and transformation tool. Using DataMask does not automatically guarantee compliance with GDPR, HIPAA, PCI-DSS, or any other data privacy regulations. Organisations remain responsible for auditing their transformed datasets and verifying their legal compliance posture.

---

## License

[MIT License](./LICENSE) - Copyright (c) 2026 HybridMindLabs
