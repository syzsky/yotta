<template>
  <div class="space-y-3">
    <UFormField :label="label" required :description="t('paths.source_variable_hint')">
      <USelectMenu
        :model-value="value"
        :aria-label="label"
        :items="items"
        value-key="value"
        label-key="label"
        :placeholder="t('paths.source_choose_variable')"
        class="w-full"
        @update:model-value="emit('change', $event)"
      />
      <p v-if="!items.length" class="mt-2 text-xs text-warning">
        {{ t('paths.source_no_variables') }}
      </p>
    </UFormField>
    <details :open="!value">
      <summary class="cursor-pointer text-xs font-medium text-primary">
        {{ t('paths.source_setup') }}
      </summary>
      <div class="mt-3 space-y-2">
        <PositionSourcePicker v-model="endpoint" slots-only @select="slot = $event" />
        <p class="text-[11px] leading-5 text-muted">{{ t('paths.source_setup_hint') }}</p>
        <UButton
          :label="t('paths.source_setup')"
          icon="i-tabler-plug-connected"
          color="primary"
          variant="soft"
          size="xs"
          :disabled="!slot || !context"
          :loading="busy"
          @click="connect"
        />
        <p v-if="problem" role="alert" class="text-xs text-error">{{ problem }}</p>
      </div>
    </details>
  </div>
</template>
<script setup lang="ts">
import { computed, inject, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { backend } from '@/lib/backend'
import { errorMessage } from '@/lib/invoke'
import PositionSourcePicker from '@/app/paths/PositionSourcePicker.vue'
import { POSITION_SOURCE_AUTHORING } from './positionSourceAuthoring'
const props = defineProps<{ nodeId: string; value: string; label: string }>()
const emit = defineEmits<{ change: [value: string] }>()
const { t } = useI18n()
const context = inject(POSITION_SOURCE_AUTHORING, null)
const items = computed(() => context?.variables() ?? [])
const endpoint = ref(''),
  slot = ref(''),
  problem = ref(''),
  busy = ref(false)
async function connect() {
  if (!context || !slot.value || busy.value) return
  busy.value = true
  problem.value = ''
  try {
    await backend.paths.sample(endpoint.value)
    context.connect(props.nodeId, slot.value)
  } catch (error) {
    problem.value = errorMessage(error)
  } finally {
    busy.value = false
  }
}
</script>
