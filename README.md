# 🗑️ DUMP (Data Universal Mapping Platform)

**AI-Assisted Schema Inference and Hyperspeed Data Mapping.**

[![Go Version](https://img.shields.io/github/go-mod/go-version/vericore/dump)](https://golang.org/)
[![Vericore Integrity](https://img.shields.io/badge/PQC-Secured-8A2BE2)](#)

The "Data Formatting Tax" is dead. DUMP is a high-performance CLI designed to instantly map disparate data formats (JSON, Parquet, SQL, Protobuf) using AI-inferred schemas. 

Instead of writing hundreds of lines of boilerplate parsing logic, you provide DUMP with a sample file. It uses an LLM to infer the semantic intent, generates a highly optimized YAML mapping manifest, and compiles it into a streaming data pipeline.

## 🚀 The workflow
1. **Infer:** `dump infer source.json --target=protobuf > schema.yaml`
2. **Map:** `dump map source.json --schema=schema.yaml > output.pb`
3. **Verify:** Every mapped file is cryptographically signed using the Vericore Post-Quantum MMR.

## ⚡ Architecture
* **CLI Engine:** Built on Go (`spf13/cobra`) for instantaneous startup and concurrent data streaming.
* **AI Inference Layer:** Interfaces with local (Ollama) or cloud (OpenAI/Anthropic) LLMs to intelligently map nested fields, unroll arrays, and cast types.
* **Zero-Copy Streaming:** Utilizes chunked readers to process multi-gigabyte files without blowing up RAM.
