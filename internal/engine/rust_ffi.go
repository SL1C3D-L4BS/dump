//go:build cgo
// +build cgo

package engine

/*
#cgo windows LDFLAGS: -L ${SRCDIR}/../core-rs/target/release -ldump_core -lws2_32 -lntdll -luserenv -lkernel32
#cgo !windows LDFLAGS: -L ${SRCDIR}/../core-rs/target/release -ldump_core -lm -lpthread
#include <stdlib.h>
#include <string.h>

extern int rust_map_set_schema(const char* schema_json);
extern char* rust_map_row(const char* input);
extern void rust_map_row_free(char* ptr);
extern char* rust_sign_file(const char* path);
extern char* rust_sign_file_with_keys(const char* path, const char* keys_path);
extern void rust_sign_free(char* ptr);
extern char* rust_sign_last_error(void);
extern char* rust_verify_file(const char* path, const char* seal, const char* keys_path);
extern void rust_verify_free(char* ptr);
*/
import "C"

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"unsafe"
)

var (
	errRustSchema = errors.New("rust: failed to set schema")
	errRustMap    = errors.New("rust: map row failed")
	errRustSign   = errors.New("rust: sign file failed")
)

// SetRustSchema sets the mapping schema in the Rust core (JSON). Must be called before RustMapRow.
func SetRustSchema(schema *Schema) error {
	data, err := json.Marshal(schema)
	if err != nil {
		return err
	}
	cstr := C.CString(string(data))
	defer C.free(unsafe.Pointer(cstr))
	rc := C.rust_map_set_schema(cstr)
	if rc != 0 {
		return errRustSchema
	}
	return nil
}

// RustMapRow transforms one JSONL line using the Rust core. SetRustSchema must have been called.
func RustMapRow(line string) (string, error) {
	cIn := C.CString(line)
	defer C.free(unsafe.Pointer(cIn))
	cOut := C.rust_map_row(cIn)
	if cOut == nil {
		return "", errRustMap
	}
	defer C.rust_map_row_free(cOut)
	return C.GoString(cOut), nil
}

// SignFileRust signs the file at path using the Rust PQC kernel and returns the seal string.
// If keysPath is non-empty, uses persistent keys from that path (e.g. ~/.config/vericore/keys.json).
func SignFileRust(path string, keysPath string) (string, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	var cSeal *C.char
	if keysPath == "" {
		cSeal = C.rust_sign_file(cPath)
	} else {
		cKeys := C.CString(keysPath)
		defer C.free(unsafe.Pointer(cKeys))
		cSeal = C.rust_sign_file_with_keys(cPath, cKeys)
	}
	if cSeal == nil {
		var errMsg string
		// Symbol provided by Rust static lib at link time; requires CGO_ENABLED=1 for gopls.
		if cErr := C.rust_sign_last_error(); cErr != nil {
			errMsg = C.GoString(cErr)
			C.rust_sign_free(cErr)
			fmt.Fprintf(os.Stderr, "\033[35m🚨 Rust Core Error: %s\033[0m\n", errMsg)
			return "", errors.Join(errRustSign, errors.New("rust: "+errMsg))
		}
		return "", errRustSign
	}
	defer C.rust_sign_free(cSeal)
	return C.GoString(cSeal), nil
}

// VerifyFileRust verifies the file at path against the seal using the public key at keysPath.
// Returns "VERIFIED" or "TAMPERED".
func VerifyFileRust(path string, seal string, keysPath string) (string, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	cSeal := C.CString(seal)
	defer C.free(unsafe.Pointer(cSeal))
	cKeys := C.CString(keysPath)
	defer C.free(unsafe.Pointer(cKeys))
	cResult := C.rust_verify_file(cPath, cSeal, cKeys)
	if cResult == nil {
		return "TAMPERED", nil
	}
	defer C.rust_verify_free(cResult)
	return C.GoString(cResult), nil
}
