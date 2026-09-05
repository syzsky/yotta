import { describe, expect, it, vi } from 'vitest'
import type { EditorSession } from './EditorSession'
import { useWorkflowRuntimeWorkbench } from './useWorkflowRuntimeWorkbench'

describe('useWorkflowRuntimeWorkbench', () => {
  it('owns mutually visible runtime tabs and breakpoint identity', async () => {
    const session = { debugSnapshot: null } as EditorSession
    const workbench = useWorkflowRuntimeWorkbench({
      session,
      translate: (key) => key,
      showError: vi.fn(),
    })
    workbench.show('diagnostics')
    expect(workbench.diagnosticsOpen.value).toBe(true)
    workbench.toggle('timeline')
    expect(workbench.runTimelineOpen.value).toBe(true)
    await workbench.toggleBreakpoint('main', 'node-a')
    expect(workbench.breakpoints()).toEqual([{ graphId: 'main', nodeId: 'node-a' }])
  })
})
