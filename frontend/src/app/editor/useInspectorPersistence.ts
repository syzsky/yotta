import { nextTick, onScopeDispose } from 'vue'

export function commitActiveInspectorInput(root: HTMLElement | null | undefined): void {
  const active = document.activeElement
  if (root && active instanceof HTMLElement && root.contains(active)) active.blur()
}

// Input widgets own parsing and IME composition. Flush their native blur before
// navigation replaces them, then persist the resulting editor commands.
export function useInspectorPersistence(options: {
  root: () => HTMLElement | null | undefined
  isDirty: () => boolean
  save: () => Promise<boolean>
}) {
  let changed = false
  let queued: ReturnType<typeof setTimeout> | undefined
  let saving: Promise<boolean> | undefined

  async function commit(): Promise<boolean> {
    commitActiveInspectorInput(options.root())
    await nextTick()
    return !options.root()?.querySelector('[aria-invalid="true"]')
  }

  async function persist(): Promise<boolean> {
    clearTimeout(queued)
    await nextTick()
    if (options.root()?.querySelector('[aria-invalid="true"]')) return false
    if (saving) return saving
    if (!changed || !options.isDirty()) return true
    changed = false
    saving = options.save()
    try {
      const ok = await saving
      if (!ok) changed = true
      return ok
    } finally {
      saving = undefined
      if (changed && !options.isDirty()) changed = false
    }
  }

  async function flush(): Promise<boolean> {
    if (!(await commit())) return false
    return persist()
  }

  function schedule(): void {
    clearTimeout(queued)
    queued = setTimeout(() => {
      void persist()
    }, 0)
  }

  function markChanged(): void {
    changed = true
    const active = document.activeElement
    if (
      !(active instanceof HTMLElement) ||
      !options.root()?.contains(active) ||
      !active.matches('input,textarea,[contenteditable="true"]')
    )
      schedule()
  }

  function beforePointerDown(event: PointerEvent): void {
    const root = options.root()
    if (event.target instanceof Node && !root?.contains(event.target))
      commitActiveInspectorInput(root)
  }

  function afterFocusOut(event: FocusEvent): void {
    if (event.target instanceof Node && options.root()?.contains(event.target)) schedule()
  }

  document.addEventListener('pointerdown', beforePointerDown, true)
  document.addEventListener('focusout', afterFocusOut, true)
  onScopeDispose(() => {
    clearTimeout(queued)
    document.removeEventListener('pointerdown', beforePointerDown, true)
    document.removeEventListener('focusout', afterFocusOut, true)
  })
  return { commit, flush, markChanged }
}
