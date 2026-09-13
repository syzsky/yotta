import { describe, expect, it } from 'vitest'
import {
  blankPath,
  parsePathDocument,
  previewPoints,
  sampleIssue,
  sufficientlyDistant,
  type PositionSample,
} from './pathModel'

describe('path recording and editing', () => {
  it('does not connect expired, replayed or changed sessions', () => {
    const path = blankPath()
    path.reference.kind = 'world'
    path.points.push({ id: 'a', name: '', x: 1, y: 2, z: null })
    const sample: PositionSample = {
      reference: path.reference,
      point: path.points[0]!,
      epoch: 'one',
      recovery: 'stable',
      sequence: 2,
      sampleTimeMs: 1000,
    }
    expect(sampleIssue(path, null, sample, 1100)).toBeNull()
    expect(sampleIssue(path, sample, sample, 1100)).toBe('replay')
    expect(sampleIssue(path, null, sample, 1700)).toBe('stale')
    expect(sampleIssue(path, sample, { ...sample, epoch: 'two', sequence: 3 }, 1100)).toBe(
      'reference',
    )
    expect(sampleIssue(path, null, { ...sample, recovery: 'session' }, 1100)).toBe('reference')
    expect(
      sampleIssue(path, null, { ...sample, reference: { ...sample.reference, floor: '2' } }, 1100),
    ).toBe('reference')
  })
  it('deduplicates without treating unknown altitude as zero', () => {
    const p = { id: 'a', name: '', x: 1, y: 2, z: null }
    expect(sufficientlyDistant(p, { ...p, z: 1000 }, 1)).toBe(false)
    expect(sufficientlyDistant(p, { ...p, x: 3 }, 1)).toBe(true)
  })
  it('validates imports and supports long paths without a fixed waypoint count', () => {
    const path = blankPath()
    path.points = Array.from({ length: 20000 }, (_, i) => ({
      id: String(i),
      name: '',
      x: i,
      y: i % 2,
      z: null,
    }))
    const loaded = parsePathDocument(JSON.stringify(path))
    expect(loaded.points).toHaveLength(20000)
    expect(
      previewPoints(loaded.points).every((p) => Number.isFinite(p.x) && Number.isFinite(p.y)),
    ).toBe(true)
    path.points[1]!.id = '0'
    expect(() => parsePathDocument(JSON.stringify(path))).toThrow('path.invalid')
    expect(() => parsePathDocument(JSON.stringify({ ...blankPath(), reference: {} }))).toThrow(
      'path.invalid',
    )
  })
})
