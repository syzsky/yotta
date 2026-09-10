<template>
  <UPopover v-model:open="menuOpen" :content="{ align: 'end', sideOffset: 8 }">
    <button
      data-testid="registry-account"
      @click="syncProfile"
      type="button"
      class="flex w-14 items-center justify-center transition-colors hover:bg-elevated focus-visible:outline-2 focus-visible:outline-primary"
      :aria-label="
        profile.user_key
          ? profile.name || t('workflow.community.member')
          : t('workflow.market.sign_in')
      "
    >
      <AccountAvatar :name="profile.name" :user-key="profile.user_key" :picture="profile.picture" />
    </button>
    <template #content>
      <RegistryAccountMenu
        v-if="menuOpen"
        :profile="profile"
        :busy="busy"
        :syncing="syncing"
        :failure="failure"
        @login="accountAction(shopTransport.login)"
        @register="accountAction(shopTransport.register)"
        @logout="accountAction(shopTransport.logout)"
        @cancel="cancel"
        @center="openAccountCenter"
        @wallet="openWallet"
      />
    </template>
  </UPopover>
</template>

<script setup lang="ts">
import { defineAsyncComponent, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { shopTransport } from '@/app/transport/shop'
import { errorMessage } from '@/lib/invoke'
import AccountAvatar from '@/components/AccountAvatar.vue'

const RegistryAccountMenu = defineAsyncComponent(() => import('./RegistryAccountMenu.vue'))
const menuOpen = ref(false)
const { t } = useI18n()
const profile = ref({ user_key: '', name: '', picture: '', signingIn: false, sessionOnly: false })
const failure = ref('')
const busy = ref(false)
const syncing = ref(false)
let disposed = false,
  lastSync = 0
let timer: ReturnType<typeof setInterval> | undefined
let refreshing = false
async function refresh() {
  if (refreshing) return
  refreshing = true
  try {
    profile.value = await shopTransport.account()
  } catch {
    /* Login owns visible errors. */
  } finally {
    refreshing = false
  }
}
async function accountAction(action: () => Promise<unknown>) {
  if (busy.value || syncing.value) return
  busy.value = true
  failure.value = ''
  try {
    await action()
  } catch (error) {
    failure.value = errorMessage(error)
  } finally {
    busy.value = false
    await refresh()
  }
}
async function cancel() {
  try {
    await shopTransport.cancelLogin()
    await refresh()
  } catch (error) {
    failure.value = errorMessage(error)
  }
}
async function syncProfile() {
  if (syncing.value || busy.value || profile.value.signingIn) return
  syncing.value = true
  lastSync = Date.now()
  try {
    const current = await shopTransport.refreshAccount()
    if (!disposed) {
      profile.value = current
      failure.value = ''
    }
  } catch (error) {
    if (!disposed) failure.value = errorMessage(error)
  } finally {
    syncing.value = false
  }
}
async function openAccountCenter() {
  try {
    await shopTransport.openAccountCenter()
  } catch (error) {
    failure.value = errorMessage(error)
  }
}
async function openWallet() {
  try {
    await shopTransport.openWallet()
  } catch (error) {
    failure.value = errorMessage(error)
  }
}
function onFocus() {
  if (Date.now() - lastSync > 1000) void syncProfile()
}
onMounted(() => {
  void syncProfile()
  timer = setInterval(() => void refresh(), 1500)
  window.addEventListener('focus', onFocus)
})
onUnmounted(() => {
  disposed = true
  clearInterval(timer)
  window.removeEventListener('focus', onFocus)
})
</script>
