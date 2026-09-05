import { createApp, h, nextTick, reactive } from 'vue'
import { describe, expect, it } from 'vitest'
import AccountAvatar from './AccountAvatar.vue'

describe('account avatar', () => {
  it('renders the Identity picture and falls back if it cannot load', async () => {
    const props = reactive({
      name: 'test',
      userKey: 'user-1',
      picture: 'https://account.example/media/avatar',
    })
    const host = document.createElement('div')
    const app = createApp({ render: () => h(AccountAvatar, props) })
    app.component('UIcon', { render: () => h('span') })
    app.mount(host)
    expect(host.querySelector('img')?.getAttribute('src')).toBe(props.picture)
    host.querySelector('img')?.dispatchEvent(new Event('error'))
    await nextTick()
    expect(host.querySelector('img')).toBeNull()
    expect(host.textContent).toBe('TE')
    props.picture = 'https://account.example/media/new'
    await nextTick()
    expect(host.querySelector('img')?.getAttribute('src')).toContain('/new')
    app.unmount()
  })
})
