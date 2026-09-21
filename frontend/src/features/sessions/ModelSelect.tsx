import { useState } from "react";
import { Check, ChevronsUpDown } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from "@/components/ui/command";

export function ModelSelect({ models, value, onChange, loading, failed }: {
  models: string[];
  value: string;
  onChange: (model: string) => void;
  loading: boolean;
  failed: boolean;
}) {
  const [open, setOpen] = useState(false);
  function choose(model: string) {
    onChange(model);
    setOpen(false);
  }
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button id="session-model" type="button" variant="outline" role="combobox" aria-expanded={open} disabled={loading || failed} className="h-11 w-full min-w-0 justify-between font-normal">
          <span className="truncate">{loading ? "Loading models…" : failed ? "Models unavailable" : value || "All models"}</span>
          <ChevronsUpDown className="size-4 shrink-0 opacity-50" />
        </Button>
      </PopoverTrigger>
      <PopoverContent align="start" className="w-80 min-w-[var(--radix-popover-trigger-width)] max-w-[calc(100vw-2rem)] p-0">
        <Command>
          <CommandInput placeholder="Search models…" aria-label="Search models" />
          <CommandList>
            <CommandEmpty>No matching models.</CommandEmpty>
            <CommandGroup>
              <CommandItem value="All models" onSelect={() => choose("")}>
                <Check className={`size-4 shrink-0 ${value ? "invisible" : ""}`} />All models
              </CommandItem>
              {models.map(model => (
                <CommandItem key={model} value={model} onSelect={() => choose(model)} className="items-start">
                  <Check className={`mt-0.5 size-4 shrink-0 ${value === model ? "" : "invisible"}`} />
                  <span className="min-w-0 break-all">{model}</span>
                </CommandItem>
              ))}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  );
}
