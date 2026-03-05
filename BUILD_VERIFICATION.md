# Build Verification Summary

This document records the verification and validation of the full DUMP build (Rust core + Go CLI/API).

## Verified (as of last run)

| Step | Command / check | Status |
|------|-----------------|--------|
| **Pure Go CLI build** | `CGO_ENABLED=0 go build -o dump-go .` | ✅ Pass |
| **CLI help** | `./dump-go --help` | ✅ Pass |
| **Map (JSONL)** | `./dump-go map demo/legacy_crm.jsonl --schema=demo/crm_schema.yaml --format=jsonl` | ✅ Pass (7 rows) |
| **Map (Parquet + seal)** | `./dump-go map demo/... --format=parquet --output=...` | ✅ Pass (Vericore Seal in stderr) |
| **Go modules** | `go mod verify` | ✅ Pass (all modules verified) |
| **Rust unit tests** | `cargo test --manifest-path internal/core-rs/Cargo.toml` | ✅ Pass (mapper + crypto tests) |
| **Rust release build** | `cargo build --release --manifest-path internal/core-rs/Cargo.toml` | ✅ Pass |
| **API build** | `CGO_ENABLED=0 go build -o dump-api ./api` | ✅ Pass |
| **API server start** | `./dump-api` (Fiber on :8080) | ✅ Pass |

## CGO / Rust-linked build (optional)

Linking the Go CLI (or API) against the Rust static library requires:

1. **Rust artifact**  
   After `cargo build --release` in `internal/core-rs/`, the linker expects either:
   - `internal/core-rs/target/release/libdump_core.a` (static), or  
   - `internal/core-rs/target/release/libdump_core.dylib` (macOS dynamic).

   On some systems Cargo may not produce the `.a` file for the top-level crate; if the artifact is missing, use the pure Go build (see below).  
**Note:** If you use a custom `CARGO_TARGET_DIR` or a global Cargo target, run Rust build from the repo so the artifact is under `internal/core-rs/target/release/`. The Makefile sets `CARGO_TARGET_DIR` to that path so the Go linker finds the lib.

2. **Build with cgo**  
   When the Rust lib is present:
   - CLI: `make` or `CGO_ENABLED=1 go build -o dump -tags cgo .`
   - API: `make api` or `CGO_ENABLED=1 go build -o dump-api -tags cgo ./api`

3. **Run verification script**  
   From repo root:
   ```bash
   ./scripts/verify_build.sh
   ```
   This runs Rust tests, Rust release build, pure Go build, map commands, and (if the Rust lib exists) the cgo CLI build and map.

## Pure Go (no Rust) workflow

- **CLI:** `make go-only` or `CGO_ENABLED=0 go build -o dump .`
- **API:** `CGO_ENABLED=0 go build -o dump-api ./api`

All mapping and signing use the Go implementation (gjson/sjson and go-pq-mmr).

## Summary

- **Pure Go path:** Fully verified (build, CLI, map to JSONL/Parquet, seal, API).
- **Rust path:** Rust crate builds and tests pass; cgo link succeeds only when `libdump_core.a` or `libdump_core.dylib` is present in `internal/core-rs/target/release/`.
