<script setup lang="ts">
import { computed } from 'vue'
import type { FilterDimension } from '@bindings/github.com/yottaapp/yotta/internal/registryclient/models.js'
const props = defineProps<{ dimensions: FilterDimension[]; assignment?: boolean }>()
const model = defineModel<string[]>({ default: () => [] })
const dimensions = computed(() =>
  props.dimensions
    .filter((d) => d.active)
    .slice()
    .sort((a, b) => a.position - b.position),
)
function rows(dim: FilterDimension) {
  const result: { id: string; name: string; depth: number; leaf: boolean }[] = []
  const seen = new Set<string>()
  function visit(parent: string, depth: number) {
    for (const v of dim.values
      .filter((v) => v.parentId === parent && v.active)
      .slice()
      .sort((a, b) => a.position - b.position)) {
      if (seen.has(v.id)) continue
      seen.add(v.id)
      result.push({
        id: v.id,
        name: v.name,
        depth,
        leaf: !dim.values.some((child) => child.parentId === v.id),
      })
      visit(v.id, depth + 1)
    }
  }
  visit('', 0)
  return result
}
function toggle(id: string, checked: boolean) {
  model.value = checked ? [...new Set([...model.value, id])] : model.value.filter((v) => v !== id)
}
</script>
<template>
  <div class="space-y-3">
    <fieldset v-for="dim in dimensions" :key="dim.id" class="space-y-1">
      <legend class="mb-2 text-xs font-medium">{{ dim.name }}</legend>
      <div
        v-for="value in rows(dim)"
        :key="value.id"
        :style="{ paddingInlineStart: `${value.depth * 12}px` }"
      >
        <UCheckbox
          :model-value="model.includes(value.id)"
          :label="value.name"
          :disabled="assignment && dim.leafOnly && !value.leaf"
          @update:model-value="
            (checked: boolean | 'indeterminate') => toggle(value.id, checked === true)
          "
        />
      </div>
    </fieldset>
  </div>
</template>
