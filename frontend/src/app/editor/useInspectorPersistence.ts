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
  let decidingExit = false

  async function commit(): Promise<boolean> {
    commitActiveInspectorInput(options.root())
    await nextTick()
    return !options.root()?.querySelector('[aria-invalid="true"]')
  }

  async function persist(): Promise<boolean> {
    clearTimeout(queued)
    await nextTick()
    if (decidingExit) return false
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
    if (decidingExit) return
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
  // Exit must offer discard even when inputs or the last save are invalid.
  // Wait for an existing write before reloading/discarding, but do not start
  // another write while the user is choosing what to do with the draft.
  async function decideExit<T>(decide: (inputsValid: boolean) => Promise<T>): Promise<T> {
    decidingExit = true
    clearTimeout(queued)
    try {
      const valid = await commit()
      if (saving) await saving
      return await decide(valid)
    } finally {
      decidingExit = false
    }
  }
  return { commit, flush, markChanged, decideExit }
}
