import { ref } from 'vue'

export function usePublicationEntry(deps: {
  login: () => Promise<unknown>
  cancel: () => Promise<unknown>
  message: (error: unknown) => string
}) {
  const busy = ref(false),
    open = ref(false),
    failure = ref('')
  let generation = 0
  async function start(ready: () => void) {
    if (busy.value) return
    const ticket = ++generation
    busy.value = true
    failure.value = ''
    // A cached session takes the direct path without flashing a login modal.
    const timer = setTimeout(() => {
      if (ticket === generation) open.value = true
    }, 150)
    try {
      await deps.login()
      if (ticket !== generation) return
      open.value = false
      ready()
    } catch (error) {
      if (ticket === generation) {
        failure.value = deps.message(error)
        open.value = true
      }
    } finally {
      clearTimeout(timer)
      if (ticket === generation) busy.value = false
    }
  }
  async function cancel() {
    const wasBusy = busy.value
    generation++
    open.value = false
    failure.value = ''
    // Keep the entry locked until the native cancellation acknowledgement.
    try {
      if (wasBusy) await deps.cancel()
    } catch (error) {
      failure.value = deps.message(error)
      open.value = true
    } finally {
      busy.value = false
    }
  }
  return { busy, open, failure, start, cancel }
}
