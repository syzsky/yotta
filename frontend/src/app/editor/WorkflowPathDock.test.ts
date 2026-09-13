import { createApp, nextTick } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { createI18n } from 'vue-i18n'
import { createMemoryHistory, createRouter } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import { afterEach, expect, it, vi } from 'vitest'
import en from '@/i18n/en'
import WorkflowPathDock from './WorkflowPathDock.vue'
import { useAssetsStore } from '@/stores/assets'

const mock = vi.hoisted(() => ({
  items: [] as Array<Record<string, unknown> & { guid: string; name: string }>,
  query: vi.fn(),
  open: vi.fn(),
  use: vi.fn(),
  batchDelete: vi.fn(),
}))
vi.mock('@/lib/backend', () => ({
  backend: {
    assets: { query: mock.query, batchDelete: mock.batchDelete },
    tools: { openPathEditor: mock.open },
  },
}))
vi.mock('@/composables/useConfirm', () => ({ useConfirm: () => ({ confirm: async () => true }) }))
vi.mock('@wailsio/runtime', () => ({ Events: { On: vi.fn(() => () => {}) } }))
let app: ReturnType<typeof createApp> | undefined
afterEach(() => {
  app?.unmount()
  document.body.innerHTML = ''
})
async function flush() {
  for (let i = 0; i < 8; i++) {
    await Promise.resolve()
    await nextTick()
  }
}
async function mountDock() {
  mock.open.mockReset()
  mock.use.mockReset()
  mock.items = [
    {
      guid: 'path-one',
      name: 'Route to delete',
      kind: 'path',
      category: '',
      tags: [],
      blob: { digest: 'sha256:test', size: 12, mediaType: 'application/vnd.yotta.path+json' },
    },
  ]
  mock.query.mockImplementation(async () => ({
    items: [...mock.items],
    total: mock.items.length,
    revision: 1,
    categories: [],
    tags: [],
  }))
  const pinia = createPinia()
  setActivePinia(pinia)
  const root = document.createElement('div')
  document.body.append(root)
  app = createApp(WorkflowPathDock, { source: { resources: [], graphs: [] }, onUse: mock.use })
  app
    .use(pinia)
    .use(ui)
    .use(createI18n({ legacy: false, locale: 'en', messages: { en } }))
  app.use(
    createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/', component: { template: '<div />' } }],
    }),
  )
  app.mount(root)
  await flush()
  return root
}

it('removes a deleted path from the mounted dock after asset invalidation', async () => {
  const root = await mountDock()
  expect(root.textContent).toContain('Route to delete')
  mock.items = []
  useAssetsStore().invalidate()
  await flush()
  expect(root.textContent).not.toContain('Route to delete')
})

it('opens the native editor without navigating away and keeps batch failures selected', async () => {
  const root = await mountDock()
  root.querySelector<HTMLButtonElement>('[data-testid="workflow-resource-create"]')!.click()
  await flush()
  expect(mock.open).toHaveBeenCalledWith('')
  root.querySelector<HTMLButtonElement>('[aria-label="Select every asset on this page"]')!.click()
  await flush()
  mock.batchDelete.mockResolvedValue([{ guid: 'path-one', deleted: false }])
  root.querySelector<HTMLButtonElement>('[aria-label="Batch delete"]')!.click()
  await flush()
  expect(mock.batchDelete).toHaveBeenCalledWith(['path-one'])
  expect(root.textContent).toContain('Deleted 0; failed 1.')
  expect(root.querySelector('[aria-label="Batch delete"]')).not.toBeNull()
  mock.batchDelete.mockImplementation(async () => {
    mock.items = []
    return [{ guid: 'path-one', deleted: true }]
  })
  root.querySelector<HTMLButtonElement>('[aria-label="Batch delete"]')!.click()
  await flush()
  expect(root.textContent).not.toContain('Route to delete')
  expect(root.textContent).toContain('Deleted 1; failed 0.')
  expect(root.querySelector('[aria-label="Batch delete"]')).toBeNull()
})

it('keeps single-click path binding while selection does not activate a node', async () => {
  const root = await mountDock()
  root.querySelector<HTMLButtonElement>('[aria-label="Select every asset on this page"]')!.click()
  await flush()
  expect(mock.use).not.toHaveBeenCalled()
  root.querySelector<HTMLElement>('[data-asset-id="path-one"]')!.click()
  await flush()
  expect(mock.use).toHaveBeenCalledWith(
    expect.objectContaining({ guid: 'path-one', kind: 'path' }),
    undefined,
    undefined,
  )
})
