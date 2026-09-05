import { onScopeDispose, ref, shallowRef, watch, type Ref } from 'vue'
import { backend, type TemplateMatchPreviewRequest } from '@/lib/backend'
import { errorMessage } from '@/lib/invoke'

export function useTemplateMatchPreview(
  request: Readonly<Ref<TemplateMatchPreviewRequest | null>>,
) {
  const enabled = ref(false)
  const loading = ref(false)
  const result = shallowRef<{
    score: number
    matched: boolean
    frameWidth: number
    frameHeight: number
  } | null>(null)
  const failure = ref('')
  let generation = 0
  let pending = false
  let timer: ReturnType<typeof setTimeout> | undefined

  async function tick(current: number): Promise<void> {
    if (current !== generation || !enabled.value || !request.value) return
    if (pending) {
      timer = setTimeout(() => void tick(current), 100)
      return
    }
    pending = true
    loading.value = true
    try {
      const next = await backend.tools.previewTemplate(request.value)
      if (current === generation) {
        result.value = next
        failure.value = ''
      }
    } catch (error) {
      if (current === generation) {
        result.value = null
        failure.value = errorMessage(error)
      }
    } finally {
      pending = false
      if (current === generation) {
        loading.value = false
        timer = setTimeout(() => void tick(current), 750)
      }
    }
  }

  watch(
    [enabled, () => JSON.stringify(request.value)],
    () => {
      const current = ++generation
      clearTimeout(timer)
      result.value = null
      failure.value = ''
      loading.value = false
      if (enabled.value && request.value) void tick(current)
    },
    { flush: 'sync' },
  )
  onScopeDispose(() => {
    generation++
    clearTimeout(timer)
  })
  return { enabled, loading, result, failure }
}
