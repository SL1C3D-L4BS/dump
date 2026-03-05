import { useCallback, useEffect } from "react";
import dagre from "dagre";
import ReactFlow, {
  Background,
  Controls,
  MiniMap,
  addEdge,
  useNodesState,
  useEdgesState,
  type Connection,
  type Edge,
  type Node,
} from "reactflow";
import "reactflow/dist/style.css";

const NODE_WIDTH = 180;
const NODE_HEIGHT = 40;

function getLayoutedElements(
  nodes: Node[],
  edges: Edge[],
  direction: "LR" | "TB" | "RL" | "BT" = "LR"
): { nodes: Node[]; edges: Edge[] } {
  const g = new dagre.graphlib.Graph();
  g.setGraph({ rankdir: direction, nodesep: 60, ranksep: 80 });
  g.setDefaultEdgeLabel(() => ({}));

  nodes.forEach((node) => {
    g.setNode(node.id, { width: NODE_WIDTH, height: NODE_HEIGHT });
  });
  edges.forEach((edge) => {
    g.setEdge(edge.source, edge.target);
  });

  dagre.layout(g);
  const isHorizontal = direction === "LR" || direction === "RL";

  const layoutedNodes = nodes.map((node) => {
    const n = g.node(node.id);
    if (!n) return node;
    return {
      ...node,
      position: {
        x: n.x - NODE_WIDTH / 2,
        y: n.y - NODE_HEIGHT / 2,
      },
      sourcePosition: isHorizontal ? "right" : "bottom",
      targetPosition: isHorizontal ? "left" : "top",
    };
  });

  return { nodes: layoutedNodes, edges };
}

const nodeStyle = { background: "#52525b", borderRadius: 8, padding: 12, color: "#f4f4f5", border: "1px solid #71717a" };

const defaultNodes: Node[] = [
  { id: "source", type: "input", position: { x: 80, y: 120 }, data: { label: "Postgres / CSV" }, style: nodeStyle },
  { id: "dest", type: "output", position: { x: 400, y: 120 }, data: { label: "Parquet" }, style: nodeStyle },
];
const defaultEdges: Edge[] = [{ id: "e1", source: "source", target: "dest", animated: true }];

export type SuggestedGraph = { nodes: Node[]; edges: Edge[] };

interface MappingGraphProps {
  suggestedGraph?: SuggestedGraph | null;
}

export function MappingGraph({ suggestedGraph }: MappingGraphProps) {
  const [nodes, setNodes, onNodesChange] = useNodesState(defaultNodes);
  const [edges, setEdges, onEdgesChange] = useEdgesState(defaultEdges);

  useEffect(() => {
    if (suggestedGraph?.nodes?.length && suggestedGraph?.edges) {
      const styled = suggestedGraph.nodes.map((n) => ({
        ...n,
        style: nodeStyle,
      }));
      const { nodes: layouted, edges: layoutedEdges } = getLayoutedElements(styled, suggestedGraph.edges, "LR");
      setNodes(layouted);
      setEdges(layoutedEdges);
    }
  }, [suggestedGraph, setNodes, setEdges]);

  const onConnect = useCallback(
    (params: Connection) => setEdges((eds) => addEdge(params, eds)),
    [setEdges]
  );

  return (
    <div className="h-[320px] w-full rounded-xl border border-zinc-800 bg-zinc-900/50">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        onConnect={onConnect}
        fitView
        className="rounded-xl"
      >
        <Background />
        <Controls />
        <MiniMap />
      </ReactFlow>
    </div>
  );
}
