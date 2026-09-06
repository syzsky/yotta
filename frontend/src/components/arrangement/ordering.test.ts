import { describe, expect, it } from 'vitest'
import { applyRowDrop, moveRowsBefore, moveRowsOneStep } from './ordering'
const rows = ['a', 'b', 'c', 'd', 'e'].map((id) => ({ id, value: id }))
const ids = (items: typeof rows) => items.map((row) => row.id)
describe('stable table arrangement', () => {
  it('moves non-adjacent selected rows as a block in their original order', () => {
    expect(ids(moveRowsBefore(rows, ['d', 'b'], 'a'))).toEqual(['b', 'd', 'a', 'c', 'e'])
    expect(ids(moveRowsBefore(rows, ['b', 'd'], null))).toEqual(['a', 'c', 'e', 'b', 'd'])
    expect(ids(rows)).toEqual(['a', 'b', 'c', 'd', 'e'])
  })
  it('turns a selected row drop into a group move without losing current edits', () => {
    const current = [rows[0]!, rows[2]!, rows[3]!, rows[4]!, { id: 'b', value: 'edited' }]
    const result = applyRowDrop(rows, current, 'b', 4, ['b', 'd'])
    expect(ids(result)).toEqual(['a', 'c', 'e', 'b', 'd'])
    expect(result.find((row) => row.id === 'b')?.value).toBe('edited')
  })
  it('moves only an unselected dragged row and rejects stale anchors', () => {
    expect(ids(applyRowDrop(rows, rows, 'c', 0, ['b', 'd']))).toEqual(['c', 'a', 'b', 'd', 'e'])
    expect(moveRowsBefore(rows, ['b'], 'deleted')).toEqual(rows)
    expect(moveRowsBefore(rows, ['b'], 'b')).toEqual(rows)
  })
  it('keeps a drop back at its original position unchanged', () => {
    expect(applyRowDrop(rows, rows, 'b', 1, ['b', 'd'])).toEqual(rows)
  })
  it('does not resurrect rows deleted during a drag', () => {
    const current = rows.filter((row) => row.id !== 'c')
    expect(applyRowDrop(rows, current, 'b', 3, ['b', 'd'])).toEqual(current)
  })
  it('supports keyboard moves without changing selected-row order', () => {
    expect(ids(moveRowsOneStep(rows, ['b', 'd'], -1))).toEqual(['b', 'd', 'a', 'c', 'e'])
    expect(ids(moveRowsOneStep(rows, ['b', 'd'], 1))).toEqual(['a', 'c', 'e', 'b', 'd'])
  })
})
