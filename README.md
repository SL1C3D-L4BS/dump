# 🗑️ DUMP (Data Universal Mapping Platform)

**AI-Assisted Schema Inference and Hyperspeed Data Mapping.**

[![Go Version](https://img.shields.io/github/go-mod/go-version/vericore/dump)](https://golang.org/)
[![Vericore Integrity](https://img.shields.io/badge/PQC-Secured-8A2BE2)](#)

---

## 🔒 **100% Air-Gapped & Privacy-First**

**DUMP is powered entirely by local Ollama models.** No cloud APIs, no telemetry, no data ever leaves your machine. Schema inference and mapping run on your own hardware—unicorn-tier security for sensitive data pipelines.

---

The "Data Formatting Tax" is dead. DUMP is a high-performance CLI designed to instantly map disparate data formats (JSON, Parquet, SQL, Protobuf) using AI-inferred schemas.

Instead of writing hundreds of lines of boilerplate parsing logic, you provide DUMP with a sample file. It uses a **local LLM (Ollama)** to infer the semantic intent, generates a highly optimized YAML mapping manifest, and compiles it into a streaming data pipeline.

## 📥 Installation

- **Pre-compiled binaries:** Download the latest release for your OS and architecture from the [Releases](https://github.com/vericore/dump/releases) page. Binaries are built for Linux (amd64/arm64), macOS (Intel & Apple Silicon), and Windows (amd64).
- **From source:**  
  `go install github.com/vericore/dump@latest`

## 🚀 The workflow

1. **Infer:** `dump infer source.json --target=protobuf > schema.yaml`
2. **Map:** `dump map source.json --schema=schema.yaml > output.pb`
3. **Verify:** Every mapped file is cryptographically signed using the Vericore Post-Quantum MMR.

## ⚡ Architecture

* **CLI Engine:** Built on Go (`spf13/cobra`) for instantaneous startup and concurrent data streaming.
* **AI Inference Layer:** **Ollama-only.** Local LLMs infer schemas, map nested fields, unroll arrays, and cast types—no cloud dependency.
* **Zero-Copy Streaming:** Chunked readers process multi-gigabyte files without blowing up RAM.
