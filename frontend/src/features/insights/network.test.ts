import { describe,it,expect } from 'vitest';
import { usageNetwork, sumUsage, type ModelUsage } from './network';
describe('usage network',()=>{
 it('keeps every event in the graph when small models are grouped',()=>{
  const rows: ModelUsage[] = Array.from({length:9},(_,i)=>({source:'codex',model:`m${i}`,calls:i+1,total_tokens:(i+1)*100,known_cost_usd:i,unknown_cost_calls:i===0?1:0}));
  rows.push({...rows[0],source:'cursor'});
  const graph=usageNetwork(rows,'known_cost_usd');
  expect(sumUsage(graph.nodes.filter(n=>n.kind==='model'||n.kind==='group').map(n=>n.usage))).toEqual(sumUsage(rows));
  expect(graph.nodes.find(n=>n.kind==='group')?.models).toHaveLength(4);
  const expanded=usageNetwork(rows,'calls',Infinity);
  expect(expanded.nodes.filter(n=>n.kind==='model')).toHaveLength(rows.length);
  expect(expanded.nodes.some(n=>n.kind==='group')).toBe(false);
  expect(sumUsage(expanded.nodes.filter(n=>n.kind==='model').map(n=>n.usage))).toEqual(sumUsage(rows));
  expect(graph.nodes.find(n=>n.id==='total')?.usage.calls).toBe(46);
  expect(graph.edges.every(e=>graph.nodes.some(n=>n.id===e.from)&&graph.nodes.some(n=>n.id===e.to))).toBe(true);
 });
 it('keeps missing model labels explicit and does not invent relationships',()=>{
  const graph=usageNetwork([{source:'cursor',model:'',calls:1,total_tokens:0,known_cost_usd:0,unknown_cost_calls:1}],'calls');
  expect(graph.nodes[2].label).toBe('Model not recorded');
  expect(graph.edges).toHaveLength(2);
  expect(usageNetwork([],'calls').nodes[0].usage.calls).toBe(0);
 });
});
