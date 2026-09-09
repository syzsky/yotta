<template>
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
          {{
            profile.name ||
            t(profile.user_key ? 'workflow.community.member' : 'workflow.market.account_title')
          }}
        </p>
        <p v-if="!profile.user_key" class="mt-1 truncate text-xs text-muted">
          {{ t('workflow.market.account_hint') }}
        </p>
      </div>
    </div>
    <p v-if="profile.signingIn" class="text-sm text-muted" role="status">
      {{ t('workflow.market.waiting_login') }}
    </p>
    <p v-if="failure" class="whitespace-pre-wrap text-sm text-error" role="alert">
      {{ failure }}
    </p>
    <p
      v-if="profile.user_key && profile.sessionOnly"
      role="status"
      class="text-xs leading-5 text-warning"
    >
      {{ t('workflow.market.session_only') }}
    </p>
    <UButton
      v-if="profile.user_key"
      block
      color="neutral"
      variant="ghost"
      icon="i-tabler-user-circle"
      class="justify-start"
      data-testid="account-center"
      @click="emit('center')"
      >{{ t('workflow.market.account_center') }}</UButton
    >
    <UButton
      v-if="profile.signingIn"
      block
      color="neutral"
      variant="ghost"
      icon="i-tabler-x"
      class="justify-start"
      @click="emit('cancel')"
      >{{ t('common.cancel') }}</UButton
    >
    <UButton
      v-else-if="profile.user_key"
      :disabled="busy || syncing"
      block
      color="neutral"
      variant="ghost"
      icon="i-tabler-logout"
      class="justify-start"
      @click="emit('logout')"
      >{{ t('workflow.market.sign_out') }}</UButton
    >
    <div v-else class="grid grid-cols-2 gap-2">
      <UButton :disabled="syncing || busy" block icon="i-tabler-login" @click="emit('login')">
        {{ t('workflow.market.sign_in') }}
      </UButton>
      <UButton
        :disabled="syncing || busy"
        block
        color="neutral"
        variant="outline"
        icon="i-tabler-user-plus"
        @click="emit('register')"
      >
        {{ t('workflow.market.sign_up') }}
      </UButton>
    </div>
  </div>
</template>
<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { Profile } from '@bindings/github.com/yottaapp/yotta/internal/nativeoidc/models.js'
import AccountAvatar from './AccountAvatar.vue'
defineProps<{ profile: Profile; busy: boolean; syncing: boolean; failure: string }>()
const emit = defineEmits<{ login: []; register: []; logout: []; cancel: []; center: [] }>()
const { t } = useI18n()
</script>
