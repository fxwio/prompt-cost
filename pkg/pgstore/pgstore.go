// Package pgstore manages the PostgreSQL connection pool and schema migrations.
package pgstore

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	mu   sync.RWMutex
	pool *pgxpool.Pool
)

func Init(ctx context.Context, dsn string, maxConns, minConns int32) error {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("parse DSN: %w", err)
	}
	cfg.MaxConns = maxConns
	cfg.MinConns = minConns
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute

	p, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return fmt.Errorf("create pgxpool: %w", err)
	}
	if err := p.Ping(ctx); err != nil {
		p.Close()
		return fmt.Errorf("ping postgres: %w", err)
	}
	mu.Lock()
	pool = p
	mu.Unlock()
	return migrate(ctx, p)
}

func Pool() *pgxpool.Pool {
	mu.RLock()
	defer mu.RUnlock()
	if pool == nil {
		panic("pgstore: Pool() called before Init()")
	}
	return pool
}

func Available() bool {
	mu.RLock()
	p := pool
	mu.RUnlock()
	if p == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return p.Ping(ctx) == nil
}

func Close() {
	mu.Lock()
	defer mu.Unlock()
	if pool != nil {
		pool.Close()
		pool = nil
	}
}

func migrate(ctx context.Context, p *pgxpool.Pool) error {
	if _, err := p.Exec(ctx, schema); err != nil {
		return fmt.Errorf("pgstore migrate: %w", err)
	}
	return nil
}

const schema = `
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Prompt templates
CREATE TABLE IF NOT EXISTS templates (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name           TEXT NOT NULL UNIQUE,
    description    TEXT NOT NULL DEFAULT '',
    tags           TEXT[] NOT NULL DEFAULT '{}',
    active_version INTEGER NOT NULL DEFAULT 0,
    version_count  INTEGER NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_templates_name ON templates(name);

-- Template versions (immutable)
CREATE TABLE IF NOT EXISTS template_versions (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    template_id UUID NOT NULL REFERENCES templates(id) ON DELETE CASCADE,
    version     INTEGER NOT NULL,
    content     TEXT NOT NULL,
    variables   TEXT[] NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (template_id, version)
);
CREATE INDEX IF NOT EXISTS idx_template_versions_template_id ON template_versions(template_id);

-- LLM usage events
CREATE TABLE IF NOT EXISTS usage_events (
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant            TEXT NOT NULL DEFAULT '',
    app               TEXT NOT NULL DEFAULT '',
    model             TEXT NOT NULL,
    prompt_tokens     INTEGER NOT NULL DEFAULT 0,
    completion_tokens INTEGER NOT NULL DEFAULT 0,
    cost_usd          NUMERIC(12,8) NOT NULL DEFAULT 0,
    metadata          JSONB NOT NULL DEFAULT '{}',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_usage_events_tenant    ON usage_events(tenant);
CREATE INDEX IF NOT EXISTS idx_usage_events_app       ON usage_events(app);
CREATE INDEX IF NOT EXISTS idx_usage_events_model     ON usage_events(model);
CREATE INDEX IF NOT EXISTS idx_usage_events_created_at ON usage_events(created_at);
`
