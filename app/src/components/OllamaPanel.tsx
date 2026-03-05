import { useCallback, useEffect, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import { Button } from "./ui/Button";

interface OllamaPanelProps {
  onModelChange?: (model: string) => void;
}

export function OllamaPanel({ onModelChange }: OllamaPanelProps = {}) {
  const [models, setModels] = useState<string[]>([]);
  const [selectedModel, setSelectedModel] = useState("");
  const [prompt, setPrompt] = useState("");
  const [response, setResponse] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const loadModels = useCallback(async () => {
    setError("");
    try {
      const list = await invoke<string[]>("ollama_list_models");
      setModels(list ?? []);
      if ((list?.length ?? 0) > 0 && !selectedModel) {
        setSelectedModel(list[0]);
        onModelChange?.(list[0]);
      }
    } catch (e) {
      setError("Ollama not reachable (localhost:11434). Is it running?");
      setModels([]);
    }
  }, [selectedModel, onModelChange]);

  useEffect(() => {
    loadModels();
  }, [loadModels]);

  const generate = useCallback(async () => {
    if (!selectedModel || !prompt.trim()) return;
    setLoading(true);
    setResponse("");
    setError("");
    try {
      const out = await invoke<string>("ollama_generate", {
        model: selectedModel,
        prompt: prompt.trim(),
      });
      setResponse(out);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  }, [selectedModel, prompt]);

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold text-zinc-200">Local AI (Ollama)</h3>
        <Button variant="outline" size="sm" onClick={loadModels}>
          Refresh models
        </Button>
      </div>
      {error && <p className="text-sm text-amber-400">{error}</p>}
      <div>
        <label className="text-xs text-zinc-400">Model</label>
        <select
          value={selectedModel}
          onChange={(e) => {
            const v = e.target.value;
            setSelectedModel(v);
            onModelChange?.(v);
          }}
          className="mt-1 w-full rounded-lg border border-zinc-700 bg-zinc-900 px-3 py-2 text-sm text-zinc-200"
        >
          {models.map((m) => (
            <option key={m} value={m}>
              {m}
            </option>
          ))}
        </select>
      </div>
      <div>
        <label className="text-xs text-zinc-400">Prompt</label>
        <textarea
          value={prompt}
          onChange={(e) => setPrompt(e.target.value)}
          placeholder="Ask the model..."
          rows={2}
          className="mt-1 w-full rounded-lg border border-zinc-700 bg-zinc-900 px-3 py-2 text-sm text-zinc-200"
        />
      </div>
      <Button onClick={generate} disabled={loading || !selectedModel || !prompt.trim()}>
        {loading ? "Generating…" : "Generate"}
      </Button>
      {response && (
        <div className="rounded-lg border border-zinc-700 bg-zinc-900/50 p-3 text-sm text-zinc-300">
          {response}
        </div>
      )}
    </div>
  );
}
