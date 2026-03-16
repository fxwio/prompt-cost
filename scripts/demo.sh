#!/usr/bin/env bash
# demo.sh — end-to-end demo for prompt-cost
# Prerequisites: service running at http://localhost:8092
set -euo pipefail

BASE="http://localhost:8092"

echo "========================================"
echo " Prompt & Cost Platform — Demo"
echo "========================================"
echo ""

# ── Step 1: Health check ─────────────────────────────────────────────────────
echo "Step 1: Health check"
curl -s "$BASE/health" | python3 -m json.tool
echo ""

# ── Step 2: List available models with pricing ───────────────────────────────
echo "Step 2: List models (first 5)"
curl -s "$BASE/cost/models" | python3 -c "
import sys,json
models = json.load(sys.stdin)
for m in sorted(models, key=lambda x: x['model'])[:5]:
    print(f\"  {m['model']:40s} in=\${m['input_per_1m_usd']}/1M  out=\${m['output_per_1m_usd']}/1M\")
"
echo ""

# ── Step 3: Create a prompt template ─────────────────────────────────────────
echo "Step 3: Create prompt template"
TEMPLATE=$(curl -s -X POST "$BASE/templates" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "rag-answer",
    "description": "RAG answer generation template",
    "tags": ["rag", "qa"]
  }')
echo "$TEMPLATE" | python3 -m json.tool
TEMPLATE_ID=$(echo "$TEMPLATE" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "Template ID: $TEMPLATE_ID"
echo ""

# ── Step 4: Create version 1 ─────────────────────────────────────────────────
echo "Step 4: Create template version 1"
V1=$(curl -s -X POST "$BASE/templates/$TEMPLATE_ID/versions" \
  -H "Content-Type: application/json" \
  -d '{
    "content": "Answer the question based on the context.\n\nContext:\n{{.context}}\n\nQuestion: {{.question}}\n\nAnswer:"
  }')
echo "$V1" | python3 -m json.tool
echo ""

# ── Step 5: Create version 2 (improved) ──────────────────────────────────────
echo "Step 5: Create template version 2 (with persona)"
V2=$(curl -s -X POST "$BASE/templates/$TEMPLATE_ID/versions" \
  -H "Content-Type: application/json" \
  -d '{
    "content": "You are a {{.persona}} expert. Answer concisely based only on the provided context.\n\nContext:\n{{.context}}\n\nQuestion: {{.question}}\n\nAnswer:"
  }')
echo "$V2" | python3 -m json.tool
echo ""

# ── Step 6: Activate version 2 ───────────────────────────────────────────────
echo "Step 6: Activate version 2"
curl -s -X PUT "$BASE/templates/$TEMPLATE_ID/versions/2/activate" | python3 -m json.tool
echo ""

# ── Step 7: Render template ───────────────────────────────────────────────────
echo "Step 7: Render template with variables"
RENDERED=$(curl -s -X POST "$BASE/templates/$TEMPLATE_ID/render" \
  -H "Content-Type: application/json" \
  -d '{
    "variables": {
      "persona": "cloud computing",
      "context": "Kubernetes is an open-source container orchestration platform that automates deployment, scaling, and management of containerized applications.",
      "question": "What is Kubernetes used for?"
    }
  }')
echo "$RENDERED" | python3 -m json.tool
echo ""

# ── Step 8: Record usage events ───────────────────────────────────────────────
echo "Step 8: Record 3 usage events"
for i in 1 2 3; do
  curl -s -X POST "$BASE/usage" \
    -H "Content-Type: application/json" \
    -d "{
      \"tenant\": \"acme-corp\",
      \"app\": \"rag-app\",
      \"model\": \"gpt-4o-mini\",
      \"prompt_tokens\": $((500 * i)),
      \"completion_tokens\": $((100 * i)),
      \"metadata\": {\"template_id\": \"$TEMPLATE_ID\"}
    }" | python3 -c "
import sys,json
e = json.load(sys.stdin)
print(f\"  id={e['id'][:8]}...  cost=\${e['cost_usd']:.6f}\")
"
done
echo ""

# ── Step 9: Cost report by model ──────────────────────────────────────────────
echo "Step 9: Cost report grouped by model"
curl -s "$BASE/cost/report?group_by=model&from=$(date -u -v-1d +%Y-%m-%dT%H:%M:%SZ)" | python3 -m json.tool
echo ""

# ── Step 10: Cost report by tenant ───────────────────────────────────────────
echo "Step 10: Cost report grouped by tenant"
curl -s "$BASE/cost/report?group_by=tenant&from=$(date -u -v-1d +%Y-%m-%dT%H:%M:%SZ)" | python3 -m json.tool
echo ""

echo "========================================"
echo " Demo complete!"
echo " Templates:    $BASE/templates"
echo " Cost report:  $BASE/cost/report?group_by=model"
echo " Prometheus:   $BASE/metrics"
echo "========================================"
