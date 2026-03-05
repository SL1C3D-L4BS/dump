import { useCallback, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import { Button } from "./ui/Button";

export function KeysSettings() {
  const [keysPath, setKeysPath] = useState(".config/vericore/keys.json");
  const [info, setInfo] = useState<{ exists: boolean; public_key_hex?: string } | null>(null);
  const [rotating, setRotating] = useState(false);
  const [error, setError] = useState("");

  const readKeys = useCallback(async () => {
    setError("");
    try {
      const result = await invoke<{ exists: boolean; public_key_hex?: string }>("keys_read", {
        keysPath: keysPath.trim(),
      });
      setInfo(result);
    } catch (e) {
      setError(String(e));
      setInfo(null);
    }
  }, [keysPath]);

  const rotate = useCallback(async () => {
    if (!keysPath.trim()) return;
    setRotating(true);
    setError("");
    try {
      const newPk = await invoke<string>("keys_rotate", { keysPath: keysPath.trim() });
      setInfo({ exists: true, public_key_hex: newPk });
    } catch (e) {
      setError(String(e));
    } finally {
      setRotating(false);
    }
  }, [keysPath]);

  return (
    <div className="space-y-4">
      <h3 className="text-sm font-semibold text-zinc-200">Dilithium2 key management</h3>
      {error && <p className="text-sm text-amber-400">{error}</p>}
      <div>
        <label className="text-xs text-zinc-400">Keys file path</label>
        <input
          value={keysPath}
          onChange={(e) => setKeysPath(e.target.value)}
          placeholder="~/.config/vericore/keys.json"
          className="mt-1 w-full rounded-lg border border-zinc-700 bg-zinc-900 px-3 py-2 text-sm text-zinc-200"
        />
      </div>
      <div className="flex gap-2">
        <Button variant="outline" onClick={readKeys}>
          View keys
        </Button>
        <Button onClick={rotate} disabled={rotating}>
          {rotating ? "Rotating…" : "Rotate keys"}
        </Button>
      </div>
      {info && (
        <div className="rounded-lg border border-zinc-700 bg-zinc-900/50 p-3">
          <p className="text-xs text-zinc-400">Exists: {info.exists ? "Yes" : "No"}</p>
          {info.public_key_hex && (
            <p className="mt-1 break-all font-mono text-xs text-zinc-300">
              Public key (hex): {info.public_key_hex.length > 32 ? `${info.public_key_hex.slice(0, 16)}…${info.public_key_hex.slice(-16)}` : info.public_key_hex}
            </p>
          )}
        </div>
      )}
    </div>
  );
}
