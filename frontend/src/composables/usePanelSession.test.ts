import { effectScope, nextTick, ref } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { usePanelSession } from './usePanelSession'
import type { PanelSnapshot } from '@/lib/panels'
vi.mock('@/lib/panels', () => ({ panelBackend: {} }))
vi.mock('@/lib/invoke', () => ({ errorMessage: (e: Error) => e.message }))
const state = (sessionId = 'one', revision = 1): PanelSnapshot => ({
  protocol: 'yotta.panel-provider/v1',
  sessionId,
  revision,
  status: 'ready',
  values: { x: 1 },
  controlRevisions: { clear: 0 },
  records: {},
})
afterEach(() => vi.useRealTimers())
describe('panel session lifecycle', () => {
  it('drops the old panel response when switching tabs without overlapping reads', async () => {
    vi.useFakeTimers()
    let resolve!: (s: PanelSnapshot) => void
    const api = {
      list: vi.fn(),
      read: vi
        .fn()
        .mockImplementationOnce(
          () =>
            new Promise((r) => {
              resolve = r
            }),
        )
        .mockResolvedValue(state('two')),
      dispatch: vi.fn(),
    }
    const id = ref('first'),
      visible = ref(true),
      scope = effectScope()
    const session = scope.run(() => usePanelSession(id, visible, api))!
    id.value = 'second'
    await nextTick()
    expect(api.read).toHaveBeenCalledTimes(1)
    resolve(state('old'))
    await vi.advanceTimersByTimeAsync(260)
    expect(session.snapshot.value?.sessionId).toBe('two')
    visible.value = false
    await nextTick()
    const calls = api.read.mock.calls.length
    await vi.advanceTimersByTimeAsync(2000)
    expect(api.read).toHaveBeenCalledTimes(calls)
    scope.stop()
  })
  it('does not let an old action reply replace a restarted provider session', async () => {
    vi.useFakeTimers()
    let resolve!: (s: { eventId: string; snapshot: PanelSnapshot }) => void
    const api = {
      list: vi.fn(),
      read: vi.fn().mockResolvedValueOnce(state()).mockResolvedValue(state('restarted')),
      dispatch: vi.fn(
        () =>
          new Promise<{ eventId: string; snapshot: PanelSnapshot }>((r) => {
            resolve = r
          }),
      ),
    }
    const scope = effectScope(),
      session = scope.run(() => usePanelSession(ref('p'), ref(true), api))!
    await vi.advanceTimersByTimeAsync(1)
    const action = session.dispatch(
      { id: 'clear', kind: 'button', titleKey: 'clear', event: 'clear' },
      null,
    )
    await vi.advanceTimersByTimeAsync(260)
    resolve({ eventId: 'event', snapshot: state('one', 1000) })
    await action
    expect(session.snapshot.value?.sessionId).toBe('restarted')
    expect(api.dispatch).toHaveBeenCalledTimes(1)
    scope.stop()
  })
  it('reports failed actions without retrying or hiding existing data', async () => {
    vi.useFakeTimers()
    const api = {
      list: vi.fn(),
      read: vi.fn().mockResolvedValue(state()),
      dispatch: vi.fn().mockRejectedValue(Error('unknown result')),
    }
    const scope = effectScope(),
      session = scope.run(() => usePanelSession(ref('p'), ref(true), api))!
    await vi.advanceTimersByTimeAsync(1)
    await session.dispatch({ id: 'clear', kind: 'button', titleKey: 'clear', event: 'clear' }, null)
    expect(session.actionFailure.value).toBe('unknown result')
    expect(session.snapshot.value?.values.x).toBe(1)
    expect(api.dispatch).toHaveBeenCalledTimes(1)
    scope.stop()
  })
})
