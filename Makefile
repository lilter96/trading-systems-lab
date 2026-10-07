SHELL := /usr/bin/env bash
.SHELLFLAGS := -euo pipefail -c

.PHONY: all build test run-engine run-dashboard run-gateway run-watchdog halt check-db clean run-infra kill-gateway ensure-env ensure-gateway-env ensure-certs verify-mtls build-docker

ROOT := $(shell pwd)
CERTS := $(ROOT)/deployment/certs
COMPOSE := docker compose -p trading-system -f $(ROOT)/deployment/docker-compose.yml
WATCHDOG_ENV_FILE ?= $(ROOT)/watchdog-go/.env

all: build

build: build-engine build-gateway build-watchdog

ensure-certs:
	@if [ ! -f "$(CERTS)/ca.crt" ] || [ ! -f "$(CERTS)/gateway.crt" ] || [ ! -f "$(CERTS)/gateway.key" ] || [ ! -f "$(CERTS)/server.crt" ] || [ ! -f "$(CERTS)/server.key" ] || [ ! -f "$(CERTS)/watchdog.crt" ] || [ ! -f "$(CERTS)/watchdog.key" ]; then \
	  echo "[INFO] Generating missing development certificates in $(CERTS)"; \
	  bash $(ROOT)/deployment/gen_certs.sh; \
	fi

ensure-env:
	@if [ ! -f "$(WATCHDOG_ENV_FILE)" ]; then \
	  echo "[INFO] Creating $(WATCHDOG_ENV_FILE) from watchdog-go/.env.example"; \
	  cp $(ROOT)/watchdog-go/.env.example "$(WATCHDOG_ENV_FILE)"; \
	fi

ensure-gateway-env:
	@if [ ! -f "$(ROOT)/trading-node/gateway-go/.env" ]; then \
	  echo "[INFO] Creating trading-node/gateway-go/.env from .env.example"; \
	  cp $(ROOT)/trading-node/gateway-go/.env.example $(ROOT)/trading-node/gateway-go/.env; \
	else \
	  while IFS='=' read -r key rest; do \
	    case "$$key" in ''|\#*) continue;; esac; \
	    if ! grep -q "^$$key=" "$(ROOT)/trading-node/gateway-go/.env" 2>/dev/null; then \
	      echo "[INFO] Adding missing $$key to trading-node/gateway-go/.env"; \
	      echo "$$key=$$rest" >> "$(ROOT)/trading-node/gateway-go/.env"; \
	    fi; \
	  done < $(ROOT)/trading-node/gateway-go/.env.example; \
	fi

build-engine:
	cd $(ROOT)/trading-node/engine-net/SMC.Engine.Service && \
	  dotnet publish -c Release -o $(ROOT)/trading-node/engine-net/publish

build-gateway:
	cd $(ROOT)/trading-node/gateway-go && go build -buildvcs=false -o gateway .

build-watchdog:
	cd $(ROOT)/watchdog-go && go build -buildvcs=false -o watchdog ./cmd/watchdog

test:
	cd $(ROOT)/watchdog-go && go test ./...
	cd $(ROOT)/trading-node/engine-net/SMC.Engine.Service && dotnet build

run-infra:
	$(COMPOSE) up -d postgres clickhouse

run-engine: ensure-certs run-infra build-engine
	cd $(ROOT)/trading-node/engine-net/SMC.Engine.Service && dotnet run

kill-gateway:
	@lsof -ti :50050 -ti :50052 2>/dev/null | xargs kill 2>/dev/null || true

run-dashboard: ensure-certs ensure-gateway-env run-infra build-gateway kill-gateway
	@echo "==> Starting gateway (forwards real bars to Engine via gRPC)..."
	@cd $(ROOT)/trading-node/gateway-go && \
	  set -a && source .env && set +a && \
	  ./gateway & \
	GW_PID=$$!; \
	echo "  PID: $$GW_PID"; \
	echo "==> Starting engine (background)..."; \
	cd $(ROOT)/trading-node/engine-net/SMC.Engine.Service && dotnet run & \
	ENG_PID=$$!; \
	echo "  PID: $$ENG_PID"; \
	echo "  waiting for engine on :50051..."; \
	for i in $$(seq 1 30); do nc -z localhost 50051 2>/dev/null && break; sleep 1; done; \
	trap 'kill $$GW_PID $$ENG_PID 2>/dev/null; exit' INT TERM; \
	echo "==> Dashboard: https://localhost:50053/dashboard"; \
	wait $$ENG_PID; \
	EC=$$?; \
	echo "Stopping gateway..."; \
	kill $$GW_PID 2>/dev/null || true; \
	exit $$EC

run-gateway: ensure-certs ensure-gateway-env build-gateway kill-gateway
	@echo "==> Starting gateway..."
	cd $(ROOT)/trading-node/gateway-go && \
	  set -a && source .env && set +a && \
	  ./gateway

run-watchdog: ensure-certs ensure-env build-watchdog
	cd $(ROOT)/watchdog-go && \
	  set -a && source .env && set +a && \
	  ./watchdog

halt: ensure-certs
	@if command -v grpcurl &>/dev/null; then \
	  grpcurl -insecure -cert $(CERTS)/gateway.crt -key $(CERTS)/gateway.key -cacert $(CERTS)/ca.crt \
	    -d '{"reason":"manual_test","triggered_by":"makefile"}' \
	    localhost:50050 common.EmergencyService/HaltSystem; \
	else \
	  echo "grpcurl not found. Install it: https://github.com/fullstorydev/grpcurl/releases"; \
	  echo "Or use: go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest"; \
	  exit 1; \
	fi

check-db:
	psql -h localhost -U postgres -d trading_engine -c "SELECT * FROM system_state"

build-docker:
	$(COMPOSE) build gateway-go engine-net

verify-mtls:
	@echo "==> Verifying mTLS connectivity..."
	@if command -v grpcurl &>/dev/null; then \
	  echo "--- Engine gRPC (port 50051, mTLS required) ---"; \
	  grpcurl -insecure -cert $(CERTS)/gateway.crt -key $(CERTS)/gateway.key -cacert $(CERTS)/ca.crt \
	    localhost:50051 list 2>&1 || echo "FAIL: engine mTLS"; \
	  echo "--- Emergency (port 50050, mTLS required) ---"; \
	  grpcurl -insecure -cert $(CERTS)/gateway.crt -key $(CERTS)/gateway.key -cacert $(CERTS)/ca.crt \
	    localhost:50050 list 2>&1 || echo "FAIL: emergency mTLS"; \
	  echo "--- Dashboard (port 50053, no mTLS) ---"; \
	  curl -sk -o /dev/null -w "HTTP %{http_code}" https://localhost:50053/dashboard || echo "FAIL: dashboard"; \
	  echo ""; \
	else \
	  echo "grpcurl not found. Install it: https://github.com/fullstorydev/grpcurl/releases"; \
	  exit 1; \
	fi

clean:
	rm -rf trading-node/engine-net/SMC.Engine.Core/obj trading-node/engine-net/SMC.Engine.Core/bin \
	       trading-node/engine-net/SMC.Engine.Service/obj trading-node/engine-net/SMC.Engine.Service/bin \
	       trading-node/engine-net/publish \
	       trading-node/gateway-go/gateway \
	       watchdog-go/watchdog
