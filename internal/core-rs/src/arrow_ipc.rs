//! Map JSONL rows to Arrow IPC stream (zero-copy buffer for frontend).

use arrow_array::{Array, Float64Array, Int64Array, RecordBatch, StringArray};
use arrow_ipc::writer::StreamWriter;
use arrow_schema::{DataType, Field, Schema};
use serde_json::Value;
use std::sync::Arc;

use crate::mapper::{map_row_impl, Schema as MappingSchema};

/// Map a batch of JSON lines using schema JSON and return Arrow IPC stream bytes.
pub fn map_rows_to_arrow_buffer_with_schema_json(
    schema_json: &str,
    json_lines: &[String],
) -> Result<Vec<u8>, String> {
    let schema: MappingSchema =
        serde_json::from_str(schema_json).map_err(|e| e.to_string())?;
    map_rows_to_arrow_buffer(&schema, json_lines)
}

/// Map a batch of JSON lines using the given schema and return Arrow IPC stream bytes.
pub fn map_rows_to_arrow_buffer(
    schema: &MappingSchema,
    json_lines: &[String],
) -> Result<Vec<u8>, String> {
    if json_lines.is_empty() {
        return Ok(vec![]);
    }

    let mut mapped: Vec<serde_json::Map<String, Value>> = Vec::with_capacity(json_lines.len());
    for line in json_lines {
        let line = line.trim();
        if line.is_empty() {
            continue;
        }
        let out = map_row_impl(line, schema)?;
        let row: Value = serde_json::from_str(&out).map_err(|e| e.to_string())?;
        if let Some(obj) = row.as_object() {
            mapped.push(obj.clone());
        }
    }

    if mapped.is_empty() {
        return Ok(vec![]);
    }

    let mut fields: Vec<Field> = Vec::with_capacity(schema.rules.len());
    let mut columns: Vec<Arc<dyn Array>> = Vec::with_capacity(schema.rules.len());
    let n = mapped.len();

    for rule in &schema.rules {
        let (field, array) = build_column(&rule.target_field, &rule.typ, &mapped, n)?;
        fields.push(field);
        columns.push(array);
    }

    let arrow_schema = Arc::new(Schema::new(fields));
    let batch = RecordBatch::try_new(arrow_schema.clone(), columns)
        .map_err(|e| e.to_string())?;

    let mut buf = Vec::new();
    let mut writer = StreamWriter::try_new(&mut buf, &batch.schema()).map_err(|e| e.to_string())?;
    writer.write(&batch).map_err(|e| e.to_string())?;
    writer.finish().map_err(|e| e.to_string())?;
    drop(writer);
    Ok(buf)
}

fn build_column(
    name: &str,
    typ: &str,
    rows: &[serde_json::Map<String, Value>],
    _n: usize,
) -> Result<(Field, Arc<dyn Array>), String> {
    let (field, arr): (Field, Arc<dyn Array>) = match typ.to_lowercase().as_str() {
        "number" | "integer" | "int" => {
            let values: Vec<Option<i64>> = rows
                .iter()
                .map(|r| r.get(name).and_then(|v| v.as_i64().or_else(|| v.as_f64().map(|f| f as i64))))
                .collect();
            let arr = Int64Array::from(values);
            (Field::new(name, DataType::Int64, true), Arc::new(arr))
        }
        "float" | "double" => {
            let values: Vec<Option<f64>> = rows
                .iter()
                .map(|r| r.get(name).and_then(|v| v.as_f64().or_else(|| v.as_i64().map(|i| i as f64))))
                .collect();
            let arr = Float64Array::from(values);
            (Field::new(name, DataType::Float64, true), Arc::new(arr))
        }
        _ => {
            let values: Vec<Option<&str>> = rows
                .iter()
                .map(|r| r.get(name).and_then(|v| v.as_str()))
                .collect();
            let strings: Vec<Option<String>> = values
                .into_iter()
                .map(|o| o.map(|s| s.to_string()))
                .collect();
            let arr = StringArray::from(strings);
            (Field::new(name, DataType::Utf8, true), Arc::new(arr))
        }
    };
    Ok((field, arr))
}
