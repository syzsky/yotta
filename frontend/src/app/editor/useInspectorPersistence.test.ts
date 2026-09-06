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
