#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
GO_OUT="$ROOT/go"
CS_OUT="$ROOT/cs"

GO_CMD="${GO:-go}"

# --- protoc ---
if [ -f /tmp/protoc/bin/protoc ]; then
  PROTOC=/tmp/protoc/bin/protoc
  PB_INCLUDE=/tmp/protoc/include
else
  echo "protoc not found — downloading..."
  wget -q https://github.com/protocolbuffers/protobuf/releases/download/v28.2/protoc-28.2-linux-x86_64.zip -O /tmp/protoc.zip
  unzip -q -o /tmp/protoc.zip -d /tmp/protoc
  PROTOC=/tmp/protoc/bin/protoc
  PB_INCLUDE=/tmp/protoc/include
fi

PROTO_INCLUDES="-I$ROOT -I$PB_INCLUDE"

# --- Go protoc plugins ---
if ! command -v protoc-gen-go &>/dev/null; then
  echo "protoc-gen-go not found — installing..."
  $GO_CMD install google.golang.org/protobuf/cmd/protoc-gen-go@latest
fi

if ! command -v protoc-gen-go-grpc &>/dev/null; then
  echo "protoc-gen-go-grpc not found — installing..."
  $GO_CMD install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
fi

echo "=== Generating Go types ==="
rm -rf "$GO_OUT"/commonpb "$GO_OUT"/marketdatapb "$GO_OUT"/tradingpb "$GO_OUT"/intelligencepb

for f in common market_data trading intelligence; do
  echo "  $f.proto → go/ (messages)"
  $PROTOC --go_out="$GO_OUT" --go_opt=module=trading-system/contracts/go \
    $PROTO_INCLUDES "$ROOT/$f.proto"
done

echo ""
echo "=== Generating Go gRPC stubs ==="
for f in common market_data trading intelligence; do
  echo "  $f.proto → go/ (gRPC)"
  $PROTOC --go-grpc_out="$GO_OUT" --go-grpc_opt=module=trading-system/contracts/go \
    $PROTO_INCLUDES "$ROOT/$f.proto"
done

echo ""
echo "=== Generating C# types ==="
mkdir -p "$CS_OUT"

for f in common market_data trading intelligence; do
  echo "  $f.proto → cs/"
  $PROTOC --csharp_out="$CS_OUT" $PROTO_INCLUDES "$ROOT/$f.proto"
done

echo ""
echo "=== Done ==="
$PROTOC --version
