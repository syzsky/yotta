<template>
  <div class="space-y-2">
    <UFormField :label="t('paths.source_select')">
      <USelectMenu
        :model-value="selectedSlot"
        :items="items"
        value-key="value"
        label-key="label"
        :placeholder="t('paths.source_select')"
        :aria-label="t('paths.source_select')"
        class="w-full"
        :disabled="disabled"
        @update:model-value="selectSource"
      />
    </UFormField>
    <p class="text-[11px] leading-4 text-muted">{{ t('paths.source_picker_hint') }}</p>
    <details v-if="!slotsOnly">
      <summary class="cursor-pointer text-xs text-muted">{{ t('paths.source_manual') }}</summary>
      <UInput
        v-model="endpoint"
        class="mt-2 w-full"
        :disabled="disabled"
        placeholder="http://127.0.0.1:…/v1/position-source/sample"
        :aria-label="t('paths.source')"
      />
    </details>
    <p v-if="problem" role="alert" class="text-xs text-error">{{ problem }}</p>
    <UButton
      v-if="!items.length"
      :to="standalone ? undefined : '/settings?section=plugins'"
      @click="standalone && emit('configure')"
      color="primary"
      variant="link"
      size="xs"
      :label="t('workflow.inspector.configure_target')"
    />
  </div>
</template>
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { errorMessage } from '@/lib/invoke'
import { useSettingsStore } from '@/stores/settings'
defineProps<{ disabled?: boolean; slotsOnly?: boolean; standalone?: boolean }>()
const endpoint = defineModel<string>({ required: true })
const emit = defineEmits<{ select: [slot: string]; configure: [] }>()
const { t } = useI18n()
const problem = ref('')
const settings = useSettingsStore()
const profiles = computed(() => settings.data?.network.httpOrigins ?? [])
const items = computed(() =>
  profiles.value.map((profile) => ({ value: profile.slot, label: profile.label || profile.slot })),
)
const address = (origin: string) => origin.replace(/\/$/, '') + '/v1/position-source/sample'
const selectedSlot = computed(
  () => profiles.value.find((profile) => address(profile.origin) === endpoint.value)?.slot,
)
function selectSource(slot: string) {
  const profile = profiles.value.find((profile) => profile.slot === slot)
  if (profile) {
    endpoint.value = address(profile.origin)
    emit('select', slot)
  }
}
onMounted(() => {
  if (!settings.loaded)
    void settings.load().catch((error) => {
      problem.value = errorMessage(error)
    })
})
</script>
