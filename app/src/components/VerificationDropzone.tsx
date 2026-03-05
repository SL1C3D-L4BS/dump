import { useCallback, useEffect, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import { listen } from "@tauri-apps/api/event";
import { TauriEvent } from "@tauri-apps/api/event";
import { cn } from "../lib/utils";

const DEFAULT_KEYS_PATH = ".config/vericore/keys.json";

export function VerificationDropzone() {
  const [status, setStatus] = useState<"idle" | "verified" | "tampered" | "error" | "verifying">("idle");
  const [message, setMessage] = useState<string>("");
  const [keysPath, setKeysPath] = useState(DEFAULT_KEYS_PATH);
  const [isDragOver, setIsDragOver] = useState(false);

  const runVerify = useCallback(
    async (parquetPath: string, sealPath: string) => {
      setStatus("verifying");
      setMessage("");
      try {
        const keys = keysPath.trim() || DEFAULT_KEYS_PATH;
        const result = await invoke<{ status: string; verified: boolean }>("verify_parquet_with_seal", {
          parquetPath,
          sealPath,
          keysPath: keys,
        });
        setStatus(result.verified ? "verified" : "tampered");
        setMessage(result.verified ? "PQC Verified" : "Tampered");
      } catch (e) {
        setStatus("error");
        setMessage(String(e));
      }
    },
    [keysPath]
  );

  useEffect(() => {
    const unlisten = listen<{ paths: string[] }>(TauriEvent.DRAG_DROP, (event) => {
      const paths = event.payload?.paths ?? [];
      const parquet = paths.find((p) => p.toLowerCase().endsWith(".parquet"));
      const seal = paths.find((p) => p.toLowerCase().endsWith(".vericore-seal"));
      if (parquet && seal) {
        runVerify(parquet, seal);
      } else {
        setStatus("error");
        setMessage("Drop both a .parquet file and a .vericore-seal file.");
      }
    });
    return () => {
      unlisten.then((fn) => fn());
    };
  }, [runVerify]);

  return (
    <div className="space-y-3">
      <label className="text-sm font-medium text-zinc-400">
        Keys path (public key for verification)
      </label>
      <input
        type="text"
        value={keysPath}
        onChange={(e) => setKeysPath(e.target.value)}
        placeholder={DEFAULT_KEYS_PATH}
        className="w-full rounded-lg border border-zinc-700 bg-zinc-900 px-3 py-2 text-sm text-zinc-200"
      />
      <div
        onDragEnter={() => setIsDragOver(true)}
        onDragLeave={() => setIsDragOver(false)}
        onDragOver={(e) => e.preventDefault()}
        className={cn(
          "flex min-h-[160px] flex-col items-center justify-center rounded-xl border-2 border-dashed p-6 transition-colors",
          isDragOver && "border-emerald-500 bg-emerald-500/10",
          status === "verified" && "border-emerald-500 bg-emerald-500/20 pqc-verified-glow",
          status === "tampered" && "border-red-500 bg-red-500/20 pqc-tampered-glow",
          status === "error" && "border-amber-600 bg-amber-500/10",
          status === "idle" && !isDragOver && "border-zinc-600",
          status === "verifying" && "border-zinc-400 dark:border-zinc-500"
        )}
      >
        {status === "verifying" && <p className="text-sm text-zinc-400">Verifying…</p>}
        {status === "verified" && (
          <p className="text-lg font-semibold text-emerald-400 drop-shadow-[0_0_8px_hsl(142_76%_42%_/0.8)">PQC Verified</p>
        )}
        {status === "tampered" && (
          <p className="text-lg font-semibold text-red-400">Tampered</p>
        )}
        {status === "error" && message && (
          <p className="max-w-md text-center text-sm text-amber-400">{message}</p>
        )}
        {(status === "idle" || status === "verified" || status === "tampered") && (
          <p className="mt-2 text-center text-sm text-zinc-400">
            Drop a .parquet file and its .vericore-seal here
          </p>
        )}
      </div>
    </div>
  );
}
