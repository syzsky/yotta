import { createApp, nextTick } from 'vue'
import ui from '@nuxt/ui/vue-plugin'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  discoverRegistry: vi.fn(),
  list: vi.fn(),
  installRegistry: vi.fn(),
  push: vi.fn(),
}))

vi.mock('@/lib/plugins', () => ({
  pluginBackend: {
    discoverRegistry: mocks.discoverRegistry,
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
  mocks.discoverRegistry.mockResolvedValue({
    items: [release],
    facets: { categories: ['automation'], tags: ['tools'] },
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
