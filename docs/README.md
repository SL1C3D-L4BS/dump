# DUMP: The Front Door

**The only zero-trust, high-performance data sovereignty platform that discovers, masks, and mirrors enterprise data at wire speed.**

No cloud. No telemetry. Local-first AI. Post-quantum integrity. One binary.

---

## The Hook

DUMP treats 30-year-old EDI and X12 billing files like first-class streams. It discovers untracked assets, infers schemas with local Ollama, maps to Parquet or FHIR at wire speed, and seals every artifact with **Vericore** (Dilithium2 PQC + MMR). You get statistical mirroring, PII masking, and a single audit trail—without sending a byte to the cloud.

---

## Format Support

| Category | Formats | Notes |
|----------|---------|--------|
| **Healthcare** | EDI, X12 837/835, HL7 v2.5, FHIR | Embedded dialect pack; Z-segment resolution via LLM |
| **Structured** | JSONL, CSV, XML, Parquet | Streaming; XML block configurable |
| **Binary / RPC** | Protobuf (heuristic), gRPC-Web | Chrome extension WASM decoder |
| **Databases** | PostgreSQL, SQLite | Query to stream |
| **Tabular** | XLSX | Excel read/write |
| **IoT** | Bit-level binary (YAML schema) | MQTT ingest to Prometheus |

| Output Sinks | |
|--------------|--|
| Local | JSONL, Parquet, FHIR Bundle |
| Cloud | S3 |
| Observability | Prometheus Pushgateway, Elasticsearch |
| Integrity | Vericore seal on every mapped file; optional `--mask=pii` |

---

## Scan to Generate to Migrate: Enterprise Workflow

| Phase | Command | Purpose |
|-------|---------|--------|
| **Scan** | `dump scan --path .` | Shadow IT discovery: find CSV, XLSX, JSONL, EDI, X12; profile row count, PII density, schema complexity; suggest migration commands. |
| **Generate** | `dump stress -o legacy.x12` | Create synthetic X12 837 (or use real data). |
| | `dump analyze legacy.x12 --target=parquet` | Detect format, infer mapping schema (Ollama), write `inferred.yaml` + optional `custom_dialect.yaml` for Z-segments. |
| | `dump generate csharp legacy.x12` | TypeGen: emit C# POCOs from HL7/X12 so dev and prod speak the same types. |
| **Migrate** | `dump map legacy.x12 --schema inferred.yaml --input-type x12 --industry healthcare --format parquet --output secure.parquet --mask=pii` | Stream legacy to Parquet; Vericore seal; PII masking. |
| **Prove** | `dump verify secure.parquet --seal-file secure.parquet.vericore-seal` | Single-file integrity. |
| | `dump audit verify --all` | Full MMR audit log: re-compute hashes, verify PQC signatures. |

---

## The Proof

### $O(1)$ memory stability

Validated on a **1.2 GB** synthetic X12 837 payload (~2M CLM loops):

| Metric | Requirement | Result |
|--------|-------------|--------|
| **Payload** | 1.2 GB | `dump stress -o stress_837.x12 -s 1200` |
| **Peak heap** | Under 5 MB | **Under 5 MB** — no linear growth |
| **Throughput** | Wire speed | ~5,800 rows/s (X12 837) |

The engine streams. No load-all-in-memory. Tree resets per transaction; constant heap regardless of file size.

### Embedded Healthcare Dialect Pack

- **Standards:** `hl7_v25`, `x12_837`, `x12_835` embedded; no external dictionary required.
- **Acronym Resolver:** Unknown or custom segments (e.g. Z-segments) are detected, inferred via local Ollama, and written to `custom_dialect.yaml`; merged with the standard dialect for schema mapping.
- **Dev–prod parity:** `dump generate csharp` produces C# POCOs from the same EDI/X12 so application code and pipelines share one type model.

---

## Installation

- **Binaries:** [Releases](https://github.com/SL1C3D-L4BS/dump/releases)
- **From source (full stack):** `make` (Rust core + `dump` + `dump-api`)
- **Go-only:** `make go-only` or `go install github.com/SL1C3D-L4BS/dump@latest`

---

*SL1C3D-L4BS Launch Manifest — 8:00 AM PST. The Unicorn ships.*
