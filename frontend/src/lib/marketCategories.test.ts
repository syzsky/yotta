import { describe, expect, it } from 'vitest'
import { categoryRows, type MarketCategory } from './marketCategories'

const category = (key: string, name: string, parentKey = '', position = 0): MarketCategory => ({
  key,
  name,
  parentKey,
  position,
  active: true,
})

describe('market category directory', () => {
  it('keeps ancestors without direct works and respects sibling order at every depth', () => {
    const rows = categoryRows([
      category('11', '11', 'nte'),
      category('deep', '深入', '11'),
      category('tools', '工具', '', 2),
      category('nte', '异环', '', 1),
    ])
    expect(rows.map((r) => [r.key, r.depth, r.label])).toEqual([
      ['nte', 0, '异环'],
      ['11', 1, '异环 / 11'],
      ['deep', 2, '异环 / 11 / 深入'],
      ['tools', 0, '工具'],
    ])
    expect(rows[2]?.ancestors).toEqual(['nte', '11'])
  })
  it('distinguishes equal child names by their paths without changing keys', () => {
    const rows = categoryRows([
      category('a', 'A'),
      category('b', 'B'),
      category('x', '11', 'a'),
      category('y', '11', 'b'),
    ])
    expect(rows.filter((r) => r.name === '11').map((r) => r.label)).toEqual(['A / 11', 'B / 11'])
  })
  it('terminates on cycles and preserves orphaned and inactive categories', () => {
    const rows = categoryRows([
      category('a', 'A', 'b'),
      category('b', 'B', 'a'),
      category('self', 'Self', 'self'),
      { ...category('orphan', 'Orphan', 'missing'), active: false },
    ])
    expect(rows).toHaveLength(4)
    expect(new Set(rows.map((r) => r.key)).size).toBe(4)
    expect(rows.find((r) => r.key === 'orphan')?.active).toBe(false)
  })
})
