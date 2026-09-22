import { useTranslation } from 'react-i18next';
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
  const { t } = useTranslation('sessions');
  const [open, setOpen] = useState(false);
  function choose(model: string) {
    onChange(model);
    setOpen(false);
  }
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button id="session-model" type="button" variant="outline" role="combobox" aria-expanded={open} disabled={loading || failed} className="h-11 w-full min-w-0 justify-between font-normal">
          <span className={loading ? "motion-loading truncate" : "truncate"}>{loading ? t("modelsLoading") : failed ? t("modelsUnavailable") : value || t("allModels")}</span>
          <ChevronsUpDown className="size-4 shrink-0 opacity-50" />
        </Button>
      </PopoverTrigger>
      <PopoverContent align="start" className="w-80 min-w-[var(--radix-popover-trigger-width)] max-w-[calc(100vw-2rem)] p-0">
        <Command>
          <CommandInput placeholder={t("searchModels")} aria-label={t("searchModels")} />
          <CommandList>
            <CommandEmpty>{t("noModels")}</CommandEmpty>
            <CommandGroup>
              <CommandItem value={t("allModels")} onSelect={() => choose("")}>
                <Check className={`size-4 shrink-0 ${value ? "invisible" : ""}`} />{t("allModels")}
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
