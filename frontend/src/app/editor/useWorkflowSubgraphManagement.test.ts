import { ref } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import type { EditorSession } from './EditorSession'
import { useWorkflowSubgraphManagement } from './useWorkflowSubgraphManagement'

function createHarness(overrides: Partial<EditorSession> = {}) {
  const session = {
    source: {
      entryGraph: 'main',
      graphs: [
        { id: 'main', kind: 'main', name: '', nodes: [], edges: [], calls: [] },
        {
          id: 'child',
          kind: 'subgraph',
          name: 'Child',
          nodes: [],
          edges: [],
          calls: [{ id: 'nested-call', graphId: 'nested', position: { x: 0, y: 0 } }],
          inputs: [],
          outputs: [],
          entries: [],
          exits: [],
        },
        {
          id: 'nested',
          kind: 'subgraph',
          name: 'Nested',
          nodes: [],
          edges: [],
          calls: [],
          inputs: [],
          outputs: [],
          entries: [],
          exits: [],
        },
      ],
    },
    currentGraph: {
      id: 'nested',
      kind: 'subgraph',
      nodes: [],
      edges: [],
      calls: [],
      inputs: [],
      outputs: [],
      entries: [],
      exits: [],
    },
    graphPath: ['main', 'nested'],
    currentGraphInterfaceReadiness: vi.fn(() => ({ valid: false, reason: 'multiple-entry' })),
    currentGraphInterfaceCandidates: vi.fn(() => []),
    currentGraphInterfaceReferences: vi.fn(() => []),
    insertGraphCall: vi.fn(() => 'call-new'),
    openGraphPath: vi.fn(),
    calleeGraph: vi.fn(() => null),
    graphInputProjection: vi.fn(() => null),
    ...overrides,
  } as unknown as EditorSession
  const selectedNodeId = ref('')
  const selectedNodeIds = ref(new Set<string>())
  const warn = vi.fn()
  const clearSelection = vi.fn()
  const fitCurrentGraph = vi.fn()
  const focusNode = vi.fn(async () => undefined)
  const management = useWorkflowSubgraphManagement({
    session,
    canvasElement: ref(null),
    selectedNodeId,
    selectedNodeIds,
    screenToFlowCoordinate: (position) => position,
    clearSelection,
    fitCurrentGraph,
    focusNode,
    confirm: vi.fn(async () => true),
    translate: (key) => key,
    warn,
    showError: vi.fn(),
  })
  return {
    session,
    selectedNodeId,
    selectedNodeIds,
    warn,
    clearSelection,
    fitCurrentGraph,
    focusNode,
    management,
  }
}

describe('useWorkflowSubgraphManagement', () => {
  it('excludes the current graph and graph definitions that would create a call cycle', () => {
    const { management } = createHarness()

    expect(management.callableGraphIds.value).toEqual([])
  })

  it('owns graph call placement and resulting selection', () => {
    const { management, session, selectedNodeId, selectedNodeIds } = createHarness()

    management.addGraphCall('child', { x: 20, y: 40 })

    expect(session.insertGraphCall).toHaveBeenCalledWith('child', { x: 20, y: 40 })
    expect(selectedNodeId.value).toBe('call-new')
    expect([...selectedNodeIds.value]).toEqual(['call-new'])
  })

  it('keeps interface inference blocked inside the module', async () => {
    const { management, warn } = createHarness()

    await management.inferGraphInterface()

    expect(warn).toHaveBeenCalledWith(
      'workflow.graphs.infer_interface_blocked',
      'workflow.graphs.infer_multiple_entries',
    )
  })

  it('opens graph paths and delegates locating a call through the entry graph', async () => {
    const { management, session, clearSelection, focusNode } = createHarness()

    management.openCalledGraph('child')
    await management.locateGraphCall('child', 'call-a')

    expect(session.openGraphPath).toHaveBeenCalledWith(['main', 'child'])
    expect(clearSelection).toHaveBeenCalledOnce()
    expect(focusNode).toHaveBeenCalledWith(['main', 'child'], 'call-a')
  })
})
