import { useCallback, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import { MappingGraph, type SuggestedGraph } from "./MappingGraph";
import { OllamaPanel } from "./OllamaPanel";
import { ArrowTableView } from "./ArrowTableView";
import { Button } from "./ui/Button";

function buildSchemaFromGraph(suggestedGraph: SuggestedGraph | null): string {
  if (!suggestedGraph?.nodes?.length || !suggestedGraph?.edges?.length) {
    return JSON.stringify({
      rules: [
        { target_field: "id", source_path: "id", type: "number" },
        { target_field: "name", source_path: "name", type: "string" },
      ],
    });
  }
  const { nodes, edges } = suggestedGraph;
  const rules = edges
    .filter((e) => e.target === "dest")
    .map((e) => {
      const node = nodes.find((n) => n.id === e.source);
      const label = (node?.data as { label?: string } | undefined)?.label ?? e.source;
      return { target_field: label, source_path: label, type: "string" };
    });
  if (rules.length === 0) {
    return JSON.stringify({
      rules: [
        { target_field: "id", source_path: "id", type: "number" },
        { target_field: "name", source_path: "name", type: "string" },
      ],
    });
  }
  return JSON.stringify({ rules });
}

const DEFAULT_SAMPLE_LINES = [
  '{"id":1,"name":"Alice","email":"alice@example.com"}',
  '{"id":2,"name":"Bob","email":"bob@example.com"}',
];

interface MappingTabProps {
  suggestedGraph: SuggestedGraph | null;
  sampleLines?: string[];
  onModelChange?: (model: string) => void;
}

export function MappingTab({ suggestedGraph, sampleLines, onModelChange }: MappingTabProps) {
  const [arrowPreviewBase64, setArrowPreviewBase64] = useState("");
  const [testLoading, setTestLoading] = useState(false);
  const [testError, setTestError] = useState("");

  const runTestMapping = useCallback(async () => {
    setTestLoading(true);
    setTestError("");
    setArrowPreviewBase64("");
    try {
      const schemaJson = buildSchemaFromGraph(suggestedGraph);
      const lines = sampleLines?.length ? sampleLines : DEFAULT_SAMPLE_LINES;
      const base64 = await invoke<string>("map_rows_to_arrow", {
        schemaJson,
        jsonLines: lines,
      });
      setArrowPreviewBase64(base64 ?? "");
    } catch (e) {
      setTestError(String(e));
    } finally {
      setTestLoading(false);
    }
  }, [suggestedGraph, sampleLines]);

  return (
    <div className="space-y-6">
      <section className="rounded-xl border border-zinc-800 bg-zinc-900/80 p-6">
        <h2 className="mb-4 text-sm font-semibold uppercase tracking-wide text-violet-400">
          Column mapping (source to Parquet)
        </h2>
        <MappingGraph suggestedGraph={suggestedGraph} />
      </section>

      <section className="rounded-xl border border-zinc-800 bg-zinc-900/80 p-6">
        <h2 className="mb-4 text-sm font-semibold uppercase tracking-wide text-violet-400">
          Local AI (Ollama)
        </h2>
        <OllamaPanel onModelChange={onModelChange} />
      </section>

      <section className="rounded-xl border border-zinc-800 bg-zinc-900/80 p-6">
        <h2 className="mb-4 text-sm font-semibold uppercase tracking-wide text-violet-400">
          Live Data Preview
        </h2>
        <p className="mb-3 text-sm text-zinc-400">
          Test the current mapping with sample data. Schema is derived from the graph; use Sources to load sample rows from CSV or SQL.
        </p>
        <Button
          onClick={runTestMapping}
          disabled={testLoading}
          className="bg-violet-600 text-white hover:bg-violet-500"
        >
          {testLoading ? "Mapping…" : "Test Mapping (Arrow IPC)"}
        </Button>
        {testError && <p className="mt-2 text-sm text-red-400">{testError}</p>}
        {arrowPreviewBase64 && (
          <div className="mt-4">
            <ArrowTableView base64Data={arrowPreviewBase64} />
          </div>
        )}
      </section>
    </div>
  );
}
