export interface OrderedRow {
  id: string
}

/** Move a set as one block, preserving its existing relative order. */
export function moveRowsBefore<T extends OrderedRow>(
  rows: readonly T[],
  ids: readonly string[],
  anchor: string | null,
): T[] {
  const chosen = new Set(ids)
  const moving = rows.filter((row) => chosen.has(row.id))
  if (
    !moving.length ||
    (anchor !== null && (!rows.some((row) => row.id === anchor) || chosen.has(anchor)))
  )
    return [...rows]
  const remaining = rows.filter((row) => !chosen.has(row.id))
  const at = anchor === null ? remaining.length : remaining.findIndex((row) => row.id === anchor)
  return [...remaining.slice(0, at), ...moving, ...remaining.slice(at)]
}

/** Sortable moves one DOM row; turn that drop into a stable multi-row move. */
export function applyRowDrop<T extends OrderedRow>(
  before: readonly T[],
  current: readonly T[],
  draggedId: string,
  newIndex: number,
  selected: readonly string[],
): T[] {
  const currentById = new Map(current.map((row) => [row.id, row]))
  if (
    currentById.size !== current.length ||
    before.length !== current.length ||
    before.some((row) => !currentById.has(row.id))
  )
    return [...current]
  const original = before.map((row) => currentById.get(row.id)!)
  if (before.findIndex((row) => row.id === draggedId) === newIndex) return [...current]
  const dragged = original.find((row) => row.id === draggedId)
  if (!dragged) return [...current]
  const single = original.filter((row) => row.id !== draggedId)
  const at = Math.max(0, Math.min(newIndex, single.length))
  single.splice(at, 0, dragged)
  const moving = new Set(selected.includes(draggedId) ? selected : [draggedId])
  const anchor = single.slice(at + 1).find((row) => !moving.has(row.id))?.id ?? null
  return moveRowsBefore(original, [...moving], anchor)
}

export function moveRowsOneStep<T extends OrderedRow>(
  rows: readonly T[],
  ids: readonly string[],
  direction: -1 | 1,
): T[] {
  const chosen = new Set(ids)
  const indices = rows.flatMap((row, index) => (chosen.has(row.id) ? [index] : []))
  if (!indices.length) return [...rows]
  if (direction < 0) {
    const previous = rows.slice(0, indices[0]).findLast((row) => !chosen.has(row.id))
    return previous ? moveRowsBefore(rows, ids, previous.id) : [...rows]
  }
  const next = rows.findIndex(
    (row, index) => index > indices[indices.length - 1]! && !chosen.has(row.id),
  )
  if (next < 0) return [...rows]
  return moveRowsBefore(
    rows,
    ids,
    rows.slice(next + 1).find((row) => !chosen.has(row.id))?.id ?? null,
  )
}
