<template>
  <div role="status" class="space-y-2 text-sm" data-testid="publication-status">
    <p class="font-medium text-highlighted">{{ t(`workflow.market.${statusKey}`) }}</p>
    <p v-if="status === 'pending_review'" class="text-muted">
      {{ t('workflow.market.pending_review_hint') }}
    </p>
    <p v-if="reason" class="whitespace-pre-wrap break-words text-muted">{{ reason }}</p>
  </div>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
const props = defineProps<{ status: string; reason?: string }>()
const { t } = useI18n()
const statusKey = computed(
  () =>
    ({
      pending_review: 'submission_pending',
      approved: 'submission_approved',
      rejected: 'submission_rejected',
      published: 'submission_published',
    })[props.status] || 'submission_received',
)
</script>
