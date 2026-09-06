<template>
  <section class="space-y-3" data-testid="plugin-storage">
    <p class="text-xs leading-5 text-muted">{{ t('settingsPlugins.storage_hint') }}</p>
    <UAlert v-if="failure" color="error" variant="soft" :description="failure" role="alert" />
    <p v-if="loading" class="text-xs text-muted">{{ t('common.loading') }}</p>
    <dl v-else class="divide-y divide-default">
      <div v-for="location in locations" :key="location.kind" class="flex items-start gap-3 py-3">
        <UIcon name="i-tabler-folder" class="mt-0.5 size-4 shrink-0 text-muted" />
        <div class="min-w-0 flex-1 space-y-1">
          <dt class="text-xs font-medium text-highlighted">
            {{ t(`settingsPlugins.location_${location.kind}`) }}
          </dt>
          <dd class="text-xs leading-5 text-muted">
            {{ t(`settingsPlugins.location_${location.kind}_hint`) }}
          </dd>
          <dd class="select-all break-all font-mono text-xs leading-5 text-muted">
            {{ location.path }}
          </dd>
        </div>
        <UButton
          size="xs"
          color="neutral"
          variant="soft"
          :loading="opening === location.kind"
          :disabled="!!opening"
          @click="open(location.kind)"
          >{{ t('settingsPlugins.open_folder') }}</UButton
        >
      </div>
    </dl>
  </section>
</template>
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { pluginBackend, type PluginLocation } from '@/lib/plugins'
import { errorMessage } from '@/lib/invoke'
const props = withDefaults(defineProps<{ pluginId?: string }>(), { pluginId: '' })
const { t } = useI18n()
const locations = ref<PluginLocation[]>([])
const loading = ref(true)
const opening = ref('')
const failure = ref('')
onMounted(async () => {
  try {
    locations.value = await pluginBackend.locations(props.pluginId)
  } catch (error) {
    failure.value = errorMessage(error)
  } finally {
    loading.value = false
  }
})
async function open(kind: string) {
  opening.value = kind
  failure.value = ''
  try {
    await pluginBackend.openLocation(kind, props.pluginId)
  } catch (error) {
    failure.value = errorMessage(error)
  } finally {
    opening.value = ''
  }
}
</script>
