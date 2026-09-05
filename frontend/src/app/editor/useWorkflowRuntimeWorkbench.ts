import { computed, ref } from 'vue'
import type { DebugBreakpoint } from '@/app/transport/workflow'
import type { EditorSession } from './EditorSession'
import type { EditorRuntimeWorkbenchTab } from './EditorRunController'

interface RuntimeWorkbenchOptions {
  session: EditorSession
  translate: (key: string) => string
  showError: (title: string, error: unknown) => void
}

export function useWorkflowRuntimeWorkbench(options: RuntimeWorkbenchOptions) {
  const open = ref(false)
  const tab = ref<EditorRuntimeWorkbenchTab>('logs')
  const breakpointKeys = ref(new Set<string>())
  const diagnosticsOpen = computed(() => open.value && tab.value === 'diagnostics')
  const runTimelineOpen = computed(() => open.value && tab.value === 'timeline')
  const debuggerOpen = computed(() => open.value && tab.value === 'debug')
  const debugModeActive = computed(
    () =>
      debuggerOpen.value ||
      Boolean(
        options.session.debugSnapshot && options.session.debugSnapshot.status !== 'completed',
      ),
  )

  function show(nextTab: EditorRuntimeWorkbenchTab): void {
    tab.value = nextTab
    open.value = true
  }

  function toggle(nextTab: EditorRuntimeWorkbenchTab): void {
    if (open.value && tab.value === nextTab) {
      open.value = false
      return
    }
    show(nextTab)
  }

  async function toggleBreakpoint(graphId: string, nodeId: string): Promise<void> {
    if (!graphId || !nodeId) return
    const key = breakpointKey(graphId, nodeId)
    const hadBreakpoint = breakpointKeys.value.has(key)
    const next = new Set(breakpointKeys.value)
    if (next.has(key)) next.delete(key)
    else next.add(key)
    breakpointKeys.value = next
    if (!options.session.debugSnapshot || options.session.debugSnapshot.status === 'completed')
      return
    try {
      await options.session.setDebugBreakpoints(breakpoints())
    } catch (error) {
      const rollback = new Set(breakpointKeys.value)
      if (hadBreakpoint) rollback.add(key)
      else rollback.delete(key)
      breakpointKeys.value = rollback
      options.showError(options.translate('workflow.toast.debug_failed'), error)
    }
  }

  function breakpoints(): DebugBreakpoint[] {
    return [...breakpointKeys.value].map((key) => {
      const separator = key.indexOf('\u0000')
      return { graphId: key.slice(0, separator), nodeId: key.slice(separator + 1) }
    })
  }

  function hasBreakpoint(graphId: string, nodeId: string): boolean {
    return breakpointKeys.value.has(breakpointKey(graphId, nodeId))
  }

  function isCurrent(graphId: string, nodeId: string): boolean {
    const snapshot = options.session.debugSnapshot
    return (
      snapshot?.status === 'paused' && snapshot.graphId === graphId && snapshot.nodeId === nodeId
    )
  }

  return {
    open,
    tab,
    diagnosticsOpen,
    runTimelineOpen,
    debuggerOpen,
    debugModeActive,
    show,
    toggle,
    toggleBreakpoint,
    breakpoints,
    hasBreakpoint,
    isCurrent,
  }
}

function breakpointKey(graphId: string, nodeId: string): string {
  return `${graphId}\u0000${nodeId}`
}
