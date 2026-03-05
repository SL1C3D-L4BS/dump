# DUMP (Data Universal Mapping Platform)

**AI-assisted schema inference and high-performance data mapping.** Map JSON, Parquet, SQL, CSV, and more using local Ollama for inference and a Rust core for zero-copy streaming and PQC integrity.

[![Go Version](https://img.shields.io/github/go-mod/go-version/SL1C3D-L4BS/dump)](https://golang.org/)
[![PQC-Secured](https://img.shields.io/badge/PQC-Secured-8A2BE2)](#)

---

## Privacy-first

DUMP uses **local Ollama** for schema inference and mapping suggestions. No cloud APIs or telemetry; data stays on your machine.

---

## What’s in the repo

* **CLI** (`cmd/`) — Infer schemas, map data, verify sealed Parquet (Go + optional Rust via cgo).
* **Rust core** (`internal/core-rs/`) — Mapping engine, Vericore PQC (Dilithium2), Arrow IPC.
* **API** (`api/`) — Fiber HTTP server: POST `/map`, verification, Ollama discovery.
* **Desktop app** (`app/`) — Tauri v2 + React + Vite: mapping graph, verification dropzone, Ollama panel, live data preview.

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
* **Desktop app:**  
  From repo root, build Rust core first, then:
  ```bash
  cd app && pnpm install && pnpm tauri build
  ```
  Run in dev: `pnpm tauri dev`

---

## Workflow

1. **Infer:** `dump infer source.json --target=parquet > schema.yaml`
2. **Map:** `dump map source.json --schema=schema.yaml --format=parquet > output.parquet`
3. **Verify:** Use the CLI or the desktop app to verify Parquet files sealed with the Vericore PQC MMR.

---

## Architecture

* **CLI:** Go (Cobra), optional cgo link to Rust for mapping and verification.
* **Rust core:** Schema application, row mapping, Arrow IPC, Dilithium2 signing/verification.
* **Inference:** Ollama-only for schema and mapping suggestions.
* **Desktop:** Tauri v2, React, Tailwind; calls into Rust core for mapping and verification.
