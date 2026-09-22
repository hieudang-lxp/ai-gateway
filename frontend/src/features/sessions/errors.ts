/** Stable error metadata can be translated when rendered, including from cache. */
export class IndexedUsageError extends Error {
  readonly status: number;
  constructor(status: number) {
    super(`HTTP ${status}`);
    this.name = 'IndexedUsageError';
    this.status = status;
  }
}
