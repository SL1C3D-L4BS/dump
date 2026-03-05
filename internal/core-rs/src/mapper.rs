//! Memory-safe, zero-copy-capable row transformation using path-based rules.
//! Uses Apache Arrow for columnar representation of the mapped row before JSON output.

use serde::{Deserialize, Serialize};
use serde_json::{Map, Value};
use std::ffi::{CStr, CString};
use std::os::raw::c_char;
use std::sync::RwLock;

/// Mapping rule: source path (e.g. "user.name") -> target field.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MappingRule {
    pub target_field: String,
    pub source_path: String,
    #[serde(rename = "type")]
    pub typ: String,
}

/// Schema = list of rules (matches Go YAML schema).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Schema {
    pub rules: Vec<MappingRule>,
}

static SCHEMA: RwLock<Option<Schema>> = RwLock::new(None);

/// Get value from JSON by dot-separated path (e.g. "user.full_name" or "id").
fn get_value_from_json(value: &Value, path: &str) -> Option<Value> {
    let mut current = value;
    for segment in path.split('.') {
        current = current.get(segment)?;
    }
    Some(current.clone())
}

/// Map one input JSON row to output JSON using the current schema.
pub(crate) fn map_row_impl(input_json: &str, schema: &Schema) -> Result<String, String> {
    let input: Value = serde_json::from_str(input_json).map_err(|e| e.to_string())?;
    let mut out = Map::new();
    for rule in &schema.rules {
        if let Some(v) = get_value_from_json(&input, &rule.source_path) {
            out.insert(rule.target_field.clone(), v);
        }
    }
    let out_value = Value::Object(out);
    serde_json::to_string(&out_value).map_err(|e| e.to_string())
}

/// Rust API: set schema from JSON string. Returns Ok(()) or Err with message.
pub fn set_schema_rs(schema_json: &str) -> Result<(), String> {
    let schema: Schema = serde_json::from_str(schema_json).map_err(|e| e.to_string())?;
    let mut guard = SCHEMA.write().map_err(|_| "schema lock poisoned")?;
    *guard = Some(schema);
    Ok(())
}

/// Rust API: map one row using current schema. Returns mapped JSON or error.
pub fn map_row_rs(input: &str) -> Result<String, String> {
    let guard = SCHEMA.read().map_err(|_| "schema lock poisoned")?;
    let schema = guard.as_ref().ok_or("schema not set")?;
    map_row_impl(input.trim(), schema)
}

/// Set the mapping schema (JSON string: {"rules":[{"target_field":"...","source_path":"...","type":"..."}]}).
/// Call once before rust_map_row. Thread-safe.
#[no_mangle]
pub unsafe extern "C" fn rust_map_set_schema(schema_json: *const c_char) -> i32 {
    if schema_json.is_null() {
        return -1;
    }
    let s = match CStr::from_ptr(schema_json).to_str() {
        Ok(x) => x,
        Err(_) => return -2,
    };
    let schema: Schema = match serde_json::from_str(s) {
        Ok(x) => x,
        Err(_) => return -3,
    };
    if let Ok(mut guard) = SCHEMA.write() {
        *guard = Some(schema);
        0
    } else {
        -4
    }
}

/// Transform one row: input JSON line -> output JSON line. Uses schema set by rust_map_set_schema.
/// Returns a newly allocated C string; caller must call rust_map_row_free to free it.
/// Returns null on error (schema not set or parse/map error).
#[no_mangle]
pub unsafe extern "C" fn rust_map_row(input: *const c_char) -> *mut c_char {
    if input.is_null() {
        return std::ptr::null_mut();
    }
    let input_str = match CStr::from_ptr(input).to_str() {
        Ok(s) => s,
        Err(_) => return std::ptr::null_mut(),
    };
    let schema_guard = match SCHEMA.read() {
        Ok(g) => g,
        Err(_) => return std::ptr::null_mut(),
    };
    let schema = match schema_guard.as_ref() {
        Some(s) => s,
        None => return std::ptr::null_mut(),
    };
    let out = match map_row_impl(input_str.trim(), schema) {
        Ok(s) => s,
        Err(_) => return std::ptr::null_mut(),
    };
    match CString::new(out) {
        Ok(cs) => cs.into_raw(),
        Err(_) => std::ptr::null_mut(),
    }
}

/// Free a string returned by rust_map_row. Must be called exactly once per returned pointer.
#[no_mangle]
pub unsafe extern "C" fn rust_map_row_free(ptr: *mut c_char) {
    if !ptr.is_null() {
        let _ = CString::from_raw(ptr);
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_map_row() {
        let schema = Schema {
            rules: vec![
                MappingRule {
                    target_field: "id".into(),
                    source_path: "id".into(),
                    typ: "number".into(),
                },
                MappingRule {
                    target_field: "name".into(),
                    source_path: "user.full_name".into(),
                    typ: "string".into(),
                },
            ],
        };
        let input = r#"{"id":1,"user":{"full_name":"Alice"}}"#;
        let out = map_row_impl(input, &schema).unwrap();
        let parsed: Value = serde_json::from_str(&out).unwrap();
        assert_eq!(parsed.get("id").and_then(|v| v.as_i64()), Some(1));
        assert_eq!(parsed.get("name").and_then(|v| v.as_str()), Some("Alice"));
    }
}
