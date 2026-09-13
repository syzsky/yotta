import { createApp, h, nextTick, type SetupContext } from 'vue'
import { describe, it, expect, vi } from 'vitest'
import { i18n } from '@/i18n'
import RegistryAccountMenu from './RegistryAccountMenu.vue'

describe('account menu presentation', () => {
  it('shows nickname and a plain account center without IDs or manual refresh', async () => {
    const host = document.createElement('div'),
      center = vi.fn(),
      submissions = vi.fn()
    const app = createApp({
      render: () =>
        h(RegistryAccountMenu, {
          profile: {
            user_key: '9x8SqsgN',
            name: 'test',
            picture: '',
            signingIn: false,
            sessionOnly: false,
          },
          busy: false,
          syncing: false,
          failure: '',
          onCenter: center,
          onSubmissions: submissions,
        }),
    })
    app.component('UButton', {
      setup: (_: unknown, ctx: SetupContext) => () => h('button', ctx.slots.default?.()),
    })
    app.component('UIcon', { render: () => h('span') })
    app.use(i18n)
    app.mount(host)
    expect(host.textContent).toContain('test')
    expect(host.textContent).not.toContain('9x8SqsgN')
    expect(host.textContent).not.toContain('刷新')
    expect(host.textContent).not.toContain('头像与资料')
    const entry = host.querySelector<HTMLButtonElement>('[data-testid="account-center"]')
    expect(entry?.textContent).toBe('用户中心')
    entry?.click()
    await nextTick()
    expect(center).toHaveBeenCalledTimes(1)
    const submissionEntry = host.querySelector<HTMLButtonElement>('[data-testid="my-submissions"]')
    expect(submissionEntry?.textContent).toBe('我的投稿')
    submissionEntry?.click()
    await nextTick()
    expect(submissions).toHaveBeenCalledTimes(1)
    app.unmount()
  })
})
