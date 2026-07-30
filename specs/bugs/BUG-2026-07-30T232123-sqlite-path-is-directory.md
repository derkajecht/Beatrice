# BUG-2026-07-30T232123: SQLite path is created as a directory

## Problem

Starting server with default database settings exits with:

```
Could not set up database: failed to attach database: failed to ping database: unable to open database file (14)
```

Expected: server creates parent directory, opens configured SQLite database file, then creates schema.

Reproduce:

```bash
go run ./cmd/server
```

Security impact: LOW. No security exploit path identified.

## Root Cause Analysis

### Reproduce

Default server command reliably exits during database ping. Existing database artifact is a directory named `beatrice.db`, not a database file.

### Isolate

Database initializer constructs full database-file path, then creates that full path with directory creation. Connection helper ignores initialized path and instead opens default directory path.

### Hypothesize

1. SQLite cannot open directory as database file. Falsification: inspect created artifact and run server. Confirmed: artifact is directory; server reports SQLite error 14.
2. Configured database flags reach connection helper. Falsification: trace initializer and helper inputs. Rejected: helper constructs fresh defaults and receives no configured path.

### Verify

`go run ./cmd/server` produced `unable to open database file (14)` while `beatrice/beatrice.db` was a directory. Root cause confirmed: code creates database filename as directory, then opens different directory path.

Contributing factor: later database operations also create independent connections from defaults, so non-default database flags cannot work consistently.

Risk level: Medium. Default startup fails; custom database configuration silently targets wrong database.

## TDD Fix Plan

1. **RED**: Write test calling database initialization with temporary parent directory and filename; assert returned path is regular file and schema is usable.
   **GREEN**: Create only parent directory; open exact joined filename.
   **verify**: `go test ./internal/server`

2. **RED**: Write test using non-default temporary database path; assert initialization and subsequent user storage use same database.
   **GREEN**: Pass configured database path through connection and storage operations; remove fresh default configuration reads.
   **verify**: `go test ./internal/server`

**REFACTOR**: Close short-lived storage connections or reuse server-owned connection; keep one path construction point.

## Acceptance Criteria

- [ ] Default server startup creates `beatrice/beatrice.db` as SQLite file.
- [ ] Database schema exists after initialization.
- [ ] Custom database name and location are honored by all database operations.
- [ ] All new tests pass.
- [ ] Existing tests still pass.

## Resolution

<!-- filled in by validate-fix -->