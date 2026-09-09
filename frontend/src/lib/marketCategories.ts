export interface MarketCategory {
  icon?: string
  imageMediaKey?: string
  key: string
  name: string
  parentKey: string
  active: boolean
  position?: number
}

export function categoryRows(categories: MarketCategory[]) {
  const byKey = new Map(categories.map((item) => [item.key, item]))
  const children = new Map<string, MarketCategory[]>()
  for (const item of byKey.values()) {
    const parent = byKey.has(item.parentKey) ? item.parentKey : ''
    children.set(parent, [...(children.get(parent) || []), item])
  }
  const compare = (a: MarketCategory, b: MarketCategory) =>
    (a.position || 0) - (b.position || 0) ||
    a.name.localeCompare(b.name) ||
    a.key.localeCompare(b.key)
  for (const group of children.values()) group.sort(compare)
  const rows: (MarketCategory & { label: string; depth: number; ancestors: string[] })[] = []
  const visited = new Set<string>()
  const visit = (roots: MarketCategory[]) => {
    const pending = roots
      .toReversed()
      .map((item) => ({ item, names: [] as string[], ancestors: [] as string[] }))
    while (pending.length) {
      const { item, names, ancestors } = pending.pop()!
      if (visited.has(item.key)) continue
      visited.add(item.key)
      const path = [...names, item.name]
      rows.push({ ...item, label: path.join(' / '), depth: ancestors.length, ancestors })
      for (const child of (children.get(item.key) || []).toReversed()) {
        pending.push({ item: child, names: path, ancestors: [...ancestors, item.key] })
      }
    }
  }
  visit(children.get('') || [])
  // Keep malformed/orphaned legacy entries discoverable; cycles never loop.
  for (const item of [...byKey.values()].sort(compare)) if (!visited.has(item.key)) visit([item])
  return rows
}
