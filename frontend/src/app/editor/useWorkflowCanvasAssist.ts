import { computed, type Ref } from 'vue'
import type { useSettingsStore } from '@/stores/settings'
import type { EditorSession, NodeProjection } from './EditorSession'

interface WorkflowCanvasAssistOptions {
  session: EditorSession
  settings: ReturnType<typeof useSettingsStore>
  catalogNodes: Readonly<Ref<NodeProjection[]>>
  canvasElement: Ref<HTMLElement | null>
  screenToFlowCoordinate: (position: { x: number; y: number }) => { x: number; y: number }
  insertionPosition: () => { x: number; y: number }
  projectionTitle: (projection: NodeProjection) => string
  addNode: (nodeTypeId: string, position: { x: number; y: number }) => void
  makeSpace: () => void
}

export function useWorkflowCanvasAssist(options: WorkflowCanvasAssistOptions) {
  const state = computed(
    () =>
      options.settings.data?.ui.canvasAssist ?? {
        collapsed: false,
        hidden: false,
        display: 'labels' as const,
        favoriteNodeTypeIds: [],
      },
  )
  const favorites = computed(() =>
    state.value.favoriteNodeTypeIds.map((nodeTypeId) => {
      const projection = options.catalogNodes.value.find(
        (candidate) => candidate.nodeRef.nodeTypeId === nodeTypeId,
      )
      return {
        nodeTypeId,
        title: projection ? options.projectionTitle(projection) : nodeTypeId,
        icon: projection?.icon ?? 'box',
        available: Boolean(projection),
      }
    }),
  )
  const nodeOptions = computed(() =>
    options.catalogNodes.value.map((projection) => ({
      label: options.projectionTitle(projection),
      value: projection.nodeRef.nodeTypeId,
    })),
  )

  function addFavorite(nodeTypeId: string): void {
    if (options.session.nodeProjection(nodeTypeId))
      options.addNode(nodeTypeId, options.insertionPosition())
  }

  function addFavoriteFromToolbar(nodeTypeId: string): void {
    if (!options.session.nodeProjection(nodeTypeId)) return
    const rect = options.canvasElement.value?.getBoundingClientRect()
    options.addNode(
      nodeTypeId,
      rect
        ? options.screenToFlowCoordinate({
            x: rect.left + rect.width / 2,
            y: rect.top + rect.height / 2,
          })
        : { x: 160, y: 160 },
    )
  }

  function setCollapsed(collapsed: boolean): void {
    void options.settings.patch({ ui: { canvasAssist: { ...state.value, collapsed } } })
  }

  function setHidden(hidden: boolean): void {
    void options.settings.patch({ ui: { canvasAssist: { ...state.value, hidden } } })
  }

  function setFavorites(favoriteNodeTypeIds: string[]): void {
    void options.settings.patch({
      ui: { canvasAssist: { ...state.value, favoriteNodeTypeIds } },
    })
  }

  return {
    state,
    favorites,
    nodeOptions,
    addFavorite,
    addFavoriteFromToolbar,
    makeSpace: options.makeSpace,
    setCollapsed,
    setHidden,
    setFavorites,
  }
}
