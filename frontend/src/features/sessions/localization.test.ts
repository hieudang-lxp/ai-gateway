import { afterEach, expect, it } from 'vitest';
import i18n from '@/i18n';
import { sessionTitle } from './format';
import { changeLabel } from './query';
import { usageNetwork } from '../insights/network';

afterEach(async () => { await i18n.changeLanguage('en'); });

it('retranslates generated titles and comparisons while preserving real titles and model IDs', async () => {
  const session = { title: '', source: 'codex', models: ['gpt-model'] };
  await i18n.changeLanguage('en');
  expect(sessionTitle(session)).toBe('Untitled Codex session');
  expect(changeLabel(0, 0)).toBe('No change');
  await i18n.changeLanguage('de');
  expect(sessionTitle(session)).toBe('Unbenannte Codex-Sitzung');
  expect(sessionTitle({ ...session, title: 'My real session title' })).toBe('My real session title');
  expect(session.models).toEqual(['gpt-model']);
  expect(changeLabel(0, 0)).toBe('Keine Änderung');
  expect(changeLabel(15, 10)).toContain('+50');
  expect(changeLabel(15, 10)).toContain('Vorzeitraum');
});

it('translates generated graph labels without changing graph identities or totals', async () => {
  const rows = [{ source: 'cursor', model: '', calls: 1, total_tokens: 0, known_cost_usd: 0, unknown_cost_calls: 1 }];
  await i18n.changeLanguage('en');
  const english = usageNetwork(rows, 'calls');
  await i18n.changeLanguage('de');
  const german = usageNetwork(rows, 'calls');
  expect(german.nodes[0].label).toBe('Gesamte Nutzung');
  expect(german.nodes[2].label).toBe('Modell nicht erfasst');
  expect(german.nodes.map(node => node.id)).toEqual(english.nodes.map(node => node.id));
  expect(german.nodes[0].usage).toEqual(english.nodes[0].usage);
});
