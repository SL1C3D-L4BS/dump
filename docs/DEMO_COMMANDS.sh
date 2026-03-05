#!/usr/bin/env bash
# SL1C3D-L4BS — Perfect terminal demo: Generate the beast, find it, tame it, prove it.
# Run from repo root. Uses a 20 MB stress file for a quick demo; use -s 1200 for full 1.2 GB.

set -e

echo "=== 1. Generate the beast (X12 837 stress file, 20 MB for quick demo) ==="
dump stress -o stress_837.x12 -s 20

echo ""
echo "=== 2. Find the beast (shadow scan) ==="
dump scan --path . --vericore-store ./vericore_ingest

echo ""
echo "=== 3. Tame the beast: map to Parquet with PII masking + Vericore seal ==="
mkdir -p vericore_ingest
dump map stress_837.x12 \
  --schema=docs/demo_schema.yaml \
  --input-type=x12 \
  --industry=healthcare \
  --format=parquet \
  --output=vericore_ingest/stress_mapped.parquet \
  --mask=pii

echo ""
echo "=== 4. Prove the beast is unchanged (audit verify) ==="
dump audit verify --all

echo ""
echo "Done. Vericore seal was printed above; audit log at ~/.vericore/audit.db."
