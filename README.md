# Trading Systems Lab — C# / Go execution boundaries

A personal engineering prototype combining a .NET trading engine, Go gateway/watchdog components and shared protobuf/gRPC contracts. The interesting work is order state, failure handling and the boundary between an engine and an independent emergency controller—not a claimed profitable strategy.

**Public snapshot:** private personal source `76374f1b4e8993cf1a8f7587f2b8a31eff3ea6b4`, sanitized on 2026-10-07. The original working repository/history remains private. Existing source namespaces are retained.

## What to inspect

| Concern | Source |
| --- | --- |
| Domain, persistence and order lifecycle | [Engine core](trading-node/engine-net/SMC.Engine.Core) and [tests](trading-node/engine-net/SMC.Engine.Tests) |
| Streaming/exchange boundary | [Go gateway](trading-node/gateway-go) — market feeds, read loops, queues and private-session supervision |
| Independent emergency path | [Watchdog](watchdog-go/pkg) — monitoring, emergency pipeline, mocked OKX and notification tests |
| Cross-language contracts | [Protobuf definitions](contracts) and generated C#/Go contracts |
| Deployment isolation | [Compose](deployment/docker-compose.yml), mTLS configuration and [local certificate generator](deployment/gen_certs.sh) |

## Verification

The .NET suite and Go watchdog tests were rerun for publication. See [verification record](docs/PUBLICATION-VERIFICATION.md) for exact results and boundaries.

Storage tests require **an explicitly selected disposable PostgreSQL database**. They no longer silently write to localhost. For example, start a throwaway PostgreSQL container on an unused port, then:

```bash
export TRADING_TEST_POSTGRES='Host=127.0.0.1;Port=55432;Database=trading_engine;Username=postgres;Password=postgres'
dotnet test trading-node/engine-net/SMC.Engine.slnx -c Release
cd watchdog-go && go test ./...
```

Do not point this setting at a working database. Tests create schema and write rows. Go uses the version specified in `watchdog-go/go.mod`.

## Publication boundaries

- Development certificates and private keys are omitted. Run `bash deployment/gen_certs.sh` locally if needed; generated material is ignored by Git.
- Exchange/Telegram credentials are absent. Example environment values are empty; signing tests use synthetic secrets.
- Passing tests establish local contracts and mocked emergency behavior. They do not establish live exchange acceptance, profitable trading, regulatory suitability, latency targets or production recovery.
- The selected .NET and watchdog suites do not certify every Go gateway/ingress path or the entire deployed system. Start with tests, not the live executables.
