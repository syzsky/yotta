import { createApp, h, KeepAlive, nextTick, ref } from 'vue'
import ui from '@nuxt/ui/vue-plugin'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  list: vi.fn(),
  batch: vi.fn(),
  confirm: vi.fn(),
  needsRestart: vi.fn(),
  pick: vi.fn(),
  import: vi.fn(),
  setEnabled: vi.fn(),
  uninstall: vi.fn(),
  rollback: vi.fn(),
  control: vi.fn(),
  load: vi.fn(),
}))
vi.mock('@/lib/plugins', () => ({
  pluginBackend: mocks,
  loadPluginMessages: vi.fn(async () => undefined),
}))
vi.mock('@/stores/settings', () => ({ useSettingsStore: () => ({ load: mocks.load }) }))
vi.mock('@/composables/useConfirm', () => ({
  useConfirm: () => ({ confirm: mocks.confirm }),
}))
vi.mock('vue-i18n', async (original) => ({
  ...(await original<typeof import('vue-i18n')>()),
  useI18n: () => ({ t: (key: string) => key, te: () => true }),
}))
import SettingsPlugins from './SettingsPlugins.vue'
import { RPCError } from '@/lib/invoke'

const item = {
  id: 'example',
  name: 'Example',
  version: '1.0.0',
  description: '',
  enabled: true,
  loaded: true,
  restartRequired: false,
  canRollback: false,
  inUse: false,
  nodes: ['plugin.example.title'],
  workflows: [],
  companions: [{ id: 'capture', name: 'Capture', running: true, status: 'ready' }],
}
let app: ReturnType<typeof createApp> | undefined
beforeEach(() => {
  vi.clearAllMocks()
  mocks.confirm.mockResolvedValue(true)
  mocks.batch.mockReset()
  mocks.list.mockResolvedValue([item])
  mocks.needsRestart.mockResolvedValue(false)
  mocks.load.mockResolvedValue(undefined)
})
afterEach(() => {
  app?.unmount()
  document.body.innerHTML = ''
  vi.useRealTimers()
})
async function mount() {
  const root = document.createElement('div')
  document.body.append(root)
  app = createApp(SettingsPlugins)
  app.use(ui)
  app.mount(root)
  await vi.waitFor(() => expect(root.textContent).toContain('Example'))
  return root
}
const button = (root: HTMLElement, key: string) =>
  Array.from(root.querySelectorAll('button')).find((b) => b.textContent?.trim() === key)!

it('cancelling import preserves the installed plugin', async () => {
  mocks.pick.mockResolvedValue('')
  const root = await mount()
  button(root, 'settingsPlugins.import').click()
  await vi.waitFor(() => expect(mocks.pick).toHaveBeenCalled())
  await nextTick()
  expect(mocks.import).not.toHaveBeenCalled()
  expect(root.textContent).toContain('Example')
})
it('disables lifecycle controls when a Run owns the package, even without current Source references', async () => {
  mocks.list.mockResolvedValue([{ ...item, inUse: true }])
  const root = await mount()
  button(root, 'settingsPlugins.details').click()
  await nextTick()
  expect(root.textContent).toContain('settingsPlugins.in_use')
  expect(button(root, 'settingsPlugins.disable').disabled).toBe(true)
  expect(button(root, 'settingsPlugins.stop').disabled).toBe(true)
  expect(button(root, 'settingsPlugins.uninstall').disabled).toBe(true)
})
it('a rejected state change retains the plugin and displays its structured error', async () => {
  mocks.setEnabled.mockRejectedValue(
    new RPCError(
      { id: 'plugins.in_use', category: 'domain' },
      'SetEnabled',
      'plugin-test-op',
      null,
    ),
  )
  const root = await mount()
  button(root, 'settingsPlugins.disable').click()
  await vi.waitFor(() => expect(root.querySelector('[role="alert"]')).not.toBeNull())
  expect(root.textContent).toContain('plugin-test-op')
  expect(root.textContent).toContain('Example')
  expect(mocks.setEnabled).toHaveBeenCalledWith('example', false)
})

const pair = [
  { ...item, id: 'alpha', name: 'Example Alpha' },
  { ...item, id: 'beta', name: 'Example Beta', enabled: false },
]
function selectAll(root: HTMLElement) {
  ;(root.querySelector('thead [role="checkbox"]') as HTMLElement).click()
}
async function search(root: HTMLElement, value: string) {
  const input = root.querySelector(
    'input[placeholder="settingsPlugins.search"]',
  ) as HTMLInputElement
  input.value = value
  input.dispatchEvent(new Event('input', { bubbles: true }))
  await nextTick()
}
it('select all affects only filtered rows and keeps hidden selections', async () => {
  mocks.list.mockResolvedValue(pair)
  mocks.batch.mockResolvedValue([{ id: 'beta', succeeded: true }])
  const root = await mount()
  selectAll(root)
  await nextTick()
  await search(root, 'Alpha')
  selectAll(root)
  await nextTick()
  expect(root.textContent).toContain('settingsPlugins.hidden_selected')
  ;(root.querySelector('[data-testid="plugin-bulk-enable"]') as HTMLElement).click()
  await vi.waitFor(() => expect(mocks.batch).toHaveBeenCalledWith('enable', ['beta']))
})
it('batch results show individual failures and retry only those items', async () => {
  mocks.list.mockResolvedValue(pair)
  mocks.batch.mockResolvedValue([
    { id: 'alpha', succeeded: true },
    {
      id: 'beta',
      succeeded: false,
      problem: { id: 'plugins.in_use', operationId: 'batch-failure-id' },
    },
  ])
  const root = await mount()
  selectAll(root)
  await nextTick()
  ;(root.querySelector('[data-testid="plugin-bulk-disable"]') as HTMLElement).click()
  await vi.waitFor(() => expect(root.textContent).toContain('batch-failure-id'))
  await vi.waitFor(() => expect(button(root, 'settingsPlugins.retry_failed').disabled).toBe(false))
  button(root, 'settingsPlugins.retry_failed').click()
  await vi.waitFor(() => expect(mocks.batch).toHaveBeenLastCalledWith('disable', ['beta']))
})
it('batch uninstall confirms once and removes only successful selections', async () => {
  mocks.list.mockResolvedValue(pair)
  mocks.batch.mockImplementation(async () => {
    mocks.list.mockResolvedValue([pair[1]])
    return [
      { id: 'alpha', succeeded: true },
      {
        id: 'beta',
        succeeded: false,
        problem: { id: 'plugins.in_use', operationId: 'still-running' },
      },
    ]
  })
  const root = await mount()
  selectAll(root)
  await nextTick()
  ;(root.querySelector('[data-testid="plugin-bulk-uninstall"]') as HTMLElement).click()
  await vi.waitFor(() => expect(mocks.batch).toHaveBeenCalledWith('uninstall', ['alpha', 'beta']))
  await vi.waitFor(() =>
    expect(root.querySelectorAll('[data-testid="plugin-item"]')).toHaveLength(1),
  )
  expect(mocks.confirm).toHaveBeenCalledTimes(1)
  expect(root.querySelector('[data-plugin-id="beta"]')?.getAttribute('aria-selected')).toBe('true')
})

it('pauses polling while its settings tab is cached and resumes once when reactivated', async () => {
  vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
  const visible = ref(true)
  const root = document.createElement('div')
  document.body.append(root)
  app = createApp({
    render: () =>
      h(KeepAlive, null, { default: () => (visible.value ? h(SettingsPlugins) : h('div')) }),
  })
  app.use(ui)
  app.mount(root)
  await vi.waitFor(() => expect(mocks.list).toHaveBeenCalledTimes(1))
  visible.value = false
  await nextTick()
  await vi.advanceTimersByTimeAsync(10000)
  expect(mocks.list).toHaveBeenCalledTimes(1)
  visible.value = true
  await nextTick()
  await vi.waitFor(() => expect(mocks.list).toHaveBeenCalledTimes(2))
})
