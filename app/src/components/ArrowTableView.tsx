import { useEffect, useState } from "react";

/** Decode base64 Arrow IPC and render a table. Uses apache-arrow if available. */
export function ArrowTableView({ base64Data }: { base64Data: string }) {
  const [error, setError] = useState<string | null>(null);
  const [table, setTable] = useState<{ columns: string[]; rows: Record<string, unknown>[] } | null>(null);

  useEffect(() => {
    if (!base64Data.trim()) {
      setTable(null);
      setError(null);
      return;
    }
    setError(null);
    let cancelled = false;
    try {
      const binary = atob(base64Data.trim());
      const bytes = new Uint8Array(binary.length);
      for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
      import("apache-arrow").then((arrow) => {
        if (cancelled) return;
        try {
          const t = arrow.tableFromIPC(bytes);
          const columns = t.schema.fields.map((f) => f.name);
          const rows: Record<string, unknown>[] = [];
          for (let i = 0; i < t.length; i++) {
            const row: Record<string, unknown> = {};
            columns.forEach((col, j) => {
              const colData = t.getChildAt(j);
              row[col] = colData?.get(i) ?? null;
            });
            rows.push(row);
          }
          setTable({ columns, rows });
        } catch (e) {
          setError(String(e));
          setTable(null);
        }
      }).catch(() => setError("apache-arrow not loaded"));
    } catch (e) {
      setError(String(e));
      setTable(null);
    }
    return () => { cancelled = true; };
  }, [base64Data]);

  if (error) return <p className="text-sm text-amber-600">{error}</p>;
  if (!table) return <p className="text-sm text-zinc-500">No Arrow data or decoding…</p>;

  return (
    <div className="overflow-auto rounded-lg border border-zinc-700 bg-zinc-900/50">
      <table className="w-full text-left text-sm text-zinc-200">
        <thead>
          <tr className="border-b border-zinc-700 bg-zinc-800">
            {table.columns.map((c) => (
              <th key={c} className="px-3 py-2 font-medium text-violet-300">{c}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {table.rows.slice(0, 100).map((row, i) => (
            <tr key={i} className="border-b border-zinc-800">
              {table.columns.map((col) => (
                <td key={col} className="px-3 py-1.5">{String(row[col] ?? "")}</td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
      {table.rows.length > 100 && (
        <p className="px-3 py-2 text-xs text-zinc-500">Showing first 100 of {table.rows.length} rows</p>
      )}
    </div>
  );
}
