import { createApp, nextTick } from 'vue'
import ui from '@nuxt/ui/vue-plugin'
import { afterEach, expect, it, vi } from 'vitest'

vi.mock('vue-i18n', async (original) => ({
  ...(await original<typeof import('vue-i18n')>()),
  useI18n: () => ({ t: (key: string) => key }),
}))
vi.mock('@/components/workflow/WorkflowMarketPanel.vue', () => ({
  default: { template: '<div data-testid="workflow-panel" />' },
}))
vi.mock('@/components/plugins/PluginMarketPanel.vue', () => ({
  default: {
    data: () => ({ selected: '' }),
    template:
      '<div data-testid="plugin-panel"><input v-model="selected" data-testid="plugin-filter" /></div>',
  },
}))

import MarketView from './MarketView.vue'

let app: ReturnType<typeof createApp> | undefined
afterEach(() => {
  app?.unmount()
  document.body.innerHTML = ''
})

it('switches the independent market between workflows and plugins', async () => {
  const root = document.createElement('div')
  document.body.append(root)
  app = createApp(MarketView)
  app.use(ui)
  app.mount(root)
  expect(root.querySelector('[data-testid="workflow-panel"]')).not.toBeNull()
  ;(root.querySelector('[data-testid="market-tab-plugins"]') as HTMLElement).click()
  await nextTick()
  expect(root.querySelector('[data-testid="plugin-panel"]')).not.toBeNull()
  expect(root.querySelector('[data-testid="workflow-panel"]')).toBeNull()
  const filter = root.querySelector<HTMLInputElement>('[data-testid="plugin-filter"]')!
  filter.value = 'windows'
  filter.dispatchEvent(new Event('input'))
  await nextTick()
  ;(root.querySelector('[data-testid="market-tab-workflows"]') as HTMLElement).click()
  await nextTick()
  ;(root.querySelector('[data-testid="market-tab-plugins"]') as HTMLElement).click()
  await nextTick()
  expect(root.querySelector<HTMLInputElement>('[data-testid="plugin-filter"]')?.value).toBe(
    'windows',
  )
})
