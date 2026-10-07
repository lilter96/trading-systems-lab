# Publication verification — 2026-10-07

Source: sanitized personal snapshot `76374f1b4e8993cf1a8f7587f2b8a31eff3ea6b4`.

The .NET engine suite runs against an explicitly supplied disposable PostgreSQL 17 container. Go watchdog tests use local doubles. No live exchange orders, provider calls or Telegram messages were sent. Native tests create schema and data only in the throwaway database.

Exact results are recorded below after the publication run. Private certificate material and operational environment files are excluded; the public snapshot gets its own clean history. A documentation example exchange signing secret was replaced with `test-secret` because the tests only assert signature formatting.

## Results

- .NET engine: **151 passed, 0 failed, 1 skipped** (the optional memory benchmark). Includes all six PostgreSQL storage tests.
- Go watchdog: all tests passed across config, emergency, monitor, OKX-double and Telegram-double packages.
- The initial isolated-database run exposed missing audit immutability triggers in the fixture. The public fixture now loads the exact trigger fragment from deployment SQL; both EF and raw SQL mutation rejection tests passed.
- Npgsql and Microsoft.Extensions.Caching.Memory transitive dependencies are explicitly pinned to patched 8.x versions. Restore no longer reported those known advisories.

Patched-version references: [Npgsql advisory](https://github.com/advisories/GHSA-x9vc-6hfv-hg8c), [Microsoft caching advisory](https://github.com/advisories/GHSA-qj66-m88j-hmgj). These targeted updates are not a comprehensive dependency or security audit.
