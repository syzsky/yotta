<template>
  <span class="account-avatar" :class="{ 'account-avatar-large': large, 'is-signed-in': userKey }">
    <img
      v-if="picture && !failed"
      :src="picture"
      :alt="name || ''"
      referrerpolicy="no-referrer"
      @error="failed = true"
    />
    <span v-else-if="initials" class="font-semibold">{{ initials }}</span>
    <UIcon v-else name="i-tabler-user" :class="large ? 'size-6' : 'size-5'" />
  </span>
</template>
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
const props = defineProps<{ name?: string; userKey?: string; picture?: string; large?: boolean }>()
const failed = ref(false)
const initials = computed(() =>
  Array.from((props.name || '').trim())
    .slice(0, 2)
    .join('')
    .toUpperCase(),
)
watch(
  () => props.picture,
  () => {
    failed.value = false
  },
)
</script>
<style scoped>
.account-avatar {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  flex-shrink: 0;
  overflow: hidden;
  border-radius: 50%;
  border: 1px solid var(--ui-border-accented);
  background: var(--ui-bg-elevated);
  color: var(--ui-text-muted);
  font-size: 12px;
}
.account-avatar.is-signed-in {
  color: var(--ui-text-highlighted);
  background: color-mix(in oklab, var(--ui-primary) 14%, var(--ui-bg-elevated));
}
.account-avatar-large {
  width: 48px;
  height: 48px;
  font-size: 16px;
}
.account-avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
</style>
