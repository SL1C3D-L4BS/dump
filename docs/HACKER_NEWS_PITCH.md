# Show HN: We built a tool that treats 30-year-old X12 billing files like modern JSON streams (local-first AI + PQC signatures)

**TL;DR:** DUMP is a CLI + desktop app that streams EDI/X12/HL7 through the same engine as JSON and Parquet. Schema inference runs on **local Ollama**; every mapped output gets a **post-quantum (Dilithium2)** signature and Merkle Mountain Range so you can prove nothing was altered. We’re calling it the death of the legacy–modern gap.

---

## The problem

You have **X12 837** from 1995, **HL7 v2** from the lab, and a stack that expects **JSON** or **Parquet**. Today that usually means brittle ETL, hand-written parsers, or “just put it in the data lake and fix it later.” Nobody wants to treat 30-year-old billing files as first-class citizens next to modern streams.

## What we built

**DUMP** (Data Universal Mapping Platform) does three things without leaving your machine:

1. **One streaming engine** — X12, HL7, XML, CSV, JSONL, and SQL feed the same pipeline. We built a stateful X12 reader that yields one JSON “row” per claim (CLM) so you can map it with the same YAML schema you use for JSON. No batch load; no “load entire file into memory.” We stress-tested it on a **1.2GB synthetic X12 file**: heap stays flat (~4 MB peak), ~5.8k claims/sec.

2. **Local-first AI** — Schema inference and “mystery file” analysis use **Ollama** on your box. No cloud APIs, no telemetry. You point `dump analyze` at a random .x12 or .edi file; it detects the format, uses embedded healthcare dialects (x12_837, x12_835, hl7_v25), and optionally uses an LLM to infer custom (e.g. Z-) segments. Then you map with one command.

3. **Post-quantum integrity** — Every `dump map` or `dump mirror` that writes a file can append to a **Vericore** seal: file hash, MMR root, and a **Dilithium2** signature. You get a persistent audit log (`~/.vericore/audit.db`) and `dump audit verify --all` to re-hash files and verify signatures so you can prove the beast is unchanged.

We added a **bi-directional proxy** too: JSON in → up-converted to X12 → sent to the legacy backend; X12 response → down-converted to FHIR/JSON for the modern app. All behind a single `dump proxy --virtualize` process.

---

## Why it matters

The “legacy–modern gap” is really a **streaming and integrity** gap. Once you can stream X12 like JSON and seal outputs with PQC, “legacy” is just another input type. We wanted a single tool where the same binary does `dump stress` (generate a 1.2GB X12 file), `dump scan` (find it), `dump map` (tame it with PII masking), and `dump audit verify` (prove it’s unchanged). That’s the demo we ship in `docs/DEMO_COMMANDS.sh`.

---

## Tech stack

- **Go** CLI (Cobra); optional **Rust** core via cgo for mapping and Dilithium2.
- **Ollama** for inference; **Vericore** (MMR + Dilithium2) for sealing and audit.
- **Tauri v2** desktop app for mapping graph, verification dropzone, and live preview.
- Healthcare: embedded X12 837/835 and HL7 v2.5 dialects; FHIR Bundle streaming in/out.

Repo: [github.com/SL1C3D-L4BS/dump](https://github.com/SL1C3D-L4BS/dump)  
Docs: `docs/README.md` (Launch Manifest), `PERFORMANCE.md` (1.2GB stress test), `docs/DEMO_COMMANDS.sh` (copy-paste terminal demo).

We’re happy to answer questions and hear how you’re bridging legacy and modern data.
