//! Phase 11: Zero-Copy Reactor & Agentic Orchestration.

use dump_core::{
    map_row_rs, map_rows_to_arrow_buffer_with_schema_json, set_schema_rs, verify_file_rs,
};
use serde::{Deserialize, Serialize};
use std::path::Path;

const OLLAMA_BASE: &str = "http://localhost:11434";

// ---------- Mapping (dump-core) ----------

#[tauri::command]
fn map_set_schema(schema_json: String) -> Result<(), String> {
    set_schema_rs(&schema_json)
}

#[tauri::command]
fn map_row(input: String) -> Result<String, String> {
    map_row_rs(&input)
}

/// Zero-copy: map JSONL rows to Arrow IPC stream; returns base64-encoded bytes for frontend.
#[tauri::command]
fn map_rows_to_arrow(schema_json: String, json_lines: Vec<String>) -> Result<String, String> {
    let buf =
        map_rows_to_arrow_buffer_with_schema_json(&schema_json, &json_lines)?;
    Ok(base64::Engine::encode(
        &base64::engine::general_purpose::STANDARD,
        &buf,
    ))
}

// ---------- PQC verification ----------

#[tauri::command]
fn verify_parquet_with_seal(
    parquet_path: String,
    seal_path: String,
    keys_path: String,
) -> Result<VerifyResult, String> {
    let seal_content =
        std::fs::read_to_string(&seal_path).map_err(|e| format!("read seal: {}", e))?;
    let verified = verify_file_rs(&parquet_path, &seal_content, &keys_path).unwrap_or(false);
    Ok(VerifyResult {
        status: if verified { "verified" } else { "tampered" }.to_string(),
        verified,
    })
}

#[derive(Serialize, Deserialize)]
pub struct VerifyResult {
    pub status: String,
    pub verified: bool,
}

// ---------- Source Discovery ----------

#[derive(Serialize, Deserialize)]
pub struct DirEntry {
    pub name: String,
    pub path: String,
    pub is_dir: bool,
}

#[tauri::command]
fn discover_list_dir(path: String) -> Result<Vec<DirEntry>, String> {
    let mut out = Vec::new();
    for e in std::fs::read_dir(&path).map_err(|e| e.to_string())? {
        let e = e.map_err(|e| e.to_string())?;
        let name = e.file_name().to_string_lossy().into_owned();
        let path_buf = e.path();
        let path_str = path_buf.to_string_lossy().into_owned();
        let is_dir = e.file_type().map_err(|e| e.to_string())?.is_dir();
        out.push(DirEntry {
            name,
            path: path_str,
            is_dir,
        });
    }
    out.sort_by(|a, b| a.name.to_lowercase().cmp(&b.name.to_lowercase()));
    Ok(out)
}

fn is_postgres(url: &str) -> bool {
    let s = url.trim();
    s.starts_with("postgres://") || s.starts_with("postgresql://")
}

#[tauri::command]
async fn discover_ping_db(db_url: String) -> Result<bool, String> {
    let db_url = db_url.trim().to_string();
    if is_postgres(&db_url) {
        tokio::task::spawn_blocking(move || {
            let u = url::Url::parse(&db_url).map_err(|e| e.to_string())?;
            let host = u.host_str().unwrap_or("localhost");
            let port = u.port().unwrap_or(5432);
            let addr: std::net::SocketAddr = format!("{}:{}", host, port)
                .parse()
                .map_err(|_| "invalid host:port")?;
            std::net::TcpStream::connect_timeout(&addr, std::time::Duration::from_secs(2))
            .map(|_| true)
            .map_err(|e| e.to_string())
        })
        .await
        .map_err(|e| e.to_string())?
    } else {
        let path = db_url
            .trim_start_matches("file:")
            .trim_start_matches("sqlite:")
            .to_string();
        tokio::task::spawn_blocking(move || {
            let conn = rusqlite::Connection::open(&path).map_err(|e| e.to_string())?;
            conn.execute("SELECT 1", []).map_err(|e| e.to_string())?;
            Ok(true)
        })
        .await
        .map_err(|e| e.to_string())?
    }
}

#[tauri::command]
fn discover_sql_tables(db_url: String) -> Result<Vec<String>, String> {
    let db_url = db_url.trim();
    if is_postgres(db_url) {
        return Err("Postgres table list not implemented in this build".into());
    }
    let path = db_url
        .trim_start_matches("file:")
        .trim_start_matches("sqlite:");
    let conn = rusqlite::Connection::open(path).map_err(|e| e.to_string())?;
    let mut stmt = conn
        .prepare("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
        .map_err(|e| e.to_string())?;
    let rows = stmt
        .query_map([], |r| r.get(0))
        .map_err(|e| e.to_string())?;
    let mut out = Vec::new();
    for row in rows {
        out.push(row.map_err(|e| e.to_string())?);
    }
    Ok(out)
}

#[tauri::command]
fn discover_sql_headers(db_url: String, table: String) -> Result<Vec<String>, String> {
    let db_url = db_url.trim();
    if is_postgres(db_url) {
        return Err("Postgres headers not implemented in this build".into());
    }
    let path = db_url
        .trim_start_matches("file:")
        .trim_start_matches("sqlite:");
    let conn = rusqlite::Connection::open(path).map_err(|e| e.to_string())?;
    let query = format!("SELECT * FROM \"{}\" LIMIT 0", table.replace('"', "\"\""));
    let stmt = conn.prepare(&query).map_err(|e| e.to_string())?;
    let names: Vec<String> = stmt.column_names().iter().map(|s| (*s).to_string()).collect();
    Ok(names)
}

#[tauri::command]
fn discover_csv_headers(file_path: String) -> Result<Vec<String>, String> {
    let mut rdr = csv::Reader::from_path(&file_path).map_err(|e| e.to_string())?;
    let headers = rdr
        .headers()
        .map_err(|e| e.to_string())?
        .iter()
        .map(|s| s.to_string())
        .collect();
    Ok(headers)
}

/// Read first n lines from a CSV file as JSONL (one JSON object per row).
#[tauri::command]
fn read_csv_sample(file_path: String, limit: u32) -> Result<Vec<String>, String> {
    let mut rdr = csv::Reader::from_path(&file_path).map_err(|e| e.to_string())?;
    let headers = rdr.headers().map_err(|e| e.to_string())?.clone();
    let mut out = Vec::new();
    let limit = limit.min(100) as usize;
    for (i, result) in rdr.records().enumerate() {
        if i >= limit {
            break;
        }
        let record = result.map_err(|e| e.to_string())?;
        let mut obj = serde_json::Map::new();
        for (j, h) in headers.iter().enumerate() {
            let v = record.get(j).unwrap_or("").to_string();
            obj.insert(h.to_string(), serde_json::Value::String(v));
        }
        out.push(serde_json::to_string(&obj).map_err(|e| e.to_string())?);
    }
    Ok(out)
}

// ---------- Ollama ----------

#[derive(Serialize, Deserialize)]
struct OllamaModel {
    name: String,
}

#[derive(Serialize, Deserialize)]
struct OllamaListResponse {
    models: Option<Vec<OllamaModel>>,
}

#[tauri::command]
async fn ollama_list_models() -> Result<Vec<String>, String> {
    let client = reqwest::Client::new();
    let resp = client
        .get(format!("{}/api/tags", OLLAMA_BASE))
        .send()
        .await
        .map_err(|e| e.to_string())?;
    let list: OllamaListResponse = resp.json().await.map_err(|e| e.to_string())?;
    Ok(list
        .models
        .unwrap_or_default()
        .into_iter()
        .map(|m| m.name)
        .collect())
}

#[derive(Serialize, Deserialize)]
struct OllamaGenerateRequest {
    model: String,
    prompt: String,
    stream: bool,
}

#[derive(Serialize, Deserialize)]
struct OllamaGenerateResponse {
    response: Option<String>,
}

#[tauri::command]
async fn ollama_generate(model: String, prompt: String) -> Result<String, String> {
    let client = reqwest::Client::new();
    let body = OllamaGenerateRequest {
        model,
        prompt,
        stream: false,
    };
    let resp = client
        .post(format!("{}/api/generate", OLLAMA_BASE))
        .json(&body)
        .send()
        .await
        .map_err(|e| e.to_string())?;
    let out: OllamaGenerateResponse = resp.json().await.map_err(|e| e.to_string())?;
    Ok(out.response.unwrap_or_default())
}

/// Agentic graph: send column headers to Ollama, get back JSON { nodes, edges } for React Flow.
#[tauri::command]
async fn ollama_generate_mapping(model: String, headers: Vec<String>) -> Result<String, String> {
    let headers_str = headers.join(", ");
    let prompt = format!(
        r#"You are a data mapping assistant. Given these source columns: [{}].
Return ONLY a valid JSON object (no markdown, no explanation) with this exact structure:
{{ "nodes": [ {{ "id": "source", "type": "input", "position": {{ "x": 0, "y": 0 }}, "data": {{ "label": "Source" }} }}, {{ "id": "dest", "type": "output", "position": {{ "x": 300, "y": 0 }}, "data": {{ "label": "Parquet" }} }}, ... ], "edges": [ {{ "id": "e1", "source": "source", "target": "dest" }}, ... ] }}
Add one node per column as needed and edges from source columns to the Parquet node. Use simple ids like "col_0", "col_1" for column nodes."#,
        headers_str
    );
    ollama_generate(model, prompt).await
}

// ---------- Audit (Integrity Stream) ----------

#[derive(Serialize, Deserialize)]
pub struct ParquetWithSeal {
    pub parquet_path: String,
    pub seal_path: String,
}

#[tauri::command]
fn audit_list_parquet_with_seal(dir_path: String) -> Result<Vec<ParquetWithSeal>, String> {
    let mut out = Vec::new();
    for e in std::fs::read_dir(&dir_path).map_err(|e| e.to_string())? {
        let e = e.map_err(|e| e.to_string())?;
        let path = e.path();
        if path.is_file() {
            if let Some(ext) = path.extension() {
                if ext == "parquet" {
                    let parquet_path = path.to_string_lossy().into_owned();
                    let seal_path = format!("{}.vericore-seal", parquet_path);
                    if Path::new(&seal_path).exists() {
                        out.push(ParquetWithSeal {
                            parquet_path,
                            seal_path,
                        });
                    }
                }
            }
        }
    }
    out.sort_by(|a, b| a.parquet_path.cmp(&b.parquet_path));
    Ok(out)
}

// ---------- Key Management ----------

#[derive(Serialize, Deserialize)]
pub struct KeysInfo {
    pub exists: bool,
    pub public_key_hex: Option<String>,
}

#[tauri::command]
fn keys_read(keys_path: String) -> Result<KeysInfo, String> {
    let path = Path::new(&keys_path);
    if !path.exists() {
        return Ok(KeysInfo {
            exists: false,
            public_key_hex: None,
        });
    }
    let content = std::fs::read_to_string(path).map_err(|e| e.to_string())?;
    let keys: serde_json::Value = serde_json::from_str(&content).map_err(|e| e.to_string())?;
    let pk = keys
        .get("public_key_hex")
        .and_then(|v| v.as_str())
        .map(String::from);
    Ok(KeysInfo {
        exists: true,
        public_key_hex: pk,
    })
}

#[tauri::command]
fn keys_rotate(keys_path: String) -> Result<String, String> {
    dump_core::rotate_keys_rs(&keys_path)
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_opener::init())
        .invoke_handler(tauri::generate_handler![
            map_set_schema,
            map_row,
            map_rows_to_arrow,
            verify_parquet_with_seal,
            discover_list_dir,
            discover_ping_db,
            discover_sql_tables,
            discover_sql_headers,
            discover_csv_headers,
            read_csv_sample,
            ollama_list_models,
            ollama_generate,
            ollama_generate_mapping,
            audit_list_parquet_with_seal,
            keys_read,
            keys_rotate,
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
