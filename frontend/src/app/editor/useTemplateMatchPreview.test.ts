import { effectScope, ref } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'
const preview = vi.hoisted(() => vi.fn())
vi.mock('@/lib/backend', () => ({ backend: { tools: { previewTemplate: preview } } }))
vi.mock('@/lib/invoke', () => ({ errorMessage: (error: Error) => error.message }))
import { useTemplateMatchPreview } from './useTemplateMatchPreview'
import type { TemplateMatchPreviewRequest } from '@/lib/backend'

afterEach(() => {
  vi.useRealTimers()
  preview.mockReset()
})
const request = {
  targetSlot: 'game',
  threshold: 0.85,
  template: { mediaType: 'image/png', digest: `sha256:${'a'.repeat(64)}`, size: 10 },
  variants: [],
  region: { x: 0, y: 0, width: 1, height: 1, unit: 'ratio' },
} satisfies TemplateMatchPreviewRequest
const score = { score: 0.82, matched: false, frameWidth: 1920, frameHeight: 1080 }

describe('live template preview lifecycle', () => {
  it('polls sequentially and stops when the panel closes', async () => {
    vi.useFakeTimers()
    let resolve!: (value: typeof score) => void
    preview
      .mockImplementationOnce(
        () =>
          new Promise((done) => {
            resolve = done
          }),
      )
      .mockResolvedValue(score)
    const scope = effectScope()
    const state = scope.run(() => useTemplateMatchPreview(ref(request)))!
    expect(preview).not.toHaveBeenCalled()
    state.enabled.value = true
    await vi.advanceTimersByTimeAsync(5000)
    expect(preview).toHaveBeenCalledTimes(1)
    resolve(score)
    await vi.advanceTimersByTimeAsync(0)
    expect(state.result.value?.score).toBe(0.82)
    await vi.advanceTimersByTimeAsync(750)
    expect(preview).toHaveBeenCalledTimes(2)
    scope.stop()
    await vi.advanceTimersByTimeAsync(5000)
    expect(preview).toHaveBeenCalledTimes(2)
  })
  it('discards stale responses after changing threshold and after stopping', async () => {
    vi.useFakeTimers()
    let resolve!: (value: typeof score) => void
    preview
      .mockImplementationOnce(
        () =>
          new Promise((done) => {
            resolve = done
          }),
      )
      .mockResolvedValue({ ...score, matched: true })
    const value = ref(request)
    const scope = effectScope()
    const state = scope.run(() => useTemplateMatchPreview(value))!
    state.enabled.value = true
    value.value = { ...request, threshold: 0.8 }
    resolve(score)
    await vi.advanceTimersByTimeAsync(0)
    expect(state.result.value).toBeNull()
    await vi.advanceTimersByTimeAsync(100)
    expect(state.result.value?.matched).toBe(true)
    state.enabled.value = false
    expect(state.result.value).toBeNull()
    await vi.advanceTimersByTimeAsync(5000)
    expect(preview).toHaveBeenCalledTimes(2)
    scope.stop()
  })
})
