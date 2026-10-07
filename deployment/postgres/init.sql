CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS trading_sessions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    strategy_name   VARCHAR(100) NOT NULL,
    started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at        TIMESTAMPTZ,
    initial_balance DECIMAL(18,8) NOT NULL,
    current_equity  DECIMAL(18,8) NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS orders (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id        UUID NOT NULL REFERENCES trading_sessions(id),
    exchange_order_id VARCHAR(100),
    asset             VARCHAR(20) NOT NULL,
    direction         VARCHAR(10) NOT NULL,
    size              DECIMAL(18,8) NOT NULL,
    status            VARCHAR(20) NOT NULL,
    execution_price   DECIMAL(18,8),
    order_type        VARCHAR(20) NOT NULL,
    limit_price       DECIMAL(18,8),
    stop_price        DECIMAL(18,8),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    filled_at         TIMESTAMPTZ,
    rejected_reason   TEXT
);

CREATE TABLE IF NOT EXISTS positions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id      UUID NOT NULL REFERENCES trading_sessions(id),
    asset           VARCHAR(20) NOT NULL,
    direction       VARCHAR(10) NOT NULL,
    entry_price     DECIMAL(18,8) NOT NULL,
    current_size    DECIMAL(18,8) NOT NULL DEFAULT 0,
    stop_loss       DECIMAL(18,8),
    take_profit     DECIMAL(18,8),
    status          VARCHAR(20) NOT NULL DEFAULT 'open',
    opened_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    closed_at       TIMESTAMPTZ,
    realized_pnl    DECIMAL(18,8)
);

CREATE TABLE IF NOT EXISTS risk_limits (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id          UUID REFERENCES trading_sessions(id),
    daily_loss_limit    DECIMAL(18,8) NOT NULL,
    max_open_positions  INTEGER NOT NULL,
    global_halt         BOOLEAN NOT NULL DEFAULT false,
    max_position_size   DECIMAL(18,8),
    max_leverage        DECIMAL(10,2),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS strategy_config (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(100) NOT NULL UNIQUE,
    value       JSONB NOT NULL,
    description TEXT,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS audit_log (
    id              BIGSERIAL PRIMARY KEY,
    session_id      UUID NOT NULL REFERENCES trading_sessions(id),
    event_type      VARCHAR(50) NOT NULL,
    correlation_id  UUID,
    payload         JSONB NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE OR REPLACE FUNCTION reject_audit_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'audit_log is immutable: UPDATE and DELETE are prohibited';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_audit_log_immutable ON audit_log;
CREATE TRIGGER trg_audit_log_immutable
    BEFORE UPDATE OR DELETE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION reject_audit_mutation();

CREATE TABLE IF NOT EXISTS system_state (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    system_id       VARCHAR(50) NOT NULL UNIQUE DEFAULT 'primary_instance',
    state           VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    trading_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO system_state (system_id, state, trading_enabled)
VALUES ('primary_instance', 'ACTIVE', TRUE)
ON CONFLICT (system_id) DO NOTHING;
