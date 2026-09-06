import { ref } from 'vue'
import { panelBackend, type PanelSource } from '@/lib/panels'
import { errorMessage } from '@/lib/invoke'
import { loadPluginMessages } from '@/lib/plugins'
const items = ref<PanelSource[]>([])
const failure = ref('')
let pending: Promise<void> | undefined
export function usePanelCatalog() {
  const refresh = () =>
    (pending ??= Promise.all([panelBackend.list(), loadPluginMessages()])
      .then(([value]) => {
        items.value = value
        failure.value = ''
      })
      .catch((e) => {
        failure.value = errorMessage(e)
      })
      .finally(() => {
        pending = undefined
      }))
  return { items, failure, refresh }
}
