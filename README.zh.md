# Prompt & Cost Platform — Prompt 管理与成本追踪平台

用 Go 实现的生产级服务，提供 **Prompt 模板管理**（版本控制 + 变量注入）和 **LLM 成本追踪**（按租户/应用/模型维度的消费报表）。是每个 AI 应用最终都需要的运营管理层。

## 功能特性

### Prompt 模板管理
- **版本控制** — 不可变版本自动递增，通过激活任意历史版本实现回滚
- **变量注入** — 使用 Go `text/template` 语法（`{{.变量名}}`），自动提取变量列表
- **渲染 API** — POST 变量即可获得完整渲染后的 Prompt；自动校验缺失变量
- **标签系统** — 按团队、用途、环境对模板分类管理

### LLM 成本追踪
- **使用事件记录** — 记录每次 LLM API 调用（租户、应用、模型、Token 数）
- **自动成本计算** — 内置 20+ 主流模型定价表（OpenAI、Anthropic、Google Gemini）
- **成本报表** — 按 `model`、`tenant`、`app` 维度聚合，支持任意时间窗口
- **定价覆盖** — 通过 `config.yaml` 添加自定义模型或覆盖内置价格

### 可观测性
- **Prometheus 指标** — 使用事件速率、Token 计数器、USD 成本累计、渲染请求速率
- **PostgreSQL 持久化** — 模板、版本、使用事件全量存储；NUMERIC(12,8) 保证成本精度
- **结构化日志** — Zap JSON 格式，可直接接入 Loki/Grafana

## 内置定价表（2025-Q1）

| 模型 | 输入/1M | 输出/1M |
|---|---|---|
| gpt-4o | $2.50 | $10.00 |
| gpt-4o-mini | $0.15 | $0.60 |
| o1 | $15.00 | $60.00 |
| claude-3-5-sonnet | $3.00 | $15.00 |
| claude-3-haiku | $0.25 | $1.25 |
| gemini-1.5-pro | $1.25 | $5.00 |
| gemini-1.5-flash | $0.075 | $0.30 |
| text-embedding-3-small | $0.02 | — |

完整列表：`GET /cost/models`

## 快速开始

```bash
cp .env.example .env
make dev
```

服务启动后访问：`http://localhost:8092`

## API 接口

### 模板管理

```bash
# 创建模板
POST /templates
{"name": "rag-answer", "description": "RAG 回答生成模板", "tags": ["rag"]}

# 列出所有模板
GET /templates

# 获取单个模板
GET /templates/{id}

# 删除模板（级联删除所有版本）
DELETE /templates/{id}
```

### 模板版本

```bash
# 创建新版本（第一个版本自动激活）
POST /templates/{id}/versions
{"content": "根据以下上下文回答问题。\n\n上下文：{{.context}}\n\n问题：{{.question}}\n\n回答："}

# 列出所有版本
GET /templates/{id}/versions

# 激活指定版本（用于回滚）
PUT /templates/{id}/versions/{version}/activate
```

### 模板渲染

```bash
# 用变量渲染当前激活版本
POST /templates/{id}/render
{
  "variables": {
    "context": "Kubernetes 是一个容器编排平台...",
    "question": "Kubernetes 是做什么的？"
  }
}
# 返回：{"rendered": "根据以下上下文...", "version": 2, "variables": ["context", "question"]}

# 渲染指定版本
POST /templates/{id}/render
{"version": 1, "variables": {...}}
```

### 使用事件

```bash
# 记录一次 LLM 调用（自动按模型定价计算成本）
POST /usage
{
  "tenant": "acme-corp",
  "app": "rag-service",
  "model": "gpt-4o-mini",
  "prompt_tokens": 850,
  "completion_tokens": 120,
  "metadata": {"template_id": "..."}
}
# 返回含 cost_usd 字段的完整事件记录

# 按条件查询使用记录
GET /usage?tenant=acme-corp&model=gpt-4o-mini&from=2025-01-01&limit=50
```

### 成本报表

```bash
# 按模型聚合成本
GET /cost/report?group_by=model&from=2025-01-01&to=2025-01-31

# 按租户聚合成本
GET /cost/report?group_by=tenant&from=2025-01-01

# 按应用聚合成本
GET /cost/report?group_by=app
```

返回示例：
```json
{
  "from": "2025-01-01T00:00:00Z",
  "to":   "2025-01-31T23:59:59Z",
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

### 定价查询

```bash
# 查看所有支持的模型及定价
GET /cost/models
```

## 配置说明

```yaml
# config.yaml
server:
  port: 8092

postgres:
  dsn_env: POSTGRES_DSN   # 环境变量名，非变量值

pricing:
  overrides:
    - model: "my-internal-model"
      input_per_1m_usd: 0.50
      output_per_1m_usd: 2.00
```

## 开发命令

```bash
make build              # 编译二进制到 build/prompt-cost
make test               # go test -race -cover ./internal/...
make test-integration   # testcontainers 集成测试（需要 Docker）
make lint               # golangci-lint
make dev                # docker compose up --build -d
make dev-down           # docker compose down
```

## 项目结构

```
cmd/server/           # 程序入口
internal/
  api/                # HTTP Handler 和路由（chi）
  config/             # YAML 配置，支持环境变量解析
  metrics/            # Prometheus 指标注册
  model/              # 领域类型（Template、TemplateVersion、UsageEvent、CostReport）
  pricing/            # 内置定价表 + 成本计算
  store/              # PostgreSQL CRUD + 成本报表聚合查询
  template/           # Go text/template 渲染器 + 变量提取
pkg/pgstore/          # 连接池 + Schema 迁移
scripts/demo.sh       # 端到端演示脚本
```

## 核心设计亮点

### 1. 模板版本控制 — AI 应用的 Git

传统应用的配置通常直接修改，但 Prompt 的修改会直接影响模型输出效果。版本控制让你可以：
- **A/B 测试**：版本 1 vs 版本 2 的效果对比（配合 llm-eval 使用）
- **安全回滚**：新版本效果变差时，一个 API 调用即可切回旧版本
- **审计追踪**：每次 Prompt 变更都有记录

### 2. 自动成本计算 — "零负担"成本感知

调用方只需传入 `model`、`prompt_tokens`、`completion_tokens`，平台自动完成：
```
cost_usd = (prompt_tokens / 1_000_000) × input_price
         + (completion_tokens / 1_000_000) × output_price
```
不同模型价格差距可达 **100x**（claude-3-opus vs claude-3-haiku），精确的成本追踪让架构决策有了量化依据。

### 3. 多维成本报表 — 真正的财务可见性

按 `model` 聚合：找出哪个模型花钱最多
按 `tenant` 聚合：按客户分摊 LLM 成本（SaaS 计费基础）
按 `app` 聚合：对比不同业务线的 LLM 投入

## 与其他服务的集成

本服务设计为整个 AI 基础设施栈的运营管理层：

- **go-llm-gateway** → 每次代理请求后向 `POST /usage` 上报用量事件
- **rag-pipeline** → 调用 LLM 前通过 `POST /templates/{id}/render` 渲染标准化 Prompt
- **llm-eval** → 记录每次评估实验的 Token 消耗和成本
