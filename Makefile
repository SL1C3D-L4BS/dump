# Phase 8: Build Rust core then Go binary with cgo
# Use 'make' for Rust+Go (cgo), or 'make go-only' for pure Go (no Rust).
.PHONY: all rust go go-only api clean run

all: rust go

# Pure Go build (no Rust lib; no cgo)
go-only:
	CGO_ENABLED=0 go build -o dump .
	CGO_ENABLED=0 go build -o dump-api ./api

rust:
	cd internal/core-rs && CARGO_TARGET_DIR=$(PWD)/internal/core-rs/target cargo build --release

go: rust
	CGO_ENABLED=1 go build -o dump -tags cgo .

# API server (Fiber POST /map, Rust engine, PQC seal in headers)
api: rust
	CGO_ENABLED=1 go build -o dump-api -tags cgo ./api

clean:
	cd internal/core-rs && cargo clean
	rm -f dump dump-api

run: all
	./dump --help
