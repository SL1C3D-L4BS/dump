import { useCallback, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import { Button } from "./ui/Button";

export function AuditTab() {
  const [dirPath, setDirPath] = useState("");
  const [keysPath, setKeysPath] = useState(".config/vericore/keys.json");
  const [entries, setEntries] = useState<Array<{ parquet_path: string; seal_path: string }>>([]);
  const [stream, setStream] = useState<Array<{ path: string; status: string }>>([]);
  const [running, setRunning] = useState(false);

  const refreshList = useCallback(async () => {
    if (!dirPath.trim()) return;
    try {
      const list = await invoke<{ parquet_path: string; seal_path: string }[]>("audit_list_parquet_with_seal", {
        dirPath: dirPath.trim(),
      });
      setEntries(list ?? []);
    } catch (e) {
      setEntries([]);
    }
  }, [dirPath]);

  const runVerification = useCallback(async () => {
    setRunning(true);
    setStream([]);
    for (const entry of entries) {
      setStream((s) => [...s, { path: entry.parquet_path, status: "…" }]);
      try {
        const result = await invoke<{ status: string; verified: boolean }>("verify_parquet_with_seal", {
          parquetPath: entry.parquet_path,
          sealPath: entry.seal_path,
          keysPath,
        });
        const status = result.verified ? "PQC Verified" : "Tampered";
        setStream((s) => {
          const next = [...s];
          const i = next.findIndex((x) => x.path === entry.parquet_path);
          if (i >= 0) next[i] = { path: entry.parquet_path, status };
          return next;
        });
      } catch (e) {
        setStream((s) => {
          const next = [...s];
          const i = next.findIndex((x) => x.path === entry.parquet_path);
          if (i >= 0) next[i] = { path: entry.parquet_path, status: "Error: " + String(e) };
          return next;
        });
      }
    }
    setRunning(false);
  }, [entries, keysPath]);

  return (
    <div className="space-y-4">
      <h3 className="text-sm font-semibold text-zinc-200">Verification Ledger — Integrity Stream</h3>
      <div>
        <label className="text-xs text-zinc-400">Directory to monitor</label>
        <div className="mt-1 flex gap-2">
          <input
            value={dirPath}
            onChange={(e) => setDirPath(e.target.value)}
            placeholder="/path/to/parquet/folder"
            className="flex-1 rounded-lg border border-zinc-700 bg-zinc-900 px-3 py-2 text-sm text-zinc-200"
          />
          <Button variant="outline" size="sm" onClick={refreshList}>
            List .parquet + seal
          </Button>
        </div>
      </div>
      <div>
        <label className="text-xs text-zinc-400">Keys path</label>
        <input
          value={keysPath}
          onChange={(e) => setKeysPath(e.target.value)}
          className="mt-1 w-full rounded-lg border border-zinc-700 bg-zinc-900 px-3 py-2 text-sm text-zinc-200"
        />
      </div>
      {entries.length > 0 && (
        <Button onClick={runVerification} disabled={running}>
          {running ? "Verifying…" : "Verify all"}
        </Button>
      )}
      <div className="rounded-lg border border-zinc-700 bg-zinc-900/50 p-3">
        <p className="mb-2 text-xs font-medium text-zinc-400">Integrity Stream</p>
        {stream.length === 0 && <p className="text-sm text-zinc-400">Run verification to see results.</p>}
        <ul className="space-y-1 text-sm">
          {stream.map((s, i) => (
            <li
              key={i}
              className={
                s.status === "PQC Verified"
                  ? "text-emerald-400 pqc-verified-glow rounded px-2 py-1"
                  : s.status === "Tampered"
                    ? "text-red-400"
                    : ""
              }
            >
              {s.path.split("/").pop()} — {s.status}
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
}
