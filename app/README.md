# Frontier Workspace (Phase 10 + 11)

Tauri v2 native desktop app: zero-copy Arrow IPC, source discovery, agentic mapping, audit ledger, key management.

## Stack

- **Frontend:** React, TypeScript, Vite, Tailwind CSS, Radix UI, React Flow, apache-arrow
- **Backend:** Rust (`src-tauri`) linked to `internal/core-rs` (mapping, Arrow IPC, Dilithium2)

## Commands (IPC)

| Command | Description |
|--------|-------------|
| `map_set_schema` | Set mapping schema (JSON rules) |
| `map_row` | Map one JSON row |
| `map_rows_to_arrow` | Map JSONL batch to Arrow IPC; returns base64-encoded buffer |
| `verify_parquet_with_seal` | Verify `.parquet` with `.vericore-seal` |
| `discover_list_dir` | List directory (path) |
| `discover_ping_db` | Ping DB (SQLite/Postgres URL) |
| `discover_sql_tables` | List tables (SQLite) |
| `discover_sql_headers` | Get column names for a table (SQLite) |
| `discover_csv_headers` | Get CSV column headers from file |
| `ollama_list_models` | List Ollama models |
| `ollama_generate` | Generate completion |
| `ollama_generate_mapping` | Agentic: headers → JSON { nodes, edges } for React Flow |
| `audit_list_parquet_with_seal` | List .parquet + .vericore-seal in directory |
| `keys_read` | Read keys file (public key, exists) |
| `keys_rotate` | Generate new keypair and write to path |

## Run

```bash
pnpm install
pnpm tauri dev
```

Build:

```bash
pnpm tauri build
```

## PQC Verification

1. Enter the path to your keys file (e.g. `~/.config/vericore/keys.json`) or leave the default.
2. Drag and drop a `.parquet` file **and** its `.vericore-seal` onto the dropzone (or onto the window).
3. The UI shows **PQC Verified** (green) or **Tampered** (red).

## Local AI (Ollama)

- Ensure [Ollama](https://ollama.com) is running on `localhost:11434`.
- Use "Refresh models" to load models, then select a model and run "Generate".

## Phase 11 features

- **Zero-copy Arrow:** `map_rows_to_arrow(schema_json, json_lines)` returns base64 Arrow IPC; frontend uses `apache-arrow` to render tables.
- **Source Explorer:** Browse directories, ping SQLite/Postgres, load table or CSV headers. "Suggest mapping (Ollama)" sends headers to Ollama and draws the returned nodes/edges on the React Flow canvas.
- **Audit tab:** Pick a directory; list all `.parquet` with `.vericore-seal`, then "Verify all" for a real-time Integrity Stream (PQC Verified / Tampered).
- **Keys tab:** View and rotate Dilithium2 keys at `~/.config/vericore/keys.json`.
