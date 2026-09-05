import { describe, it, expect, vi } from 'vitest'
import { usePublicationEntry } from './usePublicationEntry'
const deferred = () => {
  let resolve!: () => void
  const promise = new Promise<void>((done) => {
    resolve = done
  })
  return { promise, resolve }
}
describe('publication authentication entry', () => {
  it('waits for authentication before opening a form and suppresses duplicate entry', async () => {
    const gate = deferred(),
      ready = vi.fn(),
      login = vi.fn(() => gate.promise)
    const entry = usePublicationEntry({ login, cancel: async () => {}, message: String })
    const work = entry.start(ready)
    await entry.start(ready)
    expect(ready).not.toHaveBeenCalled()
    expect(login).toHaveBeenCalledTimes(1)
    gate.resolve()
    await work
    expect(ready).toHaveBeenCalledTimes(1)
    expect(entry.open.value).toBe(false)
  })
  it('does not open the form after a cancelled login resolves late', async () => {
    const gate = deferred(),
      ready = vi.fn()
    const entry = usePublicationEntry({
      login: () => gate.promise,
      cancel: async () => {},
      message: String,
    })
    const work = entry.start(ready)
    await entry.cancel()
    gate.resolve()
    await work
    expect(ready).not.toHaveBeenCalled()
    expect(entry.busy.value).toBe(false)
  })
  it('retains an actionable error and allows retry', async () => {
    const ready = vi.fn(),
      login = vi.fn().mockRejectedValueOnce(new Error('login failed')).mockResolvedValueOnce({})
    const entry = usePublicationEntry({ login, cancel: async () => {}, message: String })
    await entry.start(ready)
    expect(entry.failure.value).toContain('login failed')
    expect(entry.open.value).toBe(true)
    await entry.start(ready)
    expect(ready).toHaveBeenCalledTimes(1)
  })
})
