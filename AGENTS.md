# Contributor Guidance for AI Agents

Keep changes focused and validate claims with repository evidence. Read
`README.md` and `CONTRIBUTING.md` for user-facing behavior and contributor
setup; do not duplicate those documents here.

## Evidence and Review

- Verify MySQL behavior and version availability with official MySQL
  documentation. Verify MariaDB-specific behavior with official MariaDB
  documentation; do not assume the products are equivalent.
- Verify metric semantics against official [Prometheus documentation](https://prometheus.io/docs/practices/naming/) and, when
  necessary, upstream `client_golang`, Go, or database-driver source.
- Support findings with concrete code, unit tests, integration tests, documentation, or reproduced
  behavior. Separate correctness problems from optional improvements.

## Compatibility Invariants

- Metric names, types, labels, help text, and meanings are public compatibility
  surfaces. Avoid changing them without explicit justification and migration
  consideration.
- Collector flags and configuration keys are also compatibility surfaces.
- Check documentation, recording rules, alerts, and dashboards when changing
  metrics or flags.
- Review every new label for boundedness and cardinality risk.
- Confirm the exact MySQL, MariaDB, and Percona versions that provide every
  queried table or column, and encode accurate version gates where needed.
- Document required database grants and supported database variants for new
  collectors.

## Collector Implementation

- Follow the nearest existing collector and test as the implementation pattern.
- Handle nullable and version-dependent columns deliberately.
- Keep per-row state inside or reset it at the start of each row iteration.
- Check query, scan, iteration, and close errors, including `rows.Err()`.
- Preserve context cancellation and exporter query-timeout behavior.
- Never log credentials, DSNs containing secrets, or configuration-file
  contents.

## Tests and Validation

- For new behavior, write or update a focused test before implementation when
  practical.
- Tests should assert metric names, types, labels, and values, not only that a
  scrape succeeds.
- Use `go test ./collector` for a focused collector check and `make test` for
  the broader Go test suite.
- Run the repository's default `make` checks before considering substantive Go
  changes complete.
- Use `TEST_MYSQL_DSN='root@tcp(127.0.0.1:3306)/' make test` when behavior
  depends on a real database. Test relevant MySQL and MariaDB versions when
  semantics differ; `sqlmock` alone is insufficient for those cases.
- For mixin changes, edit source files rather than generated output, then run
  `make -C mysqld-mixin lint build`.
- After generators or formatting commands, inspect `git diff` and ensure no
  unexplained generated changes remain.
