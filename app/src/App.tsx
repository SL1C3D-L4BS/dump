import { useCallback, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import { VerificationDropzone } from "./components/VerificationDropzone";
import { type SuggestedGraph } from "./components/MappingGraph";
import { MappingTab } from "./components/MappingTab";
import { MappingGraph } from "./components/MappingGraph";
import { OllamaPanel } from "./components/OllamaPanel";
import { SourceExplorer } from "./components/SourceExplorer";
import { AuditTab } from "./components/AuditTab";
import { KeysSettings } from "./components/KeysSettings";
import { cn } from "./lib/utils";

type TabId = "mapping" | "verify" | "sources" | "audit" | "settings";

export default function App() {
  const [tab, setTab] = useState<TabId>("mapping");
  const [selectedModel, setSelectedModel] = useState("");
  const [suggestedGraph, setSuggestedGraph] = useState<SuggestedGraph | null>(null);
  const [sampleLines, setSampleLines] = useState<string[]>([]);

  const handleSuggestMapping = useCallback(async (headers: string[]) => {
    const model = selectedModel || (await invoke<string[]>("ollama_list_models").then((m) => m[0]).catch(() => ""));
    if (!model) throw new Error("No Ollama model selected");
    const raw = await invoke<string>("ollama_generate_mapping", { model, headers });
    let json = raw.trim();
    const start = json.indexOf("{");
    const end = json.lastIndexOf("}") + 1;
    if (start >= 0 && end > start) json = json.slice(start, end);
    const parsed = JSON.parse(json) as SuggestedGraph;
    if (parsed.nodes && parsed.edges) setSuggestedGraph(parsed);
  }, [selectedModel]);

  const tabs: { id: TabId; label: string }[] = [
    { id: "mapping", label: "Mapping" },
    { id: "verify", label: "Verify" },
    { id: "sources", label: "Sources" },
    { id: "audit", label: "Audit" },
    { id: "settings", label: "Keys" },
  ];

  return (
    <div className="min-h-screen bg-zinc-950">
      <header className="border-b border-zinc-800 bg-zinc-900/90 px-6 py-4">
        <h1 className="text-xl font-semibold text-zinc-100">Frontier Workspace</h1>
        <p className="mt-1 text-sm text-zinc-400">Phase 12 — Cyber-Auditor</p>
        <nav className="mt-4 flex gap-2">
          {tabs.map((t) => (
            <button
              key={t.id}
              onClick={() => setTab(t.id)}
              className={cn(
                "rounded-lg px-3 py-1.5 text-sm font-medium transition-colors",
                tab === t.id
                  ? "bg-violet-600 text-white shadow-md shadow-violet-500/20"
                  : "bg-zinc-800 text-zinc-300 hover:bg-zinc-700 hover:text-zinc-100"
              )}
            >
              {t.label}
            </button>
          ))}
        </nav>
      </header>

      <main className="mx-auto max-w-6xl p-6">
        {tab === "mapping" && (
          <MappingTab suggestedGraph={suggestedGraph} sampleLines={sampleLines.length ? sampleLines : undefined} onModelChange={setSelectedModel} />
        )}

        {tab === "verify" && (
          <section className="rounded-xl border border-zinc-800 bg-zinc-900/80 p-6">
            <h2 className="mb-4 text-sm font-semibold uppercase tracking-wide text-violet-400">PQC Verification</h2>
            <VerificationDropzone />
          </section>
        )}

        {tab === "sources" && (
          <div className="space-y-6">
            <section className="rounded-xl border border-zinc-800 bg-zinc-900/80 p-6">
              <SourceExplorer
                selectedModel={selectedModel}
                onHeadersSelected={() => {}}
                onSuggestMapping={handleSuggestMapping}
                onSampleLoaded={setSampleLines}
              />
            </section>
            <section className="rounded-xl border border-zinc-800 bg-zinc-900/80 p-6">
              <h2 className="mb-4 text-sm font-semibold uppercase tracking-wide text-violet-400">Mapping canvas</h2>
              <MappingGraph suggestedGraph={suggestedGraph} />
            </section>
          </div>
        )}

        {tab === "audit" && (
          <section className="rounded-xl border border-zinc-800 bg-zinc-900/80 p-6">
            <h2 className="mb-4 text-sm font-semibold uppercase tracking-wide text-violet-400">Verification Ledger</h2>
            <AuditTab />
          </section>
        )}

        {tab === "settings" && (
          <section className="rounded-xl border border-zinc-800 bg-zinc-900/80 p-6">
            <h2 className="mb-4 text-sm font-semibold uppercase tracking-wide text-violet-400">Key management</h2>
            <KeysSettings />
          </section>
        )}
      </main>
    </div>
  );
}
