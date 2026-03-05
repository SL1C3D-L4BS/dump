#!/usr/bin/env bash
set -e
cd "$(dirname "$0")"

if curl -s --connect-timeout 2 http://localhost:11434 >/dev/null 2>&1; then
  echo "🧠 1. Inferring Parquet Schema from Legacy CRM Data using AI (Ollama)..."
  ../dump infer legacy_crm.jsonl --target=parquet --model=llama3 > crm_schema.yaml
else
  echo "⚠️  Ollama is not running. Using pre-generated crm_schema.yaml."
  echo "   Install Ollama and run 'ollama run llama3' for AI schema inference."
fi

echo "⚡ 2. Mapping JSONL to Columnar Parquet and applying PQC Seal..."
../dump map legacy_crm.jsonl --schema=crm_schema.yaml --format=parquet --output=clean_crm.parquet

echo "✅ Demo complete. Check clean_crm.parquet and crm_schema.yaml."
