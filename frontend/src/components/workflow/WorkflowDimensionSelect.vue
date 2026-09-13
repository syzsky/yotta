<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type {
  DimensionFacet,
  FilterDimension,
} from '@bindings/github.com/yottaapp/yotta/internal/registryclient/models.js'
const props = defineProps<{
  dimensions: FilterDimension[]
  assignment?: boolean
  counts?: DimensionFacet[]
}>()
const model = defineModel<string[]>({ default: () => [] })
const dimensions = computed(() =>
  props.dimensions
    .filter(
      (d) =>
        d.active && (props.assignment ? d.authorVisible !== false : d.discoveryVisible !== false),
    )
    .slice()
    .sort((a, b) => a.position - b.position),
)
const activeDimensionId = ref('')
const activeDimension = computed(
  () => dimensions.value.find((dimension) => dimension.id === activeDimensionId.value) ?? null,
)
watch(
  dimensions,
  (next) => {
    if (!next.some((dimension) => dimension.id === activeDimensionId.value))
      activeDimensionId.value = next[0]?.id ?? ''
  },
  { immediate: true },
)
function rows(dim: FilterDimension) {
  const result: { id: string; name: string; depth: number; leaf: boolean; assignable: boolean }[] =
    []
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
        assignable: v.assignable !== false,
        leaf: !dim.values.some((child) => child.parentId === v.id && child.active),
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
function selectedCount(dim: FilterDimension): number {
  const ids = new Set(dim.values.map((value) => value.id))
  return model.value.filter((id) => ids.has(id)).length
}
function valueLabel(dimensionId: string, value: { id: string; name: string }): string {
  const count = props.counts
    ?.find((d) => d.id === dimensionId)
    ?.values.find((v) => v.value === value.id)?.count
  return count === undefined || props.assignment ? value.name : `${value.name} (${count})`
}
</script>
<template>
  <div v-if="assignment && dimensions.length" class="space-y-3">
    <div class="flex flex-wrap gap-1 rounded-lg bg-elevated p-1" role="tablist">
      <button
        v-for="dim in dimensions"
        :key="dim.id"
        type="button"
        role="tab"
        :aria-selected="activeDimensionId === dim.id"
        class="flex items-center gap-1.5 rounded-md px-3 py-1.5 text-xs font-medium transition-colors"
        :class="
          activeDimensionId === dim.id
            ? 'bg-default text-highlighted shadow-sm'
            : 'text-muted hover:text-default'
        "
        @click="activeDimensionId = dim.id"
      >
        <span>{{ dim.name }}</span>
        <span
          v-if="selectedCount(dim)"
          class="rounded-full bg-primary/12 px-1.5 py-0.5 text-[10px] tabular-nums text-primary"
          >{{ selectedCount(dim) }}</span
        >
      </button>
    </div>
    <fieldset v-if="activeDimension" class="space-y-1" role="tabpanel">
      <legend class="sr-only">{{ activeDimension.name }}</legend>
      <div
        v-for="value in rows(activeDimension)"
        :key="value.id"
        :style="{ paddingInlineStart: `${value.depth * 12}px` }"
      >
        <UCheckbox
          :model-value="model.includes(value.id)"
          :label="valueLabel(activeDimension.id, value)"
          :disabled="
            !model.includes(value.id) &&
            (!value.assignable || (activeDimension.leafOnly && !value.leaf))
          "
          @update:model-value="
            (checked: boolean | 'indeterminate') => toggle(value.id, checked === true)
          "
        />
      </div>
    </fieldset>
  </div>
  <div v-else class="space-y-3">
    <fieldset v-for="dim in dimensions" :key="dim.id" class="space-y-1">
      <legend class="mb-2 text-xs font-medium">{{ dim.name }}</legend>
      <div
        v-for="value in rows(dim)"
        :key="value.id"
        :style="{ paddingInlineStart: `${value.depth * 12}px` }"
      >
        <UCheckbox
          :model-value="model.includes(value.id)"
          :label="valueLabel(dim.id, value)"
          :disabled="
            assignment &&
            !model.includes(value.id) &&
            (!value.assignable || (dim.leafOnly && !value.leaf))
          "
          @update:model-value="
            (checked: boolean | 'indeterminate') => toggle(value.id, checked === true)
          "
        />
      </div>
    </fieldset>
  </div>
</template>
