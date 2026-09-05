// Browser-only visual fixture; not imported by the application entrypoint.
import { createApp, h } from 'vue'
import ui from '@nuxt/ui/vue-plugin'
import RegistryAccount from '@/components/RegistryAccount.vue'
import { shopTransport } from '@/app/transport/shop'
import { i18n } from '@/i18n'

export function mountAvatarPreview(picture: string) {
  shopTransport.account = async () => ({
    user_key: 'avatar-fixture',
    name: 'Avatar test',
    picture,
    signingIn: false,
    sessionOnly: false,
  })
  shopTransport.refreshAccount = shopTransport.account
  shopTransport.openAccountCenter = async () => {}
  const host = document.createElement('div')
  host.style.cssText = 'position:fixed;top:0;right:0;height:56px;display:flex;z-index:10'
  document.body.append(host)
  createApp({ render: () => h(RegistryAccount) })
    .use(ui)
    .use(i18n)
    .mount(host)
}
