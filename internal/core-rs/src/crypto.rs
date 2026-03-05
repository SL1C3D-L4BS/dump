//! PQC cryptographic kernel: MMR, Dilithium2 sign/verify, and key persistence.

use pqcrypto_dilithium::dilithium2;
use pqcrypto_traits::sign::{DetachedSignature as DetachedSignatureTrait, PublicKey, SecretKey};
use sha2::{Digest, Sha256};
use std::ffi::{CStr, CString};
use std::fs;
use std::os::raw::c_char;
use std::path::Path;

mod hex {
    const HEX: &[u8] = b"0123456789abcdef";
    pub fn encode(bytes: &[u8]) -> String {
        let mut s = String::with_capacity(bytes.len() * 2);
        for &b in bytes {
            s.push(HEX[(b >> 4) as usize] as char);
            s.push(HEX[(b & 15) as usize] as char);
        }
        s
    }
    pub fn decode(s: &str) -> Result<Vec<u8>, String> {
        let s = s.trim();
        if s.len() % 2 != 0 {
            return Err("odd hex length".into());
        }
        let mut out = Vec::with_capacity(s.len() / 2);
        for i in (0..s.len()).step_by(2) {
            let a = s.as_bytes().get(i).and_then(|&c| hex_char(c)).ok_or("invalid hex")?;
            let b = s.as_bytes().get(i + 1).and_then(|&c| hex_char(c)).ok_or("invalid hex")?;
            out.push((a << 4) | b);
        }
        Ok(out)
    }
    fn hex_char(c: u8) -> Option<u8> {
        match c {
            b'0'..=b'9' => Some(c - b'0'),
            b'a'..=b'f' => Some(c - b'a' + 10),
            b'A'..=b'F' => Some(c - b'A' + 10),
            _ => None,
        }
    }
}

#[derive(serde::Serialize, serde::Deserialize)]
struct KeysJson {
    public_key_hex: String,
    secret_key_hex: String,
}

fn hash_file(data: &[u8]) -> Vec<u8> {
    Sha256::digest(data).to_vec()
}

fn mmr_root(leaf_hashes: &[Vec<u8>]) -> Vec<u8> {
    if leaf_hashes.is_empty() {
        return Sha256::digest(b"").to_vec();
    }
    if leaf_hashes.len() == 1 {
        return leaf_hashes[0].clone();
    }
    let mut layer: Vec<Vec<u8>> = leaf_hashes.to_vec();
    while layer.len() > 1 {
        let mut next = Vec::with_capacity((layer.len() + 1) / 2);
        for chunk in layer.chunks(2) {
            let combined = if chunk.len() == 2 {
                let mut hasher = Sha256::new();
                hasher.update(&chunk[0]);
                hasher.update(&chunk[1]);
                hasher.finalize().to_vec()
            } else {
                chunk[0].clone()
            };
            next.push(combined);
        }
        layer = next;
    }
    layer.into_iter().next().unwrap_or_else(|| Sha256::digest(b"").to_vec())
}

/// Rotate keys: generate new Dilithium2 keypair, write to path, return public key hex.
pub fn rotate_keys_rs(keys_path: &str) -> Result<String, String> {
    let (pk, sk) = dilithium2::keypair();
    let path = Path::new(keys_path);
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).map_err(|e| e.to_string())?;
    }
    let keys = KeysJson {
        public_key_hex: hex::encode(pk.as_bytes()),
        secret_key_hex: hex::encode(sk.as_bytes()),
    };
    let data = serde_json::to_string_pretty(&keys).map_err(|e| e.to_string())?;
    fs::write(path, data).map_err(|e| e.to_string())?;
    Ok(keys.public_key_hex)
}

fn load_or_create_keys(keys_path: &str) -> Result<(dilithium2::PublicKey, dilithium2::SecretKey), String> {
    let path = Path::new(keys_path);
    if path.exists() {
        let data = fs::read_to_string(path).map_err(|e| e.to_string())?;
        let keys: KeysJson = serde_json::from_str(&data).map_err(|e| e.to_string())?;
        let pk_bytes = hex::decode(&keys.public_key_hex)?;
        let sk_bytes = hex::decode(&keys.secret_key_hex)?;
        let pk = dilithium2::PublicKey::from_bytes(&pk_bytes).map_err(|_| "invalid public key")?;
        let sk = dilithium2::SecretKey::from_bytes(&sk_bytes).map_err(|_| "invalid secret key")?;
        return Ok((pk, sk));
    }
    let (pk, sk) = dilithium2::keypair();
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).map_err(|e| e.to_string())?;
    }
    let keys = KeysJson {
        public_key_hex: hex::encode(pk.as_bytes()),
        secret_key_hex: hex::encode(sk.as_bytes()),
    };
    let data = serde_json::to_string_pretty(&keys).map_err(|e| e.to_string())?;
    fs::write(path, data).map_err(|e| e.to_string())?;
    Ok((pk, sk))
}

/// Parse seal string to extract PQC Sig hex line.
fn parse_seal_pqc_sig(seal: &str) -> Result<Vec<u8>, String> {
    for line in seal.lines() {
        let line = line.trim();
        if line.starts_with("PQC Sig:") {
            let hex_part = line.trim_start_matches("PQC Sig:").trim();
            return hex::decode(hex_part);
        }
    }
    Err("PQC Sig not found in seal".into())
}

/// Sign file with optional keys path. If keys_path is empty, use ephemeral key.
fn sign_result_impl(file_path: &str, keys_path: &str) -> Result<String, String> {
    let data = fs::read(file_path).map_err(|e| e.to_string())?;
    let h = hash_file(&data);
    let root = mmr_root(&[h.clone()]);

    let (pk, sk) = if keys_path.is_empty() {
        dilithium2::keypair()
    } else {
        load_or_create_keys(keys_path)?
    };

    let det_sig = dilithium2::detached_sign(&root, &sk);
    let sig_hex = hex::encode(det_sig.as_bytes());
    let _ = pk;

    let seal = format!(
        "Vericore Seal\n  MMR Root:  {}\n  PQC Sig:   {}\n  File Hash: {}",
        hex::encode(&root),
        sig_hex,
        hex::encode(&h)
    );
    Ok(seal)
}

/// Verify file against seal using public key from keys_path. Returns true if verified.
/// Public for use from Tauri/native Rust callers.
pub fn verify_file_impl(file_path: &str, seal: &str, keys_path: &str) -> Result<bool, String> {
    let data = fs::read(file_path).map_err(|e| e.to_string())?;
    let file_hash = hash_file(&data);
    let sig_bytes = parse_seal_pqc_sig(seal)?;
    let pk_bytes = if keys_path.is_empty() {
        return Err("keys path required for verification".into());
    } else {
        let key_str = fs::read_to_string(keys_path).map_err(|e| e.to_string())?;
        let keys: KeysJson = serde_json::from_str(&key_str).map_err(|e| e.to_string())?;
        hex::decode(&keys.public_key_hex)?
    };
    let pk = dilithium2::PublicKey::from_bytes(&pk_bytes).map_err(|_| "invalid public key")?;
    let sig = <dilithium2::DetachedSignature as DetachedSignatureTrait>::from_bytes(&sig_bytes)
        .map_err(|_| "invalid signature bytes")?;
    dilithium2::verify_detached_signature(&sig, &file_hash, &pk).map_err(|_| "signature verification failed")?;
    Ok(true)
}

#[no_mangle]
pub unsafe extern "C" fn rust_sign_file(path: *const c_char) -> *mut c_char {
    rust_sign_file_with_keys(path, std::ptr::null())
}

#[no_mangle]
pub unsafe extern "C" fn rust_sign_file_with_keys(path: *const c_char, keys_path: *const c_char) -> *mut c_char {
    if path.is_null() {
        return std::ptr::null_mut();
    }
    let path_str = match CStr::from_ptr(path).to_str() {
        Ok(s) => s,
        Err(_) => return std::ptr::null_mut(),
    };
    let kp = if keys_path.is_null() {
        ""
    } else {
        match CStr::from_ptr(keys_path).to_str() {
            Ok(s) => s,
            Err(_) => return std::ptr::null_mut(),
        }
    };
    let seal = match sign_result_impl(path_str, kp) {
        Ok(s) => s,
        Err(_) => return std::ptr::null_mut(),
    };
    match CString::new(seal) {
        Ok(cs) => cs.into_raw(),
        Err(_) => std::ptr::null_mut(),
    }
}

#[no_mangle]
pub unsafe extern "C" fn rust_sign_free(ptr: *mut c_char) {
    if !ptr.is_null() {
        let _ = CString::from_raw(ptr);
    }
}

/// Verify file at path against seal; keys_path must point to keys.json with public key.
/// Returns 1 if VERIFIED, 0 if TAMPERED or error. Caller can use rust_verify_free on result message.
#[no_mangle]
pub unsafe extern "C" fn rust_verify_file(path: *const c_char, seal: *const c_char, keys_path: *const c_char) -> *mut c_char {
    if path.is_null() || seal.is_null() || keys_path.is_null() {
        return result_cstring("TAMPERED");
    }
    let path_str = match CStr::from_ptr(path).to_str() {
        Ok(s) => s,
        Err(_) => return result_cstring("TAMPERED"),
    };
    let seal_str = match CStr::from_ptr(seal).to_str() {
        Ok(s) => s,
        Err(_) => return result_cstring("TAMPERED"),
    };
    let keys_str = match CStr::from_ptr(keys_path).to_str() {
        Ok(s) => s,
        Err(_) => return result_cstring("TAMPERED"),
    };
    match verify_file_impl(path_str, seal_str, keys_str) {
        Ok(true) => result_cstring("VERIFIED"),
        _ => result_cstring("TAMPERED"),
    }
}

fn result_cstring(s: &str) -> *mut c_char {
    match CString::new(s) {
        Ok(cs) => cs.into_raw(),
        Err(_) => std::ptr::null_mut(),
    }
}

#[no_mangle]
pub unsafe extern "C" fn rust_verify_free(ptr: *mut c_char) {
    if !ptr.is_null() {
        let _ = CString::from_raw(ptr);
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_mmr_root_single() {
        let h = Sha256::digest(b"hello").to_vec();
        let root = mmr_root(&[h.clone()]);
        assert_eq!(root, h);
    }

    #[test]
    fn test_hex_roundtrip() {
        let b = [1u8, 255, 16];
        let s = hex::encode(&b);
        let decoded = hex::decode(&s).unwrap();
        assert_eq!(decoded, b);
    }
}
