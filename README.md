# Prompt & Cost Platform

A production-grade service for **prompt template management** and **LLM cost tracking**, written in Go. Designed to complement LLM gateways and RAG pipelines with the operational layer every AI application eventually needs.

## Features

### Prompt Template Management
- **Versioned templates** — immutable versions with auto-increment; rollback by activating any prior version
- **Variable injection** — Go `text/template` syntax (`{{.variable}}`); automatic variable extraction
- **Render API** — POST variables to get a fully rendered prompt; missing variable validation
- **Tag support** — organize templates by team, use-case, or environment

### LLM Cost Tracking
- **Usage events** — record every LLM API call with tenant, app, model, token counts
- **Automatic cost calculation** — built-in pricing table for 20+ models (OpenAI, Anthropic, Google Gemini)
- **Cost reports** — aggregate by `model`, `tenant`, or `app` over any time window
- **Pricing overrides** — add custom models or override built-in prices via `config.yaml`

### Operational
- **Prometheus metrics** — usage event rate, token counters, USD cost, render request rate
- **PostgreSQL backend** — templates, versions, usage events; NUMERIC(12,8) for cost precision
- **Structured logging** — Zap JSON format

## Built-in Pricing Table (2025-Q1)

| Model | Input/1M | Output/1M |
|---|---|---|
| gpt-4o | $2.50 | $10.00 |
| gpt-4o-mini | $0.15 | $0.60 |
| o1 | $15.00 | $60.00 |
| claude-3-5-sonnet | $3.00 | $15.00 |
| claude-3-haiku | $0.25 | $1.25 |
| gemini-1.5-pro | $1.25 | $5.00 |
| gemini-1.5-flash | $0.075 | $0.30 |
| text-embedding-3-small | $0.02 | — |

Full list: `GET /cost/models`

## Quick Start

```bash
cp .env.example .env
make dev
```

Service starts at `http://localhost:8092`.

## API Reference

### Templates

```bash
# Create template
POST /templates
{"name": "rag-answer", "description": "RAG answer generation", "tags": ["rag"]}

# List templates
GET /templates

# Get template
GET /templates/{id}

# Delete template (cascades versions)
DELETE /templates/{id}
```

### Template Versions

```bash
# Create new version (auto-activates first version)
POST /templates/{id}/versions
{"content": "Answer based on context.\n\nContext: {{.context}}\n\nQuestion: {{.question}}\n\nAnswer:"}

# List versions
GET /templates/{id}/versions

# Activate a specific version
PUT /templates/{id}/versions/{version}/activate
```

### Render

```bash
# Render active version with variables
POST /templates/{id}/render
{
  "variables": {
    "context": "Kubernetes is a container orchestration platform...",
    "question": "What is Kubernetes?"
  }
}
# → {"rendered": "Answer based on context...", "version": 2, "variables": ["context", "question"]}

# Render a specific version
POST /templates/{id}/render
{"version": 1, "variables": {...}}
```

### Usage Events

```bash
# Record a usage event (cost auto-calculated from model pricing)
POST /usage
{
  "tenant": "acme-corp",
  "app": "rag-service",
  "model": "gpt-4o-mini",
  "prompt_tokens": 850,
  "completion_tokens": 120,
  "metadata": {"template_id": "..."}
}
# → includes calculated cost_usd field

# List events with filters
GET /usage?tenant=acme-corp&model=gpt-4o-mini&from=2025-01-01&limit=50
```

### Cost Reports

```bash
# Cost report grouped by model
GET /cost/report?group_by=model&from=2025-01-01&to=2025-01-31

# Cost report grouped by tenant
GET /cost/report?group_by=tenant&from=2025-01-01

# Cost report grouped by app
GET /cost/report?group_by=app
```

Response includes per-group breakdown and totals:
```json
{
  "from": "2025-01-01T00:00:00Z",
  "to": "2025-01-31T23:59:59Z",
  "group_by": "model",
  "total_cost_usd": 12.34,
  "groups": [
    {
      "key": "gpt-4o",
      "event_count": 1420,
      "prompt_tokens": 1850000,
      "completion_tokens": 620000,
      "cost_usd": 10.82,
      "avg_cost_per_event": 0.00762
    }
  ]
}
```

### Pricing

```bash
# List all models with pricing
GET /cost/models
```

### Health & Metrics

```bash
GET /health   # {"status":"ok","database":true}
GET /metrics  # Prometheus metrics
```

## Configuration

```yaml
# config.yaml
server:
  port: 8092

postgres:
  dsn_env: POSTGRES_DSN

pricing:
  overrides:
    - model: "my-internal-model"
      input_per_1m_usd: 0.50
      output_per_1m_usd: 2.00
```

## Development

```bash
make build              # compile to build/prompt-cost
make test               # go test -race -cover ./internal/...
make test-integration   # testcontainers integration tests (requires Docker)
make lint               # golangci-lint
make dev                # docker compose up
make dev-down           # docker compose down
```

## Project Structure

```
cmd/server/           # main entry point
internal/
  api/                # HTTP handlers and router (chi)
  config/             # YAML config with env resolution
  metrics/            # Prometheus metrics registration
  model/              # domain types
  pricing/            # built-in pricing table + cost calculation
  store/              # PostgreSQL CRUD + cost report aggregation
  template/           # Go text/template renderer + variable extraction
pkg/pgstore/          # connection pool + schema migration
scripts/demo.sh       # end-to-end demo script
```

## Integration with Other Services

This service is designed to work alongside the other portfolio projects:

- **go-llm-gateway** → sends usage events after each proxied LLM call
- **rag-pipeline** → renders prompt templates before calling the LLM
- **llm-eval** → tracks evaluation costs per experiment
