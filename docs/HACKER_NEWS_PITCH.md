# Show HN: DUMP – We treat 30-year-old X12 billing files like modern JSON streams (local AI + PQC)

**TL;DR:** We built a tool that treats 30-year-old X12 billing files like modern JSON streams using **local-first AI** and **PQC signatures**. It solves the dev–prod parity nightmare with statistical mirroring and one audit trail. No cloud. No telemetry.

---

## The problem

Healthcare and insurance still run on EDI/X12/HL7. Files are huge, schemas are tribal, and “test in prod” is a joke because you can’t mirror 1.2 GB of claims without blowing the heap or shipping PII. Dev and prod drift. Auditors want proof that the migrated Parquet is bit-identical to the source. Nobody wants to send that data to a SaaS.

## What we built

**DUMP** (Data Universal Mapping Platform):

- **Discover:** `dump scan --path .` — finds untracked CSV, XLSX, JSONL, EDI, X12; profiles row count, PII density, schema complexity; suggests migration commands.
- **Understand:** `dump generate csharp legacy.x12` — TypeGen: strictly-typed C# POCOs from HL7/X12 so dev and prod speak the same types. No more “works in staging, breaks in prod” type mismatches.
- **Stream:** `dump map legacy.x12 --schema inferred.yaml --format parquet --output secure.parquet` — X12 → Parquet at wire speed. **O(1) memory:** we validated 1.2 GB input with **&lt; 5 MB heap**. No load-all-in-memory.
- **Seal:** Every mapped file gets a **Vericore** seal: Merkle Mountain Range + **Dilithium2 (post-quantum)** signature. `dump audit verify --all` re-computes hashes and verifies every signature in the audit log.
- **Mask:** `--mask=pii` in the map step; PII is anonymized in the stream before it hits Parquet or S3.

Schema inference runs on **local Ollama**. No cloud APIs. No telemetry. Data never leaves your box.

We ship an **embedded Healthcare Dialect Pack** (X12 837/835, HL7 v2.5). Custom or undocumented segments (Z-segments) are detected and inferred by the LLM; we write `custom_dialect.yaml` and merge it with the standard dialect so mapping stays consistent.

## Dev–prod parity and statistical mirroring

The nightmare: “It works on the 10 MB sample; prod is 1.2 GB and the process OOMs.” We generate a **1.2 GB synthetic X12 837** with `dump stress -o legacy.x12 -s 1200` and run the full pipeline. Same binary, same schema, same seal. That’s the statistical mirror: if it seals and verifies on the beast, it will on prod.

TypeGen (`dump generate csharp`) means the app that reads the mapped data uses the same types as the pipeline that wrote them. One source of truth. No “we changed the segment and forgot to regenerate the DTOs.”

## Tech stack

- **CLI:** Go (Cobra); optional cgo link to **Rust** for the mapping hot path and Vericore (Dilithium2 + MMR).
- **Formats:** JSONL, CSV, XML, EDI, X12, FHIR (streaming), Parquet, SQL (Postgres/SQLite), XLSX, Protobuf (heuristic), bit-level binary for IoT.
- **Sinks:** Local files, S3, Prometheus Pushgateway, Elasticsearch.
- **Desktop:** Tauri v2 + React (mapping graph, verification dropzone, Ollama panel).
- **Browser:** Chrome extension (WASM) for Protobuf/gRPC-Web decode in DevTools.

Repo: [github.com/SL1C3D-L4BS/dump](https://github.com/SL1C3D-L4BS/dump)  
Binaries: [Releases](https://github.com/SL1C3D-L4BS/dump/releases)

We’re happy to answer questions and hear how you’re handling legacy EDI/X12 and integrity auditing today.
