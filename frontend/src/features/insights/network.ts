import { sourceName } from '../sessions/format';
import type { Metric } from './timeline';
export type ModelUsage = { source: string; model: string; calls: number; total_tokens: number; known_cost_usd: number; unknown_cost_calls: number };
export type NetworkNode = { id: string; label: string; kind: 'total' | 'source' | 'model' | 'group'; source: string; color: string; usage: ModelUsage; position: [number, number, number]; models?: string[] };
export type UsageNetwork = { nodes: NetworkNode[]; edges: { from: string; to: string }[] };
const palette = ['#bca2ff', '#66d9ee', '#efb988', '#9fdbbb'];
const empty = (): ModelUsage => ({ source: '', model: '', calls: 0, total_tokens: 0, known_cost_usd: 0, unknown_cost_calls: 0 });
export function sumUsage(rows: ModelUsage[]): ModelUsage {
  return rows.reduce((sum, row) => ({ ...sum, calls: sum.calls + row.calls, total_tokens: sum.total_tokens + row.total_tokens, known_cost_usd: sum.known_cost_usd + row.known_cost_usd, unknown_cost_calls: sum.unknown_cost_calls + row.unknown_cost_calls }), empty());
}
export function usageNetwork(rows: ModelUsage[], metric: Metric, modelLimit = 5): UsageNetwork {
  const nodes: NetworkNode[] = [{ id: 'total', label: 'All usage', kind: 'total', source: '', color: '#e9f2ff', usage: sumUsage(rows), position: [-6.7, 0, 0] }];
  const edges: UsageNetwork['edges'] = [];
  const sources = [...new Set(rows.map(row => row.source))].sort();
  const bands = sources.map(source => { const count = rows.filter(row => row.source === source).length; return Math.max(5.4, (Math.min(count, modelLimit) + (count > modelLimit ? 1 : 0)) * 1.15 + 1); });
  sources.forEach((source, index) => {
    const sourceRows = rows.filter(row => row.source === source).sort((a,b) => b[metric] - a[metric] || a.model.localeCompare(b.model));
    const id = `source:${source}`, color = palette[index % palette.length], y = bands.reduce((sum,n)=>sum+n,0)/2 - bands.slice(0,index).reduce((sum,n)=>sum+n,0) - bands[index]/2;
    nodes.push({ id, label: source === 'claude_gateway' ? 'Claude gateway' : sourceName(source), kind: 'source', source, color, usage: sumUsage(sourceRows), position: [-2.3,y,0] });
    edges.push({ from: 'total', to: id });
    const leaves = sourceRows.slice(0,modelLimit).map(row => ({ usage: row, label: row.model || 'Model not recorded', models: undefined as string[] | undefined }));
    if (sourceRows.length > modelLimit) leaves.push({ usage: sumUsage(sourceRows.slice(modelLimit)), label: `+${sourceRows.length-modelLimit} other models`, models: sourceRows.slice(modelLimit).map(row=>row.model || 'Model not recorded') });
    leaves.forEach((leaf,i) => {
      const nodeID = JSON.stringify([source,leaf.models ? 'group' : 'model',leaf.usage.model]);
      nodes.push({ id: nodeID, label: leaf.label, kind: leaf.models ? 'group' : 'model', source, color, usage: leaf.usage, models: leaf.models, position: [3.4+(i%2)*1.2,y+((leaves.length-1)/2-i)*1.15,Math.sin(i*1.7)*1.8] });
      edges.push({ from: id, to: nodeID });
    });
  });
  return { nodes, edges };
}
