// Package main runs the DUMP universal fullstack API: POST /map streams through the Rust engine and returns PQC seal in headers.
package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/SL1C3D-L4BS/dump/internal/engine"
	"github.com/SL1C3D-L4BS/dump/internal/integrity"
	"github.com/gofiber/fiber/v2"
)

const defaultPort = "8080"

func main() {
	app := fiber.New()

	app.Post("/map", handleMap)
	app.Get("/sources", handleSources)

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}
	_ = app.Listen(":" + port)
}

// handleMap pipes the request body (JSONL) through the Rust mapping engine, writes to a temp file,
// signs it with the Rust PQC kernel, and returns the mapped data with X-Vericore-Seal in headers.
func handleMap(c *fiber.Ctx) error {
	schemaJSON := c.Get("X-Mapping-Schema")
	if schemaJSON == "" {
		return c.Status(http.StatusBadRequest).SendString("missing X-Mapping-Schema header (JSON mapping schema)")
	}
	var schema engine.Schema
	if err := json.Unmarshal([]byte(schemaJSON), &schema); err != nil {
		return c.Status(http.StatusBadRequest).SendString("invalid X-Mapping-Schema JSON: " + err.Error())
	}

	tmp, err := os.CreateTemp("", "dump-map-*.jsonl")
	if err != nil {
		return c.Status(http.StatusInternalServerError).SendString("temp file: " + err.Error())
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	defer tmp.Close()

	body := c.Body()
	_, err = engine.MapStream(bytes.NewReader(body), &schema, engine.JSONLWriter{W: tmp})
	if err != nil {
		return c.Status(http.StatusInternalServerError).SendString("map stream: " + err.Error())
	}
	if err = tmp.Sync(); err != nil {
		return c.Status(http.StatusInternalServerError).SendString("sync: " + err.Error())
	}
	if _, err = tmp.Seek(0, io.SeekStart); err != nil {
		return c.Status(http.StatusInternalServerError).SendString("seek: " + err.Error())
	}

	seal, err := integrity.SignResult(tmpPath)
	if err != nil {
		return c.Status(http.StatusInternalServerError).SendString("sign: " + err.Error())
	}
	c.Set("X-Vericore-Seal", strings.ReplaceAll(seal, "\n", " "))
	c.Set("Content-Type", "application/x-ndjson")
	return c.SendStream(tmp, -1)
}

// handleSources returns the Discovery layer for the web UI: local files and DB connection status.
// DUMP_SOURCES_DIR: directory to list (default: .). DUMP_DB_URLS: comma-separated DB URLs to ping.
func handleSources(c *fiber.Ctx) error {
	type fileInfo struct {
		Path string `json:"path"`
		Name string `json:"name"`
	}
	type dbInfo struct {
		URL string `json:"url"`
		OK  bool   `json:"ok"`
	}
	var out struct {
		Files     []fileInfo `json:"files"`
		Databases []dbInfo   `json:"databases"`
	}
	dir := os.Getenv("DUMP_SOURCES_DIR")
	if dir == "" {
		dir = "."
	}
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			out.Files = append(out.Files, fileInfo{
				Path: filepath.Join(dir, e.Name()),
				Name: e.Name(),
			})
		}
	}
	for _, u := range strings.Split(os.Getenv("DUMP_DB_URLS"), ",") {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		ok := false
		if r, err := engine.NewSQLReader(u, "SELECT 1"); err == nil {
			ok = true
			r.Close()
		}
		out.Databases = append(out.Databases, dbInfo{URL: u, OK: ok})
	}
	return c.JSON(out)
}
