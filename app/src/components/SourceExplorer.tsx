import { useCallback, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import { Button } from "./ui/Button";

export type DirEntry = { name: string; path: string; is_dir: boolean };

interface SourceExplorerProps {
  onHeadersSelected: (headers: string[], sourceLabel: string) => void;
  onSuggestMapping: (headers: string[]) => Promise<void>;
  onSampleLoaded?: (lines: string[]) => void;
  selectedModel: string;
}

export function SourceExplorer(props: SourceExplorerProps) {
  const { onHeadersSelected, onSuggestMapping, onSampleLoaded, selectedModel } = props;
  const [dirPath, setDirPath] = useState("");
  const [entries, setEntries] = useState<DirEntry[]>([]);
  const [dbUrl, setDbUrl] = useState("");
  const [dbOk, setDbOk] = useState<boolean | null>(null);
  const [tables, setTables] = useState<string[]>([]);
  const [selectedTable, setSelectedTable] = useState("");
  const [headers, setHeaders] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const listDir = useCallback(async () => {
    if (!dirPath.trim()) return;
    setError("");
    try {
      const list = await invoke<DirEntry[]>("discover_list_dir", { path: dirPath.trim() });
      setEntries(list ?? []);
    } catch (e) {
      setError(String(e));
      setEntries([]);
    }
  }, [dirPath]);

  const pingDb = useCallback(async () => {
    if (!dbUrl.trim()) return;
    setError("");
    try {
      const ok = await invoke<boolean>("discover_ping_db", { dbUrl: dbUrl.trim() });
      setDbOk(ok);
      if (ok) {
        const tbls = await invoke<string[]>("discover_sql_tables", { dbUrl: dbUrl.trim() });
        setTables(tbls ?? []);
        setSelectedTable("");
        setHeaders([]);
      }
    } catch (e) {
      setError(String(e));
      setDbOk(false);
    }
  }, [dbUrl]);

  const loadTableHeaders = useCallback(async () => {
    if (!dbUrl.trim() || !selectedTable) return;
    setError("");
    try {
      const h = await invoke<string[]>("discover_sql_headers", { dbUrl: dbUrl.trim(), table: selectedTable });
      setHeaders(h ?? []);
      onHeadersSelected(h ?? [], "SQL: " + selectedTable);
    } catch (e) {
      setError(String(e));
    }
  }, [dbUrl, selectedTable, onHeadersSelected]);

  const loadCsvHeaders = useCallback(
    async (path: string) => {
      setError("");
      try {
        const h = await invoke<string[]>("discover_csv_headers", { filePath: path });
        setHeaders(h ?? []);
        const name = path.split("/").pop() ?? path;
        onHeadersSelected(h ?? [], name);
      } catch (e) {
        setError(String(e));
      }
    },
    [onHeadersSelected]
  );

  const suggestMapping = useCallback(async () => {
    if (!headers.length || !selectedModel) return;
    setLoading(true);
    setError("");
    try {
      await onSuggestMapping(headers);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  }, [headers, selectedModel, onSuggestMapping]);

  return (
    <div className="space-y-4">
      <h3 className="text-sm font-semibold text-zinc-200">Source Explorer</h3>
      {error && <p className="text-sm text-amber-400">{error}</p>}
      <div>
        <label className="text-xs text-zinc-400">Directory</label>
        <div className="mt-1 flex gap-2">
          <input
            value={dirPath}
            onChange={(e) => setDirPath(e.target.value)}
            placeholder="/path/to/folder"
            className="flex-1 rounded-lg border border-zinc-700 bg-zinc-900 px-3 py-2 text-sm text-zinc-200"
          />
          <Button variant="outline" size="sm" onClick={listDir}>List</Button>
        </div>
        {entries.length > 0 && (
          <ul className="mt-2 max-h-40 overflow-auto rounded border border-zinc-200 p-2 dark:border-zinc-700">
            {entries.map((e) => (
              <li key={e.path} className="flex items-center gap-2 text-sm">
                <span className={e.is_dir ? "text-blue-600" : ""}>{e.name}</span>
                {!e.is_dir && (e.name.endsWith(".csv") || e.name.endsWith(".jsonl")) && (
                  <>
                    <Button variant="ghost" size="sm" onClick={() => loadCsvHeaders(e.path)}>Use headers</Button>
                    {e.name.endsWith(".csv") && onSampleLoaded && (
                      <Button variant="ghost" size="sm" onClick={async () => {
                        try {
                          const lines = await invoke<string[]>("read_csv_sample", { filePath: e.path, limit: 10 });
                          onSampleLoaded(lines ?? []);
                        } catch (_) {}
                      }}>Load sample</Button>
                    )}
                  </>
                )}
              </li>
            ))}
          </ul>
        )}
      </div>
      <div>
        <label className="text-xs text-zinc-400">Database URL</label>
        <div className="mt-1 flex gap-2">
          <input
            value={dbUrl}
            onChange={(e) => { setDbUrl(e.target.value); setDbOk(null); }}
            placeholder="file:./data.db or postgres://..."
            className="flex-1 rounded-lg border border-zinc-700 bg-zinc-900 px-3 py-2 text-sm text-zinc-200"
          />
          <Button variant="outline" size="sm" onClick={pingDb}>Ping</Button>
        </div>
        {dbOk !== null && <p className="mt-1 text-xs text-zinc-400">{dbOk ? "Connected" : "Failed"}</p>}
        {tables.length > 0 && (
          <div className="mt-2">
            <label className="text-xs text-zinc-400">Table</label>
            <select
              value={selectedTable}
              onChange={(e) => setSelectedTable(e.target.value)}
              className="mt-1 w-full rounded-lg border border-zinc-700 bg-zinc-900 px-3 py-2 text-sm text-zinc-200"
            >
              <option value="">Select table</option>
              {tables.map((t) => <option key={t} value={t}>{t}</option>)}
            </select>
            <Button variant="outline" size="sm" className="mt-2" onClick={loadTableHeaders} disabled={!selectedTable}>Load headers</Button>
          </div>
        )}
      </div>
      {headers.length > 0 && (
        <div>
          <p className="text-xs text-zinc-400">Headers: {headers.join(", ")}</p>
          <Button className="mt-2" onClick={suggestMapping} disabled={loading || !selectedModel}>
            {loading ? "Asking AI..." : "Suggest mapping (Ollama)"}
          </Button>
        </div>
      )}
    </div>
  );
}
