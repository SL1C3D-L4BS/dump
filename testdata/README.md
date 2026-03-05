# DUMP testdata

Sample files and schemas for validation.

**Fan-out:** See `fanout.yaml` for a multi-target config. Run `dump fanout --config testdata/fanout.yaml` (after `go mod tidy` to fetch AWS, Prometheus, and Elasticsearch SDKs).

## Files

- **sample.jsonl** – JSON Lines (id, name, role)
- **sample.csv** – CSV with header row
- **sample.xml** – XML with repeating `<Record>` blocks
- **sample.hl7** – HL7 ADT messages (MSH, EVN, PID)
- **schema_passthrough.yaml** – Maps id, name, role (for JSONL/CSV/XML)
- **schema_xml.yaml** – Maps Record.id, Record.name, Record.role
- **schema_edi.yaml** – Maps MSH.DateTime, PID.PatientName, PID.DateTimeOfBirth

## Validation commands

```bash
# Map (default build uses Rust/cgo; use -tags='!cgo' for pure Go)
dump map testdata/sample.jsonl --schema=testdata/schema_passthrough.yaml --format=jsonl
dump map testdata/sample.csv --schema=testdata/schema_passthrough.yaml --format=jsonl
dump map testdata/sample.xml --schema=testdata/schema_xml.yaml --input-type=xml --xml-block=Record --format=jsonl
dump map testdata/sample.hl7 --schema=testdata/schema_edi.yaml --input-type=edi --dialect=internal/dialects/hl7_adt.yaml --format=jsonl

# Analyze (requires Ollama for full run)
dump analyze testdata/sample.jsonl --target=parquet

# Proxy (requires upstream and schema)
dump proxy --upstream http://example.com/api --schema=testdata/schema_xml.yaml --port=8081
```

## Notes

- **EDI**: With the **Go mapper** (build with `-tags='!cgo'`), EDI nested output and schema paths like `MSH.DateTime` and `PID.PatientName` work. The default cgo build uses the Rust core, which may handle nested paths differently.
- **Format detection**: `dump analyze` peeks at the file and prints `Format detected: jsonl|csv|xml|edi|unknown`.
