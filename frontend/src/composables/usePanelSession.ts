import { onScopeDispose, ref, shallowRef, watch, type Ref } from 'vue'
import { panelBackend, type PanelComponent, type PanelSnapshot } from '@/lib/panels'
import { errorMessage } from '@/lib/invoke'

export function usePanelSession(
  id: Ref<string>,
  visible: Ref<boolean>,
  api: Pick<typeof panelBackend, 'read' | 'dispatch'> = panelBackend,
) {
  const snapshot = shallowRef<PanelSnapshot | null>(null)
  const failure = ref('')
  const actionFailure = ref('')
  const busy = ref('')
  const loading = ref(false)
  let generation = 0
  let disposed = false
  let timer: ReturnType<typeof setTimeout> | undefined
  let reading = false

  function accept(value: PanelSnapshot) {
    const previous = snapshot.value
    if (
      !previous ||
      previous.sessionId !== value.sessionId ||
      value.revision >= previous.revision
    ) {
      snapshot.value = value
    }
  }
  async function refresh() {
    if (disposed || reading || !visible.value || !id.value) return
    reading = true
    loading.value = !snapshot.value
    const current = generation
    const source = id.value
    try {
      const value = await api.read(source)
      if (generation === current && !disposed) {
        accept(value)
        failure.value = ''
      }
    } catch (error) {
      if (generation === current && !disposed) failure.value = errorMessage(error)
    } finally {
      reading = false
      if (generation === current) loading.value = false
      if (!disposed && visible.value && id.value)
        timer = setTimeout(() => void refresh(), failure.value ? 2000 : 250)
    }
  }
  async function dispatch(component: PanelComponent, value: unknown) {
    const state = snapshot.value
    if (!state || busy.value || !component.event || failure.value || state.status === 'ended')
      return
    const current = generation
    busy.value = component.id
    actionFailure.value = ''
    try {
      const result = await api.dispatch(id.value, {
        sessionId: state.sessionId,
        eventId: crypto.randomUUID(),
        componentId: component.id,
        name: component.event,
        revision: state.controlRevisions?.[component.id] ?? 0,
        value,
      })
      if (generation === current && !disposed && snapshot.value?.sessionId === state.sessionId)
        accept(result.snapshot)
    } catch (error) {
      if (generation === current && !disposed) actionFailure.value = errorMessage(error)
    } finally {
      if (generation === current) busy.value = ''
    }
  }
  watch(id, () => {
    generation++
    snapshot.value = null
    failure.value = ''
    actionFailure.value = ''
    busy.value = ''
  })
  watch(
    [id, visible],
    () => {
      clearTimeout(timer)
      if (visible.value) void refresh()
    },
    { immediate: true },
  )
  onScopeDispose(() => {
    disposed = true
    generation++
    clearTimeout(timer)
  })
  return { snapshot, failure, actionFailure, busy, loading, refresh, dispatch }
}
