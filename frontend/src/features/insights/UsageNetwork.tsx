import { formatNumber } from '@/i18n/format';
import { useTranslation } from 'react-i18next';
import { lazy, Suspense, useMemo, useState } from 'react';
import { Box, Pause, Play, RotateCcw, MoveUpRight, Expand } from 'lucide-react';
import { Card } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { useCurrency } from '../currency/useCurrency';
import { usageNetwork, type ModelUsage } from './network';
import type { Metric } from './timeline';
const Scene=lazy(()=>import('./NetworkScene'));
export function UsageNetwork({rows,expanded=false,initialMetric='known_cost_usd'}:{rows:ModelUsage[];expanded?:boolean;initialMetric?:Metric}) {
 const { t, i18n } = useTranslation('sessions');
 const [showDetails,setShowDetails]=useState(!expanded);
 const [view,setView]=useState<'perspective'|'front'|'side'|'above'>('perspective');
 const [metric,setMetric]=useState<Metric>(initialMetric),[selected,setSelected]=useState('total'),[paused,setPaused]=useState(false),[reset,setReset]=useState(0);
 const selectNode=(id:string)=>{setSelected(id);setShowDetails(true);};
 const graph=useMemo(()=>usageNetwork(rows,metric,expanded ? Infinity : 5,i18n.resolvedLanguage),[rows,metric,expanded,i18n.resolvedLanguage]);
 const active=graph.nodes.find(n=>n.id===selected)??graph.nodes[0];
 const {fmt}=useCurrency();
 const format=(value:number)=>metric==='known_cost_usd'?fmt(value):formatNumber(value);
 const total=graph.nodes[0].usage[metric];
 const share=total>0?active.usage[metric]/total*100:0;
 const sourceNodes=graph.nodes.filter(n=>n.kind==='source');
 const unit=metric==='known_cost_usd'?t("knownUsage"):metric==='total_tokens'?t("tokensIncludingCache"):t("recordedEvents");
 return <Card data-motion="usage-network" className="gap-0 overflow-hidden border-slate-800 bg-[#080c16] p-0 text-slate-100 shadow-xl shadow-slate-950/10">
  <div className="flex flex-wrap items-start justify-between gap-5 border-b border-white/[.07] p-5 sm:p-7">
   <div><div className="mb-3 flex items-center gap-2 text-xs font-medium uppercase tracking-wider text-slate-300"><Box className="size-3.5 text-cyan-300"/> {t("insights")} <span className="ml-2 rounded border border-white/15 px-1.5 py-0.5 text-[9px] tracking-widest">3D</span></div><h2 className="text-2xl font-medium tracking-tight text-white">{t("network")}</h2><p className="mt-2 text-sm text-slate-300">{t("sourcesModels")}</p></div>
   <div className="flex flex-wrap items-center gap-3">{!expanded&&<Button asChild variant="outline" className="border-white/20 bg-white/10 text-white hover:bg-white/20 hover:text-white"><a href={`#insights?view=network&metric=${metric}`}><Expand className="size-4"/>{t("viewAll")}</a></Button>}<ToggleGroup type="single" value={metric} onValueChange={value=>{if(value)setMetric(value as Metric);}} aria-label={t("networkMetric")} className="flex-wrap rounded-lg border border-white/10 bg-white/5 p-1">
    {(['known_cost_usd','total_tokens','calls'] as const).map((value,i)=><ToggleGroupItem key={value} value={value} className="h-8 px-3 text-xs text-slate-300 hover:bg-white/5 hover:text-white data-[state=on]:bg-white/10 data-[state=on]:text-white">{[t("cost"),t("tokens"),t("events")][i]}</ToggleGroupItem>)}
   </ToggleGroup></div>
  </div>
  {expanded&&<div className="flex flex-wrap items-center gap-4 border-b border-white/10 px-5 py-3 sm:px-7"><span className="text-sm font-medium text-slate-200">{t("cameraView")}</span><ToggleGroup type="single" value={view} onValueChange={value=>{if(value)setView(value as typeof view);}} aria-label={t("cameraView")} className="flex-wrap">{(["perspective","front","side","above"] as const).map(value=><ToggleGroupItem key={value} value={value} className="text-sm capitalize text-slate-300 hover:bg-white/10 hover:text-white data-[state=on]:bg-white/15 data-[state=on]:text-white">{t(value)}</ToggleGroupItem>)}</ToggleGroup><span className="text-sm text-slate-300">{t("dragAngle")}</span><Button variant="outline" aria-expanded={showDetails} onClick={()=>setShowDetails(value=>!value)} className="ml-auto border-white/20 bg-white/5 text-slate-100 hover:bg-white/10 hover:text-white">{showDetails ? t("hideDetails") : t("showDetails")}</Button></div>}
  <div className={`grid min-w-0 ${showDetails ? expanded ? "lg:grid-cols-[minmax(0,1fr)_320px]" : "xl:grid-cols-[minmax(0,1fr)_280px]" : "grid-cols-1"}`}>
   <div className="relative min-w-0 bg-[radial-gradient(ellipse_at_50%_45%,#18223a_0%,#0c1220_42%,#080c16_75%)]">
    <div className="pointer-events-none absolute inset-x-5 top-5 z-10 flex flex-wrap items-start justify-between gap-2 sm:inset-x-7"><div><p className="text-xs uppercase tracking-wider text-slate-300">{unit}</p><p className="mt-1 text-2xl font-light tabular-nums tracking-tight text-slate-100">{format(total)}</p></div><span className="mt-1 text-xs uppercase tracking-widest text-slate-300">{t("sourcesModels")}</span></div>
    <Suspense fallback={<div className="motion-loading flex h-[440px] items-center justify-center text-sm text-slate-300 sm:h-[530px]">{t("sceneLoading")}</div>}><Scene graph={graph} metric={metric} selected={active.id} onSelect={selectNode} paused={paused} reset={reset} expanded={expanded} view={view}/></Suspense>
    <div className="flex flex-wrap items-center justify-between gap-3 px-5 pb-5 sm:px-7"><div className="flex flex-wrap gap-4">{sourceNodes.map(node=><Button variant="ghost" key={node.id} type="button" onClick={()=>selectNode(node.id)} className="h-auto gap-2 rounded p-0 text-xs font-normal text-slate-300 hover:bg-transparent hover:text-white"><span className="size-1.5 rounded-full" style={{background:node.color}}/>{node.label}</Button>)}</div><div className="flex gap-1"><Button size="icon" variant="ghost" aria-label={paused?t("resume"):t("pause")} onClick={()=>setPaused(value=>!value)} className="size-8 text-slate-300 hover:bg-white/10 hover:text-white">{paused?<Play className="size-3.5"/>:<Pause className="size-3.5"/>}</Button><Button size="icon" variant="ghost" aria-label={t("resetView")} onClick={()=>setReset(value=>value+1)} className="size-8 text-slate-300 hover:bg-white/10 hover:text-white"><RotateCcw className="size-3.5"/></Button></div></div>
   </div>
   {showDetails&&<aside className={`min-w-0 border-t border-white/[.07] bg-white/[.025] p-5 sm:p-6 ${expanded ? "lg:border-t-0 lg:border-l" : "xl:border-t-0 xl:border-l"}`}>
    <label htmlFor="network-node" className="mb-2 block text-xs font-medium uppercase tracking-wider text-slate-300">{t("inspect")}</label>
    <Select value={active.id} onValueChange={selectNode}><SelectTrigger id="network-node" className="w-full border-white/10 bg-white/5 text-xs text-slate-200 [&>span]:truncate"><SelectValue/></SelectTrigger><SelectContent className="max-h-80">{graph.nodes.map(node=><SelectItem key={node.id} value={node.id}>{node.kind==='model'||node.kind==='group'?`${sourceNodes.find(s=>s.source===node.source)?.label} · `:''}{node.label}</SelectItem>)}</SelectContent></Select>
    <div className="mt-7" aria-live="polite"><div className="flex items-center gap-2 text-xs uppercase tracking-widest text-slate-300"><span className="size-1.5 rounded-full" style={{background:active.color}}/>{active.kind==='total'?t("entireNetwork"):active.kind==='group'?t("groupedModels"):t(active.kind)}</div><h3 className="mt-3 break-words text-lg font-medium leading-snug text-white">{active.label}</h3><p className="mt-4 break-words text-2xl font-light tracking-tight tabular-nums" style={{color:active.color}}>{format(active.usage[metric])}</p><p className="mt-2 text-xs text-slate-300">{t(metric==='known_cost_usd'?'shareCost':metric==='calls'?'shareEvents':'shareTokens', { value: formatNumber(share/100, { style: 'percent', minimumFractionDigits: 1, maximumFractionDigits: 1 }) })}</p>
     <div className="mt-5 h-1 overflow-hidden rounded-full bg-white/5"><div className="h-full rounded-full transition-[width] duration-300 motion-reduce:transition-none" style={{width:`${Math.min(share,100)}%`,background:active.color}}/></div>
     <dl data-motion="usage-network-details" data-motion-reveal="false" data-motion-update={JSON.stringify([active.usage.calls, active.usage.total_tokens, active.usage.known_cost_usd, active.usage.unknown_cost_calls])} className="mt-7 space-y-4 text-xs">{[["calls",t("events"),formatNumber(active.usage.calls)],["tokens",t("tokens"),formatNumber(active.usage.total_tokens)],["value",t("knownValue"),fmt(active.usage.known_cost_usd)],["unpriced",t("unpricedEvents"),formatNumber(active.usage.unknown_cost_calls)]].map(([id,label,value])=><div key={id} className="flex flex-wrap justify-between gap-2"><dt className="text-slate-300">{label}</dt><dd className="tabular-nums text-slate-200">{value}</dd></div>)}</dl>
     {active.models&&<p className="mt-5 break-words text-xs leading-5 text-slate-300">{t('includesModels', { models: active.models.join(', ') })}</p>}
    </div>
    <p className="mt-7 flex gap-2 border-t border-white/10 pt-5 text-xs leading-5 text-slate-300"><MoveUpRight className="mt-1 size-3 shrink-0"/>{t("nodeHelp")}</p>
   </aside>}
  </div>
  <div className="flex flex-wrap justify-between gap-2 border-t border-white/[.07] px-5 py-3 text-xs leading-5 text-slate-300 sm:px-7"><span>{t("orbitHelp")}</span><span>{expanded ? t("allRecordedModels") : t("topModels")} · {t("costEstimates")}</span></div>
 </Card>;
}
