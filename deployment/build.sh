#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
GO="${GO:-go}"

echo "=== Regenerating protos ==="
PATH="$HOME/go/bin:/tmp/protoc/bin:$PATH" GO="$GO" "$ROOT/contracts/generate_protos.sh"

echo ""
echo "=== Building Go services ==="

for dir in "$ROOT/trading-node/gateway-go" "$ROOT/intelligence-node/osint-go" "$ROOT/watchdog-go"; do
    echo "  Building $(basename "$dir")..."
    (cd "$dir" && GOFLAGS=-mod=mod "$GO" build ./...)
done

echo ""
echo "=== Building .NET solution ==="

dotnet build "$ROOT/trading-node/engine-net/SMC.Engine.slnx"

echo ""
echo "=== All builds passed ==="
