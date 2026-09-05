import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

const read = (path: string) => readFileSync(join(process.cwd(), path), 'utf8')

describe('frontend module seams', () => {
  it('keeps workflow editor interaction state behind its established modules', () => {
    const view = read('src/views/WorkflowEditorView.vue')
    expect(view).toContain('useEditorPanelLayout(')
    expect(view).toContain('useWorkflowNodeSearch(')
    expect(view).toContain('useWorkflowConnectionAuthoring(')
    expect(view).toContain('useWorkflowRuntimeWorkbench(')
    expect(view).toContain('useWorkflowSubgraphManagement(')
    expect(view).toContain('useWorkflowResourceAuthoring(')
    expect(view).toContain('useWorkflowSnippetAuthoring(')
    expect(view).toContain('useWorkflowQuickAdd(')
    expect(view).toContain('useWorkflowCanvasGestures(')
    expect(view).toContain('useWorkflowCanvasAssist(')
    expect(view).toContain('useWorkflowStateAuthoring(')
    expect(view).toContain('useWorkflowEditorDrop(')
    expect(view).toContain('useWorkflowEdgeInteractions(')
    expect(view).toContain('<WorkflowEditorCanvas')
    expect(view).toContain('<WorkflowEditorDialogs')
    expect(view).toContain('<WorkflowRecordingDialogs')
    expect(view).not.toMatch(/^function (connect|isValidConnection|startConnection)\b/m)
    expect(view).not.toMatch(
      /^function (toggleBreakpoint|debugBreakpoints|toggleRuntimeWorkbench)\b/m,
    )
    expect(view).not.toMatch(
      /^function (inferGraphInterface|addGraphCall|deleteGraphDefinition)\b/m,
    )
    expect(view).not.toMatch(
      /^function (importWorkflowResource|placeWorkflowResource|useSnippet)\b/m,
    )
    expect(view).not.toMatch(/^function (openQuickAdd|finishMarqueeSelection|trackNodeDrag)\b/m)
    expect(view).not.toMatch(
      /^function (insertStateReference|continueNodeDrag|selectedSourceEdges)\b/m,
    )
  })

  it('keeps library query and cross-page selection rules outside page views', () => {
    const workflows = read('src/views/WorkflowsView.vue')
    const assets = read('src/views/AssetsView.vue')
    expect(workflows).toContain('useWorkflowLibraryQuery(')
    expect(workflows).toContain('useWorkflowLibrarySelection(')
    expect(workflows).not.toMatch(/^function (applySearch|queryChanged|toggleCurrentPage)\b/m)
    expect(assets).toContain('useAssetLibraryBrowse(')
    expect(assets).not.toMatch(/^async function (refreshAssets|applyQuery|changeQuery)\b/m)
  })

  it('keeps target drafts and schedule filtering in pure models', () => {
    const automation = read('src/views/SettingsAutomation.vue')
    const schedules = read('src/views/SchedulesView.vue')
    expect(automation).toContain("from '@/app/settings/automationTargetDraft'")
    expect(automation).not.toContain('interface AutomationTargetDraft')
    expect(schedules).toContain('filterSchedules(store.list')
    expect(schedules).not.toMatch(/^function (rangeStart|compareSchedules|facetValues)\b/m)
  })
})
