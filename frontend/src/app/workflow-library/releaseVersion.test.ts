import { describe, expect, it } from 'vitest'
import { validReleaseVersion } from './releaseVersion'

describe('workflow release version', () => {
  it.each(['0.0.0', '1.0.0', '12.34.567'])('accepts three canonical integers: %s', (value) => {
    expect(validReleaseVersion(value)).toBe(true)
  })
  it.each([
    '',
    '1',
    '1.2',
    '1.2.3.4',
    '-1.0.0',
    '1.0.-1',
    '01.2.3',
    '1.0.0-beta',
    '1.0.0+build',
    '1. 0.0',
    'a.b.c',
    '1.0.0\n',
  ])('rejects malformed versions: %s', (value) => {
    expect(validReleaseVersion(value)).toBe(false)
  })
})
