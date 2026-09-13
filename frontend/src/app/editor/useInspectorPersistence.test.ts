import { effectScope } from 'vue'
import { afterEach, expect, it, vi } from 'vitest'
import { useInspectorPersistence } from './useInspectorPersistence'

const scopes: ReturnType<typeof effectScope>[] = []
afterEach(() => {
  scopes.splice(0).forEach((scope) => scope.stop())
  document.body.innerHTML = ''
  vi.useRealTimers()
})

function harness() {
  vi.useFakeTimers()
  const root = document.createElement('section')
  const input = document.createElement('input')
  const next = document.createElement('input')
  const outside = document.createElement('button')
  root.append(input, next)
  document.body.append(root, outside)
  let value = 'old'
  let stored = value
  const save = vi.fn(async () => {
    stored = value
    return true
  })
  const scope = effectScope()
  scopes.push(scope)
  const persistence = scope.run(() =>
    useInspectorPersistence({ root: () => root, isDirty: () => value !== stored, save }),
  )!
  input.addEventListener('blur', () => {
    value = input.value
    persistence.markChanged()
  })
  return { input, next, outside, persistence, save, value: () => value, stored: () => stored }
}

it('commits before a pointer navigation that prevents normal blur', async () => {
  const h = harness()
  h.input.focus()
  h.input.value = 'edited'
  h.outside.addEventListener('pointerdown', (event) => event.preventDefault())
  h.outside.dispatchEvent(new Event('pointerdown', { bubbles: true, cancelable: true }))
  expect(h.value()).toBe('edited')
  await expect(h.persistence.flush()).resolves.toBe(true)
  expect(h.stored()).toBe('edited')
})

it('saves on blur without blurring the next property field', async () => {
  const h = harness()
  h.input.focus()
  h.input.value = 'edited'
  h.next.focus()
  await vi.advanceTimersByTimeAsync(0)
  expect(h.stored()).toBe('edited')
  expect(document.activeElement).toBe(h.next)
})

it('blocks leaving on invalid input and retries a failed save', async () => {
  const h = harness()
  h.input.focus()
  h.input.value = 'edited'
  h.input.setAttribute('aria-invalid', 'true')
  await expect(h.persistence.flush()).resolves.toBe(false)
  expect(h.save).not.toHaveBeenCalled()
  h.input.removeAttribute('aria-invalid')
  h.save.mockResolvedValueOnce(false)
  await expect(h.persistence.flush()).resolves.toBe(false)
  expect(h.value()).toBe('edited')
  await expect(h.persistence.flush()).resolves.toBe(true)
  expect(h.stored()).toBe('edited')
})

it('allows an exit decision after invalid input or a rejected autosave', async () => {
  const h = harness()
  h.input.focus()
  h.input.value = 'edited'
  h.save.mockResolvedValue(false)
  await expect(h.persistence.flush()).resolves.toBe(false)
  h.input.setAttribute('aria-invalid', 'true')
  const decide = vi.fn(async (valid: boolean) => {
    expect(valid).toBe(false)
    expect(h.value()).toBe('edited')
    await vi.advanceTimersByTimeAsync(0)
    return 'discard'
  })
  const saves = h.save.mock.calls.length
  await expect(h.persistence.decideExit(decide)).resolves.toBe('discard')
  expect(decide).toHaveBeenCalledOnce()
  expect(h.save).toHaveBeenCalledTimes(saves)
})

it('waits for a running autosave before allowing discard without saving again', async () => {
  const h = harness()
  let finish!: (ok: boolean) => void
  h.save.mockImplementationOnce(
    () =>
      new Promise<boolean>((resolve) => {
        finish = resolve
      }),
  )
  h.input.focus()
  h.input.value = 'edited'
  h.next.focus()
  await vi.advanceTimersByTimeAsync(0)
  const decide = vi.fn(async () => 'discard')
  const exiting = h.persistence.decideExit(decide)
  await Promise.resolve()
  expect(decide).not.toHaveBeenCalled()
  finish(false)
  await expect(exiting).resolves.toBe('discard')
  expect(h.save).toHaveBeenCalledOnce()
})
