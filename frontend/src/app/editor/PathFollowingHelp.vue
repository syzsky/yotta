<template>
  <section v-if="isPathFollower(nodeTypeId)" class="space-y-2 text-xs leading-5">
    <p class="text-muted">{{ t('node.navigation.followPath.help.intro') }}</p>
    <details class="nodrag nowheel">
      <summary class="cursor-pointer font-medium text-primary">
        {{ t('node.navigation.followPath.help.title') }}
      </summary>
      <dl class="mt-3 space-y-3">
        <div v-for="port in ports" :key="port">
          <dt class="font-medium text-toned">{{ t(`workflow.node.port.${port}`) }}</dt>
          <dd class="mt-1 text-muted">{{ t(`node.navigation.followPath.help.${port}`) }}</dd>
        </div>
      </dl>
      <p class="mt-3 text-muted">{{ t('node.navigation.followPath.help.data') }}</p>
      <p class="mt-3 text-muted">{{ t('node.navigation.followPath.help.start') }}</p>
    </details>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { isPathFollower } from './pathFollowingAuthoring'

defineProps<{ nodeTypeId: string }>()
const { t } = useI18n()
const ports = ['moving', 'marker', 'recover', 'stuck'] as const
</script>
