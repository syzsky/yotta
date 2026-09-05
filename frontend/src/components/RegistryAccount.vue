<template>
  <UPopover :content="{ align: 'end', sideOffset: 8 }">
    <button
      data-testid="registry-account"
      type="button"
      class="flex w-14 items-center justify-center transition-colors hover:bg-elevated focus-visible:outline-2 focus-visible:outline-primary"
      :aria-label="
        profile.user_key ? profile.name || profile.user_key : t('workflow.market.sign_in')
      "
    >
      <AccountAvatar :name="profile.name" :user-key="profile.user_key" :picture="profile.picture" />
    </button>
    <template #content>
      <div class="w-72 space-y-3 p-3">
        <div class="flex items-center gap-3 border-b border-default px-1 pb-4 pt-1">
          <AccountAvatar
            :name="profile.name"
            :user-key="profile.user_key"
            :picture="profile.picture"
            large
          />
          <div class="min-w-0">
            <p class="truncate text-sm font-semibold text-highlighted">
              {{ profile.name || profile.user_key || t('workflow.market.account_title') }}
            </p>
            <p class="mt-1 truncate text-xs text-muted">
              {{
                profile.user_key
                  ? t('workflow.market.account_id', { id: profile.user_key })
                  : t('workflow.market.account_hint')
              }}
            </p>
          </div>
        </div>
        <p v-if="profile.signingIn" class="text-sm text-muted" role="status">
          {{ t('workflow.market.waiting_login') }}
        </p>
        <p v-if="failure" class="whitespace-pre-wrap text-sm text-error" role="alert">
          {{ failure }}
        </p>
        <UButton
          v-if="profile.signingIn"
          block
          color="neutral"
          variant="ghost"
          icon="i-tabler-x"
          class="justify-start"
          @click="cancel"
          >{{ t('common.cancel') }}</UButton
        >
        <UButton
          v-else-if="profile.user_key"
          block
          color="neutral"
          variant="ghost"
          icon="i-tabler-logout"
          class="justify-start"
          @click="logout"
          >{{ t('workflow.market.sign_out') }}</UButton
        >
        <UButton
          v-else
          block
          variant="ghost"
          icon="i-tabler-login"
          class="justify-start"
          :loading="busy"
          @click="login"
          >{{ t('workflow.market.sign_in') }}</UButton
        >
      </div>
    </template>
  </UPopover>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { shopTransport } from '@/app/transport/shop'
import { errorMessage } from '@/lib/invoke'
import AccountAvatar from '@/components/AccountAvatar.vue'

const { t } = useI18n()
const profile = ref({ user_key: '', name: '', picture: '', signingIn: false })
const failure = ref('')
const busy = ref(false)
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
async function login() {
  busy.value = true
  failure.value = ''
  try {
    profile.value = await shopTransport.login()
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
async function logout() {
  try {
    await shopTransport.logout()
    await refresh()
  } catch (error) {
    failure.value = errorMessage(error)
  }
}
onMounted(() => {
  void refresh()
  timer = setInterval(() => void refresh(), 1500)
})
onUnmounted(() => clearInterval(timer))
</script>
