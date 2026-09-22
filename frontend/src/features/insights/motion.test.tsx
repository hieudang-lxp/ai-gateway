import { afterEach, expect, it, vi } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import i18n from '@/i18n';
import { CurrencyCtx } from '../currency/useCurrency';
import type { InsightsResponse } from '../sessions/types';
import { Insights } from './Insights';

vi.mock('../sessions/hooks', () => ({ useInsights: () => ({ data, isPending: false, error: null }) }));

const totals = { sessions: 2, calls: 25, total_tokens: 1200, known_cost_usd: 4.5, unknown_cost_calls: 0 };
const data: InsightsResponse = {
  totals, previous: { ...totals, calls: 15 }, network: [], daily: [], top_sessions: [], findings: [],
  time_zone: 'UTC', date_from: '2026-09-01', date_to: '2026-09-22', since: '2026-09-01', until: '2026-09-22',
  previous_since: '2026-08-01', unassigned_events: 0, indexed_at: '', generated_at: '',
};
const render = () => renderToStaticMarkup(<CurrencyCtx.Provider value={{ currency: 'USD', toggle() {}, rate: null, fetchedAt: null, fmt: value => `$${value}` }}><Insights period="month" setPeriod={() => {}} /></CurrencyCtx.Provider>);
const markers = (html: string) => [...html.matchAll(/data-motion(?:-update)?="[^"]*"/g)].map(match => match[0]);

afterEach(async () => { data.totals.calls = 25; data.indexed_at = ''; await i18n.changeLanguage('en'); });

it('keeps reveal identities and update signatures stable across locale and index timestamp changes', async () => {
  await i18n.changeLanguage('en');
  const english = markers(render());
  expect(english).toContain('data-motion="insights-stat-calls"');
  expect(english).toContain('data-motion-update="25:15"');
  data.indexed_at = '2026-09-22T12:00:00Z';
  await i18n.changeLanguage('de');
  expect(markers(render())).toEqual(english);
});

it('changes only the affected numeric stat signature when its value changes', () => {
  const before = markers(render());
  data.totals.calls = 26;
  const after = markers(render());
  expect(after).toContain('data-motion-update="26:15"');
  expect(after.filter(value => !before.includes(value))).toEqual(['data-motion-update="26:15"']);
});
