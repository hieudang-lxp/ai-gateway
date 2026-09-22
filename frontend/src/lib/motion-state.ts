export function createMotionState() {
  const entered = new Set<string>();
  const versions = new Map<string, string>();
  return {
    enter(id: string, order: number, reduced: boolean): number | undefined {
      if (entered.has(id)) return;
      entered.add(id);
      if (!reduced) return Math.min(Math.max(order, 0), 3) * 55;
    },
    update(id: string, version: string, reduced: boolean): boolean {
      const previous = versions.get(id);
      versions.set(id, version);
      return !reduced && previous !== undefined && previous !== version;
    },
  };
}
