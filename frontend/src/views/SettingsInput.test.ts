import { createApp, h, nextTick } from 'vue'
import ui from '@nuxt/ui/vue-plugin'
import { createMemoryHistory, createRouter } from 'vue-router'
import { expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({ patch: vi.fn(async () => undefined) }))
vi.mock('@/stores/settings', () => ({
  useSettingsStore: () => ({
    data: { ui: { activeMouseProfile: 'Game' } },
    mouseProfiles: [{ label: 'Game', counts360: 1000 }],
    patch: mocks.patch,
  }),
}))
vi.mock('@/stores/hotkeys', () => ({
  useHotkeysStore: () => ({ keyFor: () => 'F8' }),
}))
vi.mock('@/composables/useConfirm', () => ({ useConfirm: () => ({ confirm: vi.fn() }) }))
vi.mock('vue-i18n', async (original) => ({
  ...(await original<typeof import('vue-i18n')>()),
  useI18n: () => ({ t: (key: string) => key }),
}))
import SettingsInput from './SettingsInput.vue'

it('saves every count entered in a mouse calibration profile', async () => {
  const host = document.createElement('div')
  document.body.append(host)
  const app = createApp({ render: () => h(SettingsInput) })
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/', component: { render: () => null } }],
  })
  app.use(router)
  app.use(ui)
  try {
    await router.isReady()
    app.mount(host)
    await nextTick()
    const input = host.querySelector<HTMLInputElement>('input[role="spinbutton"]')!
    expect(input).not.toBeNull()
    for (const counts of [1111, 7, 999999, 0]) {
      input.value = String(counts)
      input.dispatchEvent(new Event('input', { bubbles: true }))
      input.dispatchEvent(new FocusEvent('blur'))
      await nextTick()
      expect(mocks.patch).toHaveBeenLastCalledWith({
        ui: { mouseProfiles: [{ label: 'Game', counts360: counts }], activeMouseProfile: 'Game' },
      })
      expect(Number(input.value.replaceAll(',', ''))).toBe(counts)
    }
  } finally {
    app.unmount()
    host.remove()
  }
})
