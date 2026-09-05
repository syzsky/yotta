import { describe, expect, it } from 'vitest'
import type { InstalledApplicationProfile, InstalledAutomationTargetProfile } from '@/lib/backend'
import { draftFromProfile, targetDraftComplete, targetMetadata } from './automationTargetDraft'

describe('automationTargetDraft', () => {
  it('round-trips a desktop target while keeping transient edit state local', () => {
    const installed = {
      slot: 'game',
      label: 'Game',
      targetKind: 'desktop-window',
      adapterKind: 'win32',
      profileVersion: '1',
      profile: {
        applicationSlot: 'game-app',
        windowTitle: 'Game',
        windowTitleMatch: 'exact',
        windowSelection: 'unique',
        windowClass: 'GameWindow',
        inputBackend: 'sendinput',
        captureBackend: 'wgc',
        mouseCounts360: 12000,
        resolveTimeoutMilliseconds: 3000,
      },
    } as InstalledAutomationTargetProfile
    const draft = draftFromProfile(installed)
    expect(draft.mouseCalibrationMode).toBe('custom')
    expect(
      targetDraftComplete(draft, [{ slot: 'game-app' } as InstalledApplicationProfile], undefined),
    ).toBe(true)
    expect(targetMetadata(draft)).toEqual(installed)
  })
})
