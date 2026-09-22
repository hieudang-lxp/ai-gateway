import { useTranslation } from "react-i18next";
import { changeLanguage } from "@/i18n";
import { languages, useLocale, type Language } from "@/i18n/locale";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";

export function LanguageSelect() {
  const { t } = useTranslation("common");
  const language = useLocale();
  const selected = languages.find(item => item.code === language)!;
  return <Select value={language} onValueChange={value => { void changeLanguage(value as Language); }}>
    <SelectTrigger aria-label={t("language")} className="min-w-32 max-w-full text-sky-950">
      <SelectValue><span lang={selected.code}>{selected.name}</span></SelectValue>
    </SelectTrigger>
    <SelectContent position="popper" align="end" sideOffset={6}>
      {languages.map(item => <SelectItem key={item.code} value={item.code} textValue={item.name}><span lang={item.code}>{item.name}</span></SelectItem>)}
    </SelectContent>
  </Select>;
}
