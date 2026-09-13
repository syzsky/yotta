<template>
  <div class="workspace-page workspace-canvas flex h-full min-h-0 w-full flex-col overflow-hidden">
    <header
      class="workspace-page__header flex min-h-[72px] shrink-0 items-center gap-3 px-8 py-4 max-[900px]:px-6"
    >
      <span
        class="workspace-page__mark flex size-10 shrink-0 items-center justify-center rounded-[10px] border border-primary/25 bg-primary/10 text-primary"
      >
        <UIcon name="i-tabler-building-store" class="size-5" />
      </span>
      <h1
        class="workspace-page__title truncate text-xl leading-tight font-semibold tracking-[-0.02em] text-highlighted"
      >
        {{ t('sidebar.market') }}
      </h1>
      <div class="ml-auto flex items-center rounded-lg bg-elevated p-1" role="tablist">
        <UButton
          size="sm"
          :variant="contentType === 'workflows' ? 'solid' : 'ghost'"
          :color="contentType === 'workflows' ? 'primary' : 'neutral'"
          role="tab"
          :aria-selected="contentType === 'workflows'"
          data-testid="market-tab-workflows"
          @click="contentType = 'workflows'"
        >
          {{ t('market.tabs.workflows') }}
        </UButton>
        <UButton
          size="sm"
          :variant="contentType === 'plugins' ? 'solid' : 'ghost'"
          :color="contentType === 'plugins' ? 'primary' : 'neutral'"
          role="tab"
          :aria-selected="contentType === 'plugins'"
          data-testid="market-tab-plugins"
          @click="contentType = 'plugins'"
        >
          {{ t('market.tabs.plugins') }}
        </UButton>
      </div>
    </header>

    <KeepAlive>
      <WorkflowMarketPanel v-if="contentType === 'workflows'" />
      <PluginMarketPanel v-else />
    </KeepAlive>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import PluginMarketPanel from '@/components/plugins/PluginMarketPanel.vue'
import WorkflowMarketPanel from '@/components/workflow/WorkflowMarketPanel.vue'

defineOptions({ name: 'MarketView' })

const { t } = useI18n()
const contentType = ref<'workflows' | 'plugins'>('workflows')
</script>
