import { describe, expect, it } from 'vitest'
import { validReleaseVersion, isNewerRelease, nextRelease } from './releaseVersion'

describe('workflow release version', () => {
  it('never treats lower or equal versions as updates', () => {
    expect(isNewerRelease('0.9.0', '1.0.2')).toBe(false)
    expect(isNewerRelease('1.0.2', '1.0.2')).toBe(false)
    expect(isNewerRelease('1.10.0', '1.9.0')).toBe(true)
    expect(isNewerRelease('99999999999999999999.0.0', '99999999999999999998.0.0')).toBe(true)
    expect(nextRelease('1.9.9', 1)).toBe('1.10.0')
  })
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
