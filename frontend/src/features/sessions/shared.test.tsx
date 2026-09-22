import { afterEach, expect, it } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import i18n from '@/i18n';
import { Coverage, LoadError } from './shared';
import { IndexedUsageError } from './errors';

afterEach(async () => { await i18n.changeLanguage('en'); });

it.each([
  ['vi', 'Dữ liệu chỉ mục này chưa có.'],
  ['ko', '색인된 데이터를 아직 사용할 수 없습니다.'],
  ['zh-Hans', '此索引数据尚不可用。'],
  ['de', 'Diese indexierten Daten sind noch nicht verfügbar.'],
])('retranslates the same cached HTTP error in %s', async (locale, expected) => {
  const error = new IndexedUsageError(404);
  await i18n.changeLanguage('en');
  expect(renderToStaticMarkup(<LoadError error={error} retry={() => {}} />)).toContain('This indexed data is not available yet.');
  await i18n.changeLanguage(locale);
  expect(renderToStaticMarkup(<LoadError error={error} retry={() => {}} />)).toContain(expected);
  expect(error.message).toBe('HTTP 404');
});

it('keeps unknown diagnostics verbatim below a translated summary', async () => {
  await i18n.changeLanguage('de');
  const html = renderToStaticMarkup(<LoadError error={new Error('Upstream diagnostic XYZ')} retry={() => {}} />);
  expect(html).toContain('Nutzungsdaten konnten nicht geladen werden.');
  expect(html).toContain('Technische Details');
  expect(html).toContain('Upstream diagnostic XYZ');
});

it('uses a singular event label and localized large numbers in coverage', async () => {
  await i18n.changeLanguage('en');
  expect(renderToStaticMarkup(<Coverage unassigned={1} indexedAt="" />)).toContain('1 event has no session attribution');
  await i18n.changeLanguage('de');
  expect(renderToStaticMarkup(<Coverage unassigned={1234} indexedAt="" />)).toContain('1.234 Ereignisse');
});
