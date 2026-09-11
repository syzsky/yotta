import { callRPC } from './invoke'

export interface PanelOption {
  value: string
  labelKey: string
}
export interface PanelComponent {
  icon?: string
  id: string
  kind: string
  titleKey: string
  field?: string
  unitKey?: string
  event?: string
  precision?: number
  options?: PanelOption[]
  children?: PanelComponent[]
}
export interface PanelSource {
  listeningComponents?: string[]
  waitingComponents?: string[]
  waiting?: number
  managed: boolean
  updatedAt?: string
  lastRunId?: string
  lastRunStatus?: string
  id: string
  ownerId: string
  ownerName: string
  generation: string
  status?: string
  definition: {
    format: string
    id: string
    titleKey: string
    descriptionKey?: string
    fields?: Array<{ id: string; kind: string }>
    components: PanelComponent[]
  }
}
export interface PanelRecord {
  id: string
  timeMs: number
  level: string
  text?: string
  messageKey?: string
}
export interface PanelSnapshot {
  protocol: string
  sessionId: string
  revision: number
  status: string
  values: Record<string, string | number | boolean>
  controlRevisions: Record<string, number>
  records: Record<string, PanelRecord[]>
}
export interface PanelEvent {
  sessionId: string
  eventId: string
  componentId: string
  name: string
  revision: number
  value: unknown
}
type Bindings =
  typeof import('@bindings/github.com/yottaapp/yotta/internal/services/panels/service.js')
function rpc(method: keyof Bindings, ...args: unknown[]) {
  return callRPC(method, async () => {
    const service =
      await import('@bindings/github.com/yottaapp/yotta/internal/services/panels/service.js')
    return Reflect.apply(service[method], undefined, args)
  })
}
export const panelBackend = {
  save: (draft: PanelDraft) => rpc('Save', draft) as Promise<PanelDraft>,
  edit: (id: string) => rpc('Edit', id) as Promise<PanelDraft>,
  remove: (id: string, revision: number) => rpc('Delete', id, revision) as Promise<void>,
  show: (id: string) => rpc('Show', id) as Promise<void>,
  selected: () => rpc('Selected') as Promise<string>,
  list: () => rpc('List') as Promise<PanelSource[]>,
  read: (id: string) => rpc('Read', id) as Promise<PanelSnapshot>,
  dispatch: (id: string, event: PanelEvent) =>
    rpc('Dispatch', id, event) as Promise<{ eventId: string; snapshot: PanelSnapshot }>,
}

export interface PanelComponentDraft {
  icon?: string
  id: string
  kind: string
  title: string
  initial: string | number | boolean | null
  options?: string[]
}
export interface PanelDraft {
  id: string
  revision: number
  title: string
  components: PanelComponentDraft[]
}
