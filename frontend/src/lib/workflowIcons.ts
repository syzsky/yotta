import { addCollection, iconLoaded } from '@iconify/vue'

const pending = new Map<number, Promise<void>>()
export async function ensureWorkflowIcons(names: string[]): Promise<void> {
  const missing = names.filter(
    (name) => /^i-tabler-[a-z0-9-]+$/.test(name) && !iconLoaded(name.slice(2)),
  )
  if (!missing.length) return
  const { lookup, packs } = await import('virtual:tabler-icon-packs')
  const indices = new Set(
    missing
      .map((name) => lookup[name.slice('i-tabler-'.length)])
      .filter((index) => index !== undefined),
  )
  await Promise.all(
    [...indices].map((index) => {
      let loading = pending.get(index)
      if (!loading) {
        loading = packs[index]()
          .then(({ default: icons }) => {
            addCollection(icons)
          })
          .catch((error) => {
            pending.delete(index)
            throw error
          })
        pending.set(index, loading)
      }
      return loading
    }),
  )
}
export async function resolveWorkflowIcon(name?: string): Promise<string> {
  if (!name || !/^i-tabler-[a-z0-9-]+$/.test(name)) return 'i-tabler-route'
  await ensureWorkflowIcons([name])
  return iconLoaded(name.slice(2)) ? name : 'i-tabler-route'
}
