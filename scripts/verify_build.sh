#!/usr/bin/env bash
# Full verification and validation of the DUMP build (Rust core + Go CLI/API).
set -e
cd "$(dirname "$0")/.."
ROOT="$PWD"
FAILED=0

run() {
  echo "--- $*"
  if "$@"; then echo "OK"; else echo "FAIL"; FAILED=1; fi
}

echo "========== 1. Rust core: test =========="
run cargo test --manifest-path internal/core-rs/Cargo.toml 2>&1

echo "========== 2. Rust core: release build =========="
run env CARGO_TARGET_DIR="$ROOT/internal/core-rs/target" cargo build --release --manifest-path internal/core-rs/Cargo.toml 2>&1

echo "========== 3. Rust artifact check =========="
RUST_LIB=""
for f in internal/core-rs/target/release/libdump_core.a internal/core-rs/target/release/libdump_core.dylib; do
  if [ -f "$f" ]; then RUST_LIB="$f"; break; fi
done
if [ -n "$RUST_LIB" ]; then
  echo "Rust lib found: $RUST_LIB"
  echo "========== 4. Go CLI (cgo / Rust) =========="
  run env CGO_ENABLED=1 go build -o dump -tags cgo . 2>&1
  if [ -f ./dump ]; then
    run ./dump --help 2>&1
    echo "========== 5. Go CLI map (Rust path) =========="
    run ./dump map demo/legacy_crm.jsonl --schema=test_schema.yaml --format=jsonl 2>&1 | head -5
  fi
else
  echo "No Rust lib (libdump_core.a or .dylib) in internal/core-rs/target/release/; skipping cgo build."
fi

echo "========== 6. Pure Go build =========="
run env CGO_ENABLED=0 go build -o dump-go . 2>&1
run ./dump-go --help 2>&1

echo "========== 7. Pure Go map =========="
run ./dump-go map demo/legacy_crm.jsonl --schema=demo/crm_schema.yaml --format=jsonl 2>&1 | head -5

echo "========== 8. Go API build =========="
run env CGO_ENABLED=0 go build -o dump-api ./api 2>&1
echo "API binary built (run ./dump-api to start server)."

echo "========== 9. Go modules =========="
run go mod verify 2>&1

if [ "$FAILED" -eq 0 ]; then
  echo ""
  echo "All verification steps passed."
else
  echo ""
  echo "Some steps failed."
  exit 1
fi
