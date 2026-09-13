import type { BlobRef } from '@/lib/backend'

export interface PathReference {
  kind: 'world' | 'local'
  frame: string
  unit: string
  axisHeading: number
  axisSign: number
  map: string
  floor: string
}
export interface PathPoint {
  id: string
  name: string
  x: number
  y: number
  z: number | null
}
export interface NavigationPath {
  version: number
  reference: PathReference
  points: PathPoint[]
}
export interface SavedPath {
  guid: string
  name: string
  path: NavigationPath
  blob: BlobRef
}
export interface PositionSample {
  reference: PathReference
  point: PathPoint
  epoch: string
  sequence: number
  sampleTimeMs: number
  recovery: string
}

export function blankPath(): NavigationPath {
  return {
    version: 1,
    reference: {
      kind: 'local',
      frame: 'local',
      unit: 'm',
      axisHeading: 0,
      axisSign: 1,
      map: '',
      floor: '',
    },
    points: [],
  }
}
export function sameReference(a: PathReference, b: PathReference): boolean {
  return (
    a.kind === b.kind &&
    a.frame === b.frame &&
    a.unit === b.unit &&
    a.axisHeading === b.axisHeading &&
    a.axisSign === b.axisSign &&
    a.map === b.map &&
    a.floor === b.floor
  )
}
export function sampleIssue(
  path: NavigationPath,
  previous: PositionSample | null,
  sample: PositionSample,
  now: number,
): 'stale' | 'reference' | 'replay' | null {
  if (now - sample.sampleTimeMs > 500 || now - sample.sampleTimeMs < -100) return 'stale'
  if (path.points.length && !sameReference(path.reference, sample.reference)) return 'reference'
  if (path.points.length && !previous && sample.recovery !== 'stable') return 'reference'
  if (
    previous &&
    (sample.epoch !== previous.epoch || !sameReference(previous.reference, sample.reference))
  )
    return 'reference'
  if (
    previous &&
    (sample.sequence <= previous.sequence || sample.sampleTimeMs < previous.sampleTimeMs)
  )
    return 'replay'
  return null
}
export function sampledPoint(sample: PositionSample): PathPoint {
  return { ...sample.point, id: crypto.randomUUID(), name: '' }
}
export function sufficientlyDistant(a: PathPoint, b: PathPoint, distance: number): boolean {
  return Math.hypot(a.x - b.x, a.y - b.y, a.z !== null && b.z !== null ? a.z - b.z : 0) >= distance
}

export function parsePathDocument(text: string): NavigationPath {
  const value: unknown = JSON.parse(text)
  const object = (v: unknown): v is Record<string, unknown> =>
    typeof v === 'object' && v !== null && !Array.isArray(v)
  const keys = (v: Record<string, unknown>, expected: string[]) =>
    Object.keys(v).length === expected.length && expected.every((key) => key in v)
  const finite = (v: unknown): v is number => typeof v === 'number' && Number.isFinite(v)
  const bounded = (v: unknown, max: number): v is string =>
    typeof v === 'string' && v.trim().length > 0 && new TextEncoder().encode(v).length <= max
  if (
    !object(value) ||
    !keys(value, ['version', 'reference', 'points']) ||
    value.version !== 1 ||
    !object(value.reference) ||
    !Array.isArray(value.points)
  )
    throw new Error('path.invalid')
  const r = value.reference
  if (
    !keys(r, ['kind', 'frame', 'unit', 'axisHeading', 'axisSign', 'map', 'floor']) ||
    !['world', 'local'].includes(String(r.kind)) ||
    !bounded(r.frame, 128) ||
    !bounded(r.unit, 32) ||
    !finite(r.axisHeading) ||
    r.axisHeading < 0 ||
    r.axisHeading >= 360 ||
    (r.axisSign !== -1 && r.axisSign !== 1) ||
    typeof r.map !== 'string' ||
    typeof r.floor !== 'string'
  )
    throw new Error('path.invalid')
  const ids = new Set<string>()
  for (const p of value.points) {
    if (
      !object(p) ||
      !keys(p, ['id', 'name', 'x', 'y', 'z']) ||
      !bounded(p.id, 128) ||
      ids.has(p.id) ||
      typeof p.name !== 'string' ||
      !finite(p.x) ||
      !finite(p.y) ||
      (p.z !== null && !finite(p.z))
    )
      throw new Error('path.invalid')
    ids.add(p.id)
  }
  return value as unknown as NavigationPath
}
export function previewPoints(points: PathPoint[]): Array<{ id: string; x: number; y: number }> {
  if (!points.length) return []
  let minX = Infinity,
    minY = Infinity,
    maxX = -Infinity,
    maxY = -Infinity
  for (const p of points) {
    minX = Math.min(minX, p.x)
    minY = Math.min(minY, p.y)
    maxX = Math.max(maxX, p.x)
    maxY = Math.max(maxY, p.y)
  }
  const scale = Math.min(540 / Math.max(1, maxX - minX), 340 / Math.max(1, maxY - minY))
  return points.map((p) => ({
    id: p.id,
    x: 300 + (p.x - (minX + maxX) / 2) * scale,
    y: 200 - (p.y - (minY + maxY) / 2) * scale,
  }))
}
