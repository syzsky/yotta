import { createApp, h, nextTick, ref } from 'vue'
import { createMemoryHistory, createRouter } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import { afterEach, expect, it } from 'vitest'
import { i18n } from '@/i18n'
import PluginReferences from './PluginReferences.vue'

let app: ReturnType<typeof createApp> | undefined
afterEach(() => {
  app?.unmount()
  document.body.innerHTML = ''
})
it('bounds hundreds of workflow references and supports search, running filter and page clamping', async () => {
  const workflows = ref(
    Array.from({ length: 500 }, (_, i) => ({
      id: `w${i}`,
      name: `Workflow ${i}`,
      running: i % 37 === 0,
    })),
  )
  const root = document.createElement('div')
  document.body.append(root)
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/:pathMatch(.*)*', component: { render: () => null } }],
  })
  app = createApp({ render: () => h(PluginReferences, { workflows: workflows.value }) })
  app.use(router).use(ui).use(i18n)
  app.mount(root)
  await nextTick()
  const rows = () => [...root.querySelectorAll('[data-testid="plugin-reference-row"]')]
  const next = () =>
    [...root.querySelectorAll('button')].find(
      (button) => button.textContent?.trim() === i18n.global.t('settingsPlugins.next'),
    )!
  expect(rows()).toHaveLength(10)
  expect(rows()[0]?.textContent).toContain('Workflow 0')
  next().click()
  await nextTick()
  expect(rows()[0]?.textContent).toContain('Workflow 10')
  const input = root.querySelector('input[placeholder]') as HTMLInputElement
  input.value = 'Workflow 49'
  input.dispatchEvent(new Event('input', { bubbles: true }))
  await nextTick()
  expect(rows()).toHaveLength(10)
  expect(rows()[0]?.textContent).toContain('Workflow 49')
  next().click()
  await nextTick()
  expect(rows()).toHaveLength(1)
  input.value = ''
  input.dispatchEvent(new Event('input', { bubbles: true }))
  await nextTick()
  ;(root.querySelector('[role="checkbox"]') as HTMLElement).click()
  await nextTick()
  expect(rows()).toHaveLength(10)
  expect(
    rows().every((row) => row.textContent?.includes(i18n.global.t('settingsPlugins.active_run'))),
  ).toBe(true)
  next().click()
  await nextTick()
  expect(rows()).toHaveLength(4)
  workflows.value = [{ id: 'remaining', name: 'Remaining', running: true }]
  await nextTick()
  expect(rows()).toHaveLength(1)
  expect(rows()[0]?.getAttribute('href')).toBe('/workflows/remaining/edit')
})
