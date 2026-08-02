// Generic comparator for client-side table sorting (SortableHeader-driven
// lists) — strings compare locale-aware (fr), everything else falls back to
// `<`/`>`, nullish values always sort last regardless of direction.
export function compareValues(a: unknown, b: unknown, direction: 'asc' | 'desc' = 'asc'): number {
  const dir = direction === 'asc' ? 1 : -1
  if (a == null && b == null) return 0
  if (a == null) return 1 * dir
  if (b == null) return -1 * dir
  if (typeof a === 'string' || typeof b === 'string') {
    return String(a).localeCompare(String(b), 'fr', { sensitivity: 'base' }) * dir
  }
  if (a < b) return -1 * dir
  if (a > b) return 1 * dir
  return 0
}
