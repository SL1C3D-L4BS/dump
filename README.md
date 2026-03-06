# DUMP (Data Universal Mapping Platform)

**AI-assisted schema inference and high-performance data mapping.** Map JSON, Parquet, SQL, CSV, XML, EDI/HL7, and more using local Ollama for inference and a Rust core for zero-copy streaming and PQC integrity.

[![Go Version](https://img.shields.io/github/go-mod/go-version/SL1C3D-L4BS/dump)](https://golang.org/)
[![PQC-Secured](https://img.shields.io/badge/PQC-Secured-8A2BE2)](#)

---

## Privacy-first

DUMP uses **local Ollama** for schema inference and mapping suggestions. No cloud APIs or telemetry; data stays on your machine.

---

## What's in the repo

* **CLI** (`cmd/`) — Infer, map, analyze, fan-out, proxy, verify, diff, ingest, nl2s, decode, mirror (Go + optional Rust via cgo).
* **Rust core** (`internal/core-rs/`) — Mapping engine, Vericore PQC (Dilithium2), Arrow IPC.
* **API** (`api/`) — Fiber HTTP server: POST `/map`, verification, Ollama discovery.
* **Desktop app** (`app/`) — Tauri v2 + React + Vite: mapping graph, verification dropzone, Ollama panel, live data preview.
* **Chrome extension** (`app/extension/`) — DevTools panel: decode Protobuf/gRPC-Web payloads via WASM (heuristic decoder).

**Sample files** in the repo root for quick try-outs: `sample.jsonl`, `sample_schema.yaml`, and `stress_837.x12` (X12 837 sample). More test data in `testdata/` and `demo/`.

---

## Installation

* **Pre-built binaries:** See [Releases](https://github.com/SL1C3D-L4BS/dump/releases) for your OS/arch.
* **From source (Go only):**  
  `go install github.com/SL1C3D-L4BS/dump@latest`

---

## Building from source

* **Rust core + Go CLI (cgo):**  
  `make`  
  Builds Rust lib then `dump` and `dump-api`.
* **Pure Go (no Rust):**  
  `make go-only`  
  Produces `dump` and `dump-api` without the Rust engine.
* **API only (Rust-linked):**  
  `make api`
* **Chrome extension (Protobuf WASM decoder):**  
  `make extension`  
  Builds `app/extension/lib/dump.wasm` and copies `wasm_exec.js`. Load `app/extension` as an unpacked extension in Chrome; open DevTools → **DUMP Proto** tab.
* **Desktop app:**  
  From repo root, build Rust core first, then:
  ```bash
  cd app && pnpm install && pnpm tauri build
  ```
  Run in dev: `pnpm tauri dev`

---

## Workflow

1. **Infer:** `dump infer source.json --target=parquet > schema.yaml`  
   Use `--from=xml` or `--from=edi` for non-JSON sources.
2. **Map:** `dump map source.json --schema=schema.yaml --format=parquet --output=output.parquet`  
   Use `--input-type xml`, `--input-type edi`, `--input-type x12`, or `--input-type fhir` (with `--dialect` for EDI/X12). Use `--format fhir` to write a FHIR Bundle. Add `--mask=pii` to anonymize PII in the stream.
3. **Analyze (zero-knowledge):** `dump analyze mystery.txt --target=parquet`  
   Detects format (jsonl, csv, xml, edi), samples the file, and infers a mapping via Ollama. For EDI/X12, the **Healthcare Dialect Pack & Acronym Resolver** (SL1C3D-L4BS) runs automatically: unknown or custom segments (e.g. Z-segments) are detected against embedded standards (`hl7_v25`, `x12_837`, `x12_835`), an LLM infers custom segment definitions, and the result is written to `custom_dialect.yaml` and merged with the standard dialect for schema mapping.
4. **Fan-out:** `dump fanout --config fanout.yaml`  
   Multiplexes one stream to local files, S3, Prometheus Pushgateway, and/or Elasticsearch. Supports `--mask=pii`.
5. **Proxy (JIT sidecar):** `dump proxy --upstream http://legacy.example.com/api --schema=schema.yaml --port=8081`  
   Translates upstream XML to JSONL on the fly.
6. **Verify:** Use the CLI or the desktop app to verify Parquet files sealed with the Vericore PQC MMR.
7. **Diff (heterogeneous):** `dump diff --s1 a.xlsx --s2 b.jsonl --on id` — Compare two sources (Excel, JSON, CSV, SQL) on a primary key.
8. **NL2S (prompt-to-schema):** `echo "text" | dump nl2s --template schema.json` — Extract structured JSON from unstructured text via Ollama.
9. **IoT ingest:** `dump ingest mqtt --broker tcp://localhost:1883 --topic telemetry/# --schema bits.yaml --pushgateway http://localhost:9091` — Decode binary MQTT payloads with a bit-level schema and push to Prometheus.
10. **FHIR:** `dump map bundle.json --input-type fhir --schema schema.yaml --format fhir --output out.json` — Stream FHIR Bundles or single resources in; write mapped rows as a FHIR Bundle out.

---

## CLI commands

| Command   | Description |
|----------|-------------|
| `dump infer [file]`   | Infer a YAML mapping from sample data (Ollama). `--target`, `--from=json\|xml\|edi`, `--model` |
| `dump map [file]`     | Map input to JSONL, Parquet, or FHIR Bundle. `--schema`, `--input-type` (jsonl/csv/xml/edi/x12/fhir), `--format` (jsonl/parquet/fhir), `--mask=pii`, `--dialect` (EDI/X12), `--xml-block` |
| `dump analyze [file]` | Detect format and infer mapping from a mystery file. For EDI/X12, auto-resolves custom (Z-) segments via embedded dialect pack + Ollama. `--target`, `--model`, `--dialect` (optional for EDI) |
| `dump fanout`        | Multi-target fan-out from a YAML config. `--config`, `--mask=pii` |
| `dump proxy`         | HTTP sidecar: forward requests to upstream and stream mapped JSONL. `--upstream`, `--schema`, `--port`, `--xml-block` |
| `dump verify [file]`  | Verify a file against its Vericore seal. `--seal`, `--seal-file` |
| `dump diff`          | Heterogeneous diff: compare two sources (XLSX/JSON/CSV/SQL) on a primary key. `--s1`, `--s2`, `--on`, `--ignore`, `--format` (table\|json) |
| `dump nl2s`          | AI prompt-to-schema: extract JSON from stdin to match a template via Ollama. `--template`, `--model`, `--strict` |
| `dump ingest mqtt`    | MQTT ingestor: decode binary payloads with a bit-level YAML schema and push to Prometheus. `--broker`, `--topic`, `--schema`, `--pushgateway` |
| `dump decode`        | Decode Protobuf (heuristic) or other formats. |
| `dump mirror`        | Mirror/sync data from a source to a destination. |

---

## Architecture

* **CLI:** Go (Cobra), optional cgo link to Rust for mapping and verification.
* **Rust core:** Schema application, row mapping, Arrow IPC, Dilithium2 signing/verification.
* **Inference:** Ollama-only for schema and mapping suggestions.
* **Formats:** JSONL, CSV, XML (streaming), EDI/HL7 (with dialect YAML), SQL (Postgres, SQLite), Excel (XLSX), X12 (stateful loops), binary (bit-level for IoT), FHIR (streaming Bundle/single resource).
* **Healthcare Dialect Pack:** Embedded standard dialects (`internal/dialects/std/`: `hl7_v25`, `x12_837`, `x12_835`) plus an LLM-driven **Acronym Resolver** that infers custom/undocumented segments (e.g. Z-segments) during `dump analyze` and writes `custom_dialect.yaml` for merged schema mapping.
* **Sinks:** Local JSONL/Parquet/FHIR Bundle, S3, Prometheus Pushgateway, Elasticsearch; PII masking via `--mask=pii`.
* **Desktop:** Tauri v2, React, Tailwind; calls into Rust core for mapping and verification.
* **Browser:** Chrome DevTools extension (WASM) for heuristic Protobuf/gRPC-Web decoding in the Network panel.
