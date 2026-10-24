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

## Adding an Authentication Provider

- Implement `auth.Provider` in `internal/auth/<provider>.go`: `Name` returns a
  stable `ProviderType`, `Configure` applies connection policy, and
  `Authenticate` supplies credentials before each physical connection.
- Keep provider options in its constructor. Add a provider-specific name to
  `auth.ProviderType` constants and select it in
  `config.MySqlConfig.AuthProvider`. Add INI fields only as needed; keep
  provider-specific branches out of runtimes and collectors.
- `Configure` receives a parsed, normalized `mysql.Config`. Validate user,
  password, transport, and TLS requirements here without retrieving credentials
  or making network calls. `auth.OpenConnector` owns `mysql.BeforeConnect`
  registration; providers must not install another hook. The MySQL driver owns
  the wire authentication handshake.
- `Authenticate` receives the driver's private configuration copy for every
  new connection, including later pool connections. Respect cancellation and
  pass its context to credential requests. Do not retain or share that copy.
- Support concurrent calls and keep provider options immutable. Avoid global
  registries and authentication state. Omitted or empty `auth` selects a nil
  provider and native MySQL authentication without provider hooks; no password
  provider is needed.
- Loaded config snapshots retain one provider per section across scrapes. Keep
  snapshots immutable after publication; reloads create fresh providers while
  in-flight requests retain their snapshot. Constructors must not retrieve
  credentials or make network calls. Runtimes borrow providers without closing
  them; define cleanup ownership before adding workers or managed dialers.
- Retain successfully loaded SDK configuration for the provider's lifetime and
  let the SDK cache and refresh credentials. Keep failed initialization retryable
  and let callers cancel while waiting for concurrent initialization. Do not
  cache a request context or a database authentication token.
- Document settings, credential discovery, database grants, supported services,
  and transport requirements in `README.md`. Verify behavior against official
  provider documentation and cover validation, cancellation, credential refresh,
  and concurrent connections with focused tests.

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
