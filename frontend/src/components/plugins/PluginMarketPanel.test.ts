import { createApp, nextTick } from 'vue'
import ui from '@nuxt/ui/vue-plugin'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  discoverRegistry: vi.fn(),
  registryTaxonomy: vi.fn(),
  list: vi.fn(),
  installRegistry: vi.fn(),
  push: vi.fn(),
}))

vi.mock('@/lib/plugins', () => ({
  pluginBackend: {
    discoverRegistry: mocks.discoverRegistry,
    registryTaxonomy: mocks.registryTaxonomy,
    list: mocks.list,
    installRegistry: mocks.installRegistry,
  },
}))
vi.mock('vue-router', async (original) => ({
  ...(await original<typeof import('vue-router')>()),
  useRouter: () => ({ push: mocks.push }),
  useRoute: () => ({ path: '/market', fullPath: '/market', query: {}, params: {}, meta: {} }),
}))
vi.mock('vue-i18n', async (original) => ({
  ...(await original<typeof import('vue-i18n')>()),
  useI18n: () => ({ t: (key: string) => key, locale: { value: 'zh' } }),
}))
vi.mock('@/components/AccountAvatar.vue', () => ({
  default: { template: '<span data-testid="avatar" />' },
}))
vi.mock('@/components/workflow/WorkflowMarketDocument.vue', () => ({
  default: { props: ['content'], template: '<div>{{ content }}</div>' },
}))

import PluginMarketPanel from './PluginMarketPanel.vue'
import { RPCError } from '@/lib/invoke'
import { i18n } from '@/i18n'

const release = {
  releaseId: 'release-1',
  publisherNamespace: 'creator',
  packageId: 'plugin.example',
  packageVersion: '1.1.0',
  contributionDigest: 'sha256:contribution',
  title: 'Example Plugin',
  summary: 'Adds useful nodes',
  releaseNotes: 'Improved node support',
  listing: {
    filterValues: [],
    icon: 'i-tabler-plug',
    category: 'automation',
    tags: ['tools'],
    description: 'Long description',
    instructions: 'Install and use',
  },
  nodes: [
    {
      nodeRef: { nodeTypeId: 'example.node', nodeVersion: '1.0.0', semanticDigest: 'sha256:node' },
      packageId: 'plugin.example',
      packageVersion: '1.1.0',
      contributionDigest: 'sha256:contribution',
      name: 'Example Node',
      summary: 'Does something useful',
      category: 'automation',
      inputs: [{ id: 'input', name: 'Input', description: '' }],
      outputs: [{ id: 'output', name: 'Output', description: '' }],
    },
  ],
  variants: [
    {
      variantId: 'windows-amd64',
      runtimeFamily: 'process',
      runtimeProfile: '',
      operatingSystems: ['windows'],
      architectures: ['amd64'],
      manifestDigest: 'sha256:manifest',
      artifactDigest: 'sha256:artifact',
      downloadBytes: 4096,
    },
  ],
  creator: { qualityAuthor: true, picture: '', userKey: 'creator-1', displayName: 'Creator' },
  availability: 'public',
  publishedAt: '2026-09-12T00:00:00Z',
}

const installed = {
  id: 'plugin.example',
  name: 'Example Plugin',
  description: '',
  version: '1.1.0',
  inUse: false,
  enabled: true,
  loaded: false,
  restartRequired: true,
  canRollback: false,
  nodes: ['example.node'],
  workflows: [],
  companions: [],
}

let app: ReturnType<typeof createApp> | undefined
beforeEach(() => {
  vi.clearAllMocks()
  mocks.registryTaxonomy.mockResolvedValue({
    kind: 'node-pack',
    revision: 2,
    categories: [
      { key: 'automation', name: 'Plugin Automation', parentKey: '', active: true, position: 0 },
    ],
    dimensions: [],
    systemFacets: [
      {
        id: 'node-pack.operating-system',
        name: 'Operating system',
        source: 'system',
        description: '',
      },
    ],
  })
  mocks.discoverRegistry.mockResolvedValue({
    items: [release],
    facets: {
      categories: ['automation'],
      tags: ['tools'],
      systemFacets: [
        {
          id: 'node-pack.operating-system',
          source: 'system',
          values: [{ value: 'windows', count: 1 }],
        },
      ],
    },
    nextCursor: '',
  })
  mocks.list.mockResolvedValue([])
  mocks.installRegistry.mockResolvedValue({
    plugin: installed,
    alreadyInstalled: false,
    planId: 'plan-1',
  })
})
afterEach(() => {
  app?.unmount()
  document.body.innerHTML = ''
})

async function mount() {
  const root = document.createElement('div')
  document.body.append(root)
  app = createApp(PluginMarketPanel)
  app.use(ui)
  app.mount(root)
  await vi.waitFor(() => expect(root.textContent).toContain('Example Plugin'))
  return root
}

it.each([
  ['plugins.market_cancelled', '插件市场操作已取消', false],
  ['plugins.market_timeout', '插件市场请求超时', true],
] as const)('shows %s without losing the search input', async (id, message, retryable) => {
  const root = await mount()
  i18n.global.locale.value = 'zh'
  const query = root.querySelector<HTMLInputElement>('input')!
  query.value = 'preserved query'
  query.dispatchEvent(new Event('input'))
  await nextTick()
  mocks.registryTaxonomy.mockRejectedValueOnce(
    new RPCError(
      { id, category: 'domain', retryable },
      'RegistryTaxonomy',
      'taxonomy-test-operation',
      null,
    ),
  )
  root
    .querySelector('form')!
    .dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
  await vi.waitFor(() => expect(root.textContent).toContain(message))
  expect(root.textContent).toContain('taxonomy-test-operation')
  expect(query.value).toBe('preserved query')
})

it('discovers plugins and installs the selected release through the registry service', async () => {
  mocks.list.mockResolvedValueOnce([]).mockResolvedValue([installed])
  const root = await mount()
  const installButton = root.querySelector(
    '[data-testid="plugin-market-install"]',
  ) as HTMLButtonElement
  expect(installButton.textContent).toContain('market.plugins.install')
  installButton.click()
  await vi.waitFor(() => expect(mocks.installRegistry).toHaveBeenCalledWith('release-1'))
  await vi.waitFor(() => expect(root.textContent).toContain('market.plugins.restart_required'))
  expect(mocks.list).toHaveBeenCalledTimes(2)
})

it('shows update state when the registry release is newer than the installed plugin', async () => {
  mocks.list.mockResolvedValue([{ ...installed, version: '1.0.0', restartRequired: false }])
  const root = await mount()
  await nextTick()
  expect(root.querySelector('[data-testid="plugin-market-install"]')?.textContent).toContain(
    'market.plugins.update',
  )
})

it('keeps a selected tag removable when the current query has no matching tags', async () => {
  const root = await mount()
  ;(root.querySelector('[data-testid="plugin-market-filter-toggle"]') as HTMLButtonElement).click()
  await vi.waitFor(() => expect(document.body.querySelector('select')).not.toBeNull())
  const select = document.body.querySelector('select') as HTMLSelectElement
  mocks.discoverRegistry.mockResolvedValueOnce({
    items: [],
    facets: { categories: [], tags: [], systemFacets: [] },
    nextCursor: '',
  })
  select.value = 'tools'
  select.dispatchEvent(new Event('change', { bubbles: true }))
  await vi.waitFor(() =>
    expect(mocks.discoverRegistry).toHaveBeenLastCalledWith(
      expect.objectContaining({ tag: 'tools' }),
    ),
  )
  await vi.waitFor(() => expect(root.textContent).toContain('market.plugins.empty_title'))
  const remaining = document.body.querySelector('select') as HTMLSelectElement
  expect(remaining.value).toBe('tools')
  remaining.value = ''
  remaining.dispatchEvent(new Event('change', { bubbles: true }))
  await vi.waitFor(() =>
    expect(mocks.discoverRegistry).toHaveBeenLastCalledWith(expect.objectContaining({ tag: '' })),
  )
})

it('shows business candidate counts only with matching profile metadata', async () => {
  mocks.registryTaxonomy.mockResolvedValue({
    kind: 'node-pack',
    revision: 7,
    categories: [],
    systemFacets: [],
    dimensions: [
      {
        id: 'purpose',
        name: 'Purpose',
        active: true,
        position: 0,
        values: [{ id: 'tools', name: 'Tools', active: true, position: 0, parentId: '' }],
      },
    ],
  })
  mocks.discoverRegistry.mockResolvedValue({
    items: [release],
    facets: {
      categories: [],
      tags: [],
      systemFacets: [],
      profileRevision: 7,
      dimensions: [{ id: 'purpose', values: [{ value: 'tools', count: 2 }] }],
    },
    nextCursor: '',
  })
  const root = await mount()
  ;(root.querySelector('[data-testid="plugin-market-filter-toggle"]') as HTMLButtonElement).click()
  await vi.waitFor(() => expect(document.body.textContent).toContain('Tools (2)'))
})

it('uses plugin taxonomy and keeps a selected system facet removable after an empty result', async () => {
  const root = await mount()
  expect(root.textContent).toContain('Plugin Automation')
  ;(root.querySelector('[data-testid="plugin-market-filter-toggle"]') as HTMLButtonElement).click()
  await vi.waitFor(() => expect(document.body.textContent).toContain('windows (1)'))
  const checkbox = () =>
    [...document.querySelectorAll('[role="checkbox"]')].find((element) => {
      const label = document.querySelector(`label[for="${element.id}"]`)
      return label?.textContent?.includes('windows')
    }) as HTMLButtonElement
  mocks.discoverRegistry.mockResolvedValueOnce({
    items: [],
    facets: { categories: [], tags: [], systemFacets: [] },
    nextCursor: '',
  })
  checkbox().click()
  await vi.waitFor(() =>
    expect(mocks.discoverRegistry).toHaveBeenLastCalledWith(
      expect.objectContaining({ systemFacets: ['node-pack.operating-system:windows'] }),
    ),
  )
  await vi.waitFor(() => expect(document.body.textContent).toContain('windows (0)'))
  checkbox().click()
  await vi.waitFor(() =>
    expect(mocks.discoverRegistry).toHaveBeenLastCalledWith(
      expect.objectContaining({ systemFacets: [] }),
    ),
  )
})
