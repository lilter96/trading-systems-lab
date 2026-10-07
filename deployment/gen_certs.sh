#!/usr/bin/env bash
set -euo pipefail

DIR="$(dirname "$0")/certs"
mkdir -p "$DIR"
DIR="$(cd "$DIR" && pwd)"

# CA
openssl req -x509 -newkey rsa:2048 -days 3650 -nodes \
  -keyout "$DIR/ca.key" -out "$DIR/ca.crt" \
  -subj "/CN=DevCA"

# Server (engine-net identity)
openssl req -newkey rsa:2048 -nodes \
  -keyout "$DIR/server.key" -out "$DIR/server.csr" \
  -subj "/CN=localhost"

openssl x509 -req -CA "$DIR/ca.crt" -CAkey "$DIR/ca.key" -CAcreateserial \
  -in "$DIR/server.csr" -out "$DIR/server.crt" -days 3650 \
  -extfile <(printf "subjectAltName=DNS:localhost,IP:127.0.0.1,DNS:engine-net")

# Gateway (gateway-go identity)
openssl req -newkey rsa:2048 -nodes \
  -keyout "$DIR/gateway.key" -out "$DIR/gateway.csr" \
  -subj "/CN=gateway-go"

openssl x509 -req -CA "$DIR/ca.crt" -CAkey "$DIR/ca.key" -CAcreateserial \
  -in "$DIR/gateway.csr" -out "$DIR/gateway.crt" -days 3650 \
  -extfile <(printf "subjectAltName=DNS:localhost,IP:127.0.0.1,DNS:gateway-go")

# Watchdog identity (watchdog-go)
openssl req -newkey rsa:2048 -nodes \
  -keyout "$DIR/watchdog.key" -out "$DIR/watchdog.csr" \
  -subj "/CN=watchdog-go"

openssl x509 -req -CA "$DIR/ca.crt" -CAkey "$DIR/ca.key" -CAcreateserial \
  -in "$DIR/watchdog.csr" -out "$DIR/watchdog.crt" -days 3650 \
  -extfile <(printf "subjectAltName=DNS:localhost,IP:127.0.0.1,DNS:watchdog-go")

rm -f "$DIR/"*.csr "$DIR/"*.srl

echo "=== Certificates created in $DIR ==="
ls -1 "$DIR/"
