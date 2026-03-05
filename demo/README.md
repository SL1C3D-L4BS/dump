# DUMP Demo — AI Inference + Parquet + PQC Seal

Self-contained demo that shows the full pipeline: messy JSONL → AI-inferred schema → columnar Parquet with Vericore integrity seal. **Runs entirely on local AI (Ollama); no cloud or API keys.**

## Quick run

From this directory:

```bash
./run_demo.sh
```

- **With Ollama running:** Full flow — infers a new schema from `legacy_crm.jsonl` with local AI, then maps to Parquet and applies the PQC seal.
- **Without Ollama:** Uses the pre-generated `crm_schema.yaml` and runs the map step only (Parquet + seal).

## Running Ollama

Ollama must be **running as a server** on `http://localhost:11434` (the DUMP CLI talks to it via HTTP). You do **not** need to run `ollama run llama3` in the same terminal — that starts the **interactive chat** and will block the shell.

- **macOS:** Open the Ollama app (or run `ollama serve` in a separate terminal). Then in another terminal: `ollama pull llama3` (once), then `cd demo && ./run_demo.sh`.
- **MLX warnings** (e.g. "Failed to load MLX dynamic library") come from Ollama on Apple Silicon and are usually harmless; the server still works.

## Outputs

- `crm_schema.yaml` — Mapping schema (inferred by Ollama or pre-generated).
- `clean_crm.parquet` — Mapped columnar output.
- Vericore Seal (MMR root + PQC signature + file hash) is printed to stderr in violet.

## Source data

`legacy_crm.jsonl` contains 7 rows of intentionally messy CRM-style data: nested objects, mixed casing (`Customer_ID` vs `customer_id`), stringified numbers (`"145000.50"`), and string booleans (`"true"`) to exercise the inferencer and mapper.
