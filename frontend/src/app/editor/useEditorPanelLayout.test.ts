import { describe, expect, it } from 'vitest'
import { useEditorPanelLayout } from './useEditorPanelLayout'

describe('useEditorPanelLayout', () => {
  it('owns workspace toggling and restores a usable width', () => {
    const layout = useEditorPanelLayout(true)
    layout.workspaceSidebarWidth.value = 250
    layout.toggleWorkspacePanel('variables')
    expect(layout.workspacePanel.value).toBe('variables')
    expect(layout.workspaceSidebarOpen.value).toBe(true)
    expect(layout.workspaceSidebarWidth.value).toBe(320)
    layout.toggleWorkspacePanel('variables')
    expect(layout.workspaceSidebarOpen.value).toBe(false)
  })

  it('clamps both sidebars to their supported ranges', () => {
    const layout = useEditorPanelLayout(false)
    expect(layout.resizeWorkspaceSidebar(320, -500)).toBe(240)
    expect(layout.resizeWorkspaceSidebar(320, 500)).toBe(480)
    expect(layout.resizeInspectorSidebar(360, -500)).toBe(280)
    expect(layout.resizeInspectorSidebar(360, 500)).toBe(560)
  })
})
