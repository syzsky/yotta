import { computed, ref, type Ref } from 'vue'
import type { SourceView } from '@/app/transport/workflow'

export type SelectedWorkflowSource = Pick<
  SourceView,
  'workflowId' | 'name' | 'revision' | 'sourceHash' | 'category' | 'tags'
>

export function useWorkflowLibrarySelection(currentPage: Ref<SourceView[]>) {
  const selected = ref<Record<string, SelectedWorkflowSource>>({})
  const rows = computed(() => Object.values(selected.value))
  const allCurrentPageSelected = computed(
    () =>
      currentPage.value.length > 0 &&
      currentPage.value.every((source) => Boolean(selected.value[source.workflowId])),
  )

  function toggle(source: SourceView, checked: boolean): void {
    const next = { ...selected.value }
    if (checked) next[source.workflowId] = source
    else delete next[source.workflowId]
    selected.value = next
  }

  function toggleCurrentPage(checked: boolean): void {
    const next = { ...selected.value }
    for (const source of currentPage.value) {
      if (checked) next[source.workflowId] = source
      else delete next[source.workflowId]
    }
    selected.value = next
  }

  function clear(): void {
    selected.value = {}
  }

  function retainOnly(workflowIds: string[]): void {
    const retained = new Set(workflowIds)
    selected.value = Object.fromEntries(
      rows.value
        .filter((source) => retained.has(source.workflowId))
        .map((source) => [source.workflowId, source]),
    )
  }

  function remove(workflowIds: string[]): void {
    if (!workflowIds.length) return
    const next = { ...selected.value }
    for (const workflowId of workflowIds) delete next[workflowId]
    selected.value = next
  }

  function name(workflowId: string): string {
    return selected.value[workflowId]?.name ?? workflowId
  }

  return {
    selected,
    rows,
    allCurrentPageSelected,
    toggle,
    toggleCurrentPage,
    clear,
    retainOnly,
    remove,
    name,
  }
}
