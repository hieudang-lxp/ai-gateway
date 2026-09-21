export type ModelOption = { source: string; model: string };

export function modelsForSource(options: ModelOption[], source: string) {
  return [...new Set(options.filter(option => source === "all" || option.source === source).map(option => option.model))].sort();
}

export function modelForSource(options: ModelOption[], source: string, selected: string) {
  return modelsForSource(options, source).includes(selected) ? selected : "";
}
