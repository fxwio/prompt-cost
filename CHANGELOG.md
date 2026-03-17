# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [1.0.0] - 2026-03-17

### Added

- **Prompt template management** — CRUD endpoints for named templates
  (`/templates`); each template supports Go `text/template` variable syntax
  (`{{.VarName}}`).

- **Template versioning** — immutable versioned snapshots (`/templates/{id}/versions`);
  `POST /templates/{id}/versions/{v}/activate` promotes a version to active;
  GET renders the active version by default.

- **Template rendering** — `POST /templates/{id}/render` substitutes caller-
  supplied variables and returns the final prompt text; integrated with
  `go-llm-gateway` via `X-Prompt-Template` request header.

- **LLM cost tracking** — `POST /usage` records prompt/completion token counts
  per model; `GET /usage` returns paginated usage events filterable by template,
  model, and time range.

- **Cost reporting** — `GET /reports/cost` aggregates usage events into
  total cost (USD) broken down by model or template; built-in pricing table
  covers major OpenAI and Anthropic models (2025-Q1 rates).

- **Pricing overrides** — `pricing.overrides` config section lets operators
  supply custom per-model input/output prices without code changes.

- **Prometheus metrics** — `prompt_render_total`, `usage_records_total`,
  `cost_usd_total` by model; scraped at `GET /metrics`.

- **Health endpoints** — `GET /health/live` and `GET /health/ready` (checks
  PostgreSQL connectivity).

- **pgstore integration** — automatic schema migration creates `templates`,
  `template_versions`, and `usage_events` tables.

- **Integration tests** — testcontainers-based test suite spins up a real
  PostgreSQL instance to validate the full template lifecycle and cost
  aggregation without mocks.

- **GitHub Actions CI** — build, vet, test (unit + integration), and Docker
  image build pipeline.

### Fixed

- Replaced `pgstore.Pool()` panic with `db()` helper pattern across the store
  layer; unavailable database returns a clean 503 (Day 33 hardening).
