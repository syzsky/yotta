declare module 'virtual:tabler-icon-names' {
  const names: readonly string[]
  export default names
}

declare module 'virtual:tabler-icon-packs' {
  export const lookup: Record<string, number>
  export const packs: Array<() => Promise<{ default: import('@iconify/vue').IconifyJSON }>>
}
