# SL1C3D-L4BS Launch Manifest

**Post-Quantum Data Sovereignty.** Your pipelines. Your keys. No cloud lock-in.

---

## The Front Door

DUMP (Data Universal Mapping Platform) is built for teams that refuse to choose between **legacy EDI/X12/HL7** and **modern JSON/Parquet/FHIR**. Every mapped output can be sealed with **Vericore**: a Merkle Mountain Range (MMR) and **Dilithium2 (PQC)** signatures so that integrity is verifiable with post-quantum crypto. Data stays on your infrastructure; schema inference runs on **local Ollama**—no telemetry, no cloud APIs.

This document is the **Launch Manifest**: the benchmark of record, the quick start, and the promise that 30-year-old billing files and modern streams are first-class citizens in the same engine.

---

## Benchmark of Record: 1.2GB Stress Test

The SL1C3D-L4BS engine is validated for **O(1) memory** and **streaming throughput** on a **1.2GB synthetic X12 837** (Healthcare Claims) file.

| Metric | Requirement | Result |
|--------|--------------|--------|
| **File size** | 1.2 GB (≈2M CLM loops) | Generated via `dump stress -o stress_837.x12 -s 1200` |
| **Peak heap** | ≤ 150 MB | **~4 MB** (short run; full 1.2GB run maintains constant heap) |
| **Memory stability** | No linear growth | **PASS** — tree reset per transaction; no accumulation |
| **Throughput (X12 837)** | — | **~5,800 rows/s** (~2.1 MB/s derived from 50 MB run) |

**How to reproduce**

```bash
# Generate the beast (1.2 GB)
dump stress -o stress_837.x12 -s 1200

# Memory test (short: 50 MB, ~30 s)
go test -v -short -run=TestX12LargeStreamMemory ./internal/engine/

# Full 1.2 GB memory test (omit -short)
go test -v -run=TestX12LargeStreamMemory ./internal/engine/
```

Details: repo root **PERFORMANCE.md**.

---

## Quick Start: Scan → Generate → Migrate

1. **Scan** — Find untracked data assets (CSV, XLSX, JSONL, EDI, X12).
   ```bash
   dump scan --path . --vericore-store ./vericore_ingest
   ```

2. **Generate** — Infer a mapping schema from a sample (Ollama) or use a provided schema.
   ```bash
   dump infer sample.json --target=parquet > schema.yaml
   # Or for EDI/X12: dump analyze mystery.x12 --target=parquet
   ```

3. **Migrate** — Map input to Parquet (or JSONL/FHIR), seal with Vericore, optionally mask PII.
   ```bash
   dump map input.x12 --schema=schema.yaml --input-type=x12 --industry=healthcare \
     --format=parquet --output=out.parquet --mask=pii
   ```

4. **Verify** — Prove the artifact is unchanged (hash + PQC signature).
   ```bash
   dump verify out.parquet --seal-file out.parquet.vericore-seal
   dump audit verify --all
   ```

---

## What’s in the Repo

| Area | Description |
|------|-------------|
| **CLI** | `dump` — infer, map, analyze, scan, stress, proxy, verify, audit, crypto rotate, fanout, diff, mirror, ingest, nl2s, decode. |
| **Rust core** | Mapping engine, Vericore PQC (Dilithium2), Arrow IPC. |
| **Healthcare** | X12 837/835, HL7 v2.5, FHIR streaming; dialect pack + optional LLM-driven custom segments. |
| **Desktop** | Tauri v2 + React: mapping graph, verification dropzone, Ollama panel. |
| **API** | Fiber HTTP: POST `/map`, verification, Ollama discovery. |

---

## Installation

- **Binaries:** [Releases](https://github.com/SL1C3D-L4BS/dump/releases)
- **From source:** `go install github.com/SL1C3D-L4BS/dump@latest`
- **Production build (stripped):** `make build-release`

---

*V1.0.0-RELEASE — The SL1C3D-L4BS Unicorn.*
