const KEY = "dashboard_token";
let volatileToken: string | undefined;

export function getToken(): string {
  if (volatileToken !== undefined) return volatileToken;
  try { return localStorage.getItem(KEY) ?? ""; } catch { return ""; }
}
export function setToken(t: string): void {
  try { localStorage.setItem(KEY, t); volatileToken = undefined; } catch { volatileToken = t; }
}
export function clearToken(): void {
  try { localStorage.removeItem(KEY); volatileToken = undefined; } catch { volatileToken = ""; }
}
