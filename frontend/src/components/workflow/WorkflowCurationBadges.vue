<script setup lang="ts">
import { useI18n } from 'vue-i18n'
defineProps<{ official?: boolean; recommended?: boolean }>()
const { t } = useI18n()
</script>
<template>
  <span v-if="official || recommended" class="inline-flex shrink-0 flex-wrap items-center gap-1">
    <UBadge v-if="official" icon="i-tabler-rosette-discount-check" size="xs" variant="subtle">{{
      t('workflow.community.official')
    }}</UBadge>
    <UBadge
      v-if="recommended"
      icon="i-tabler-star-filled"
      color="warning"
      size="xs"
      variant="subtle"
      >{{ t('workflow.community.recommended') }}</UBadge
    >
  </span>
</template>
