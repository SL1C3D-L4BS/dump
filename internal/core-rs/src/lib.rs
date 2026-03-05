//! DUMP Rust Performance Core: mapping hot-loop and PQC signing.
//! Exposes C-compatible FFI for use from Go via cgo, and Rust API for Tauri/native.

mod arrow_ipc;
mod crypto;
mod mapper;

pub use arrow_ipc::{map_rows_to_arrow_buffer, map_rows_to_arrow_buffer_with_schema_json};
pub use crypto::{
    rust_sign_file, rust_sign_file_with_keys, rust_sign_free,
    rust_verify_file, rust_verify_free, verify_file_impl as verify_file_rs,
    rotate_keys_rs,
};
pub use mapper::{rust_map_row, rust_map_row_free, rust_map_set_schema, map_row_rs, set_schema_rs};
