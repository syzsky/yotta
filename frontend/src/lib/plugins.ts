import { Dialogs } from '@wailsio/runtime'
import { callRPC } from './invoke'
import type { NormalizedError } from './invoke'
import { i18n } from '@/i18n'
import type {
  Facets,
  NodePackRelease,
  SearchOptions,
} from '@bindings/github.com/yottaapp/yotta/internal/registryclient/models.js'

type Bindings =
  typeof import('@bindings/github.com/yottaapp/yotta/internal/services/plugins/service.js')
export interface PluginView {
  id: string
  name: string
  description: string
  version: string
  inUse: boolean
  enabled: boolean
  loaded: boolean
  restartRequired: boolean
  canRollback: boolean
  nodes: string[]
  workflows: { id: string; name: string; running: boolean }[]
  companions: { id: string; name: string; running: boolean; status: string }[]
}

export type PluginBatchAction = 'enable' | 'disable' | 'uninstall'
export interface PluginLocation {
  kind: string
  path: string
}
export interface PluginBatchResult {
  id: string
  succeeded: boolean
  problem?: NormalizedError
}
export interface PluginRegistrySearchPage {
  items: NodePackRelease[]
  facets: Facets
  nextCursor?: string
}
export interface PluginRegistryInstallResult {
  plugin: PluginView
  alreadyInstalled: boolean
  planId: string
}

function invokePlugin<K extends keyof Bindings>(method: K, ...args: Parameters<Bindings[K]>) {
  return callRPC<unknown>(method, async () => {
    const service =
      await import('@bindings/github.com/yottaapp/yotta/internal/services/plugins/service.js')
    return Reflect.apply(service[method], undefined, args)
  }) as Promise<Awaited<ReturnType<Bindings[K]>>>
}

export const pluginBackend = {
  locations: (id = '') => invokePlugin('Locations', id) as Promise<PluginLocation[]>,
  openLocation: (kind: string, id = '') => invokePlugin('OpenLocation', kind, id),
  batch: (action: PluginBatchAction, ids: string[]) =>
    invokePlugin('Batch', action, ids) as Promise<PluginBatchResult[]>,
  needsRestart: () => invokePlugin('NeedsRestart'),
  list: () => invokePlugin('List') as Promise<PluginView[]>,
  discoverRegistry: (options: SearchOptions) =>
    invokePlugin('DiscoverRegistry', options) as Promise<PluginRegistrySearchPage>,
  installRegistry: (releaseID: string) =>
    invokePlugin('InstallRegistry', releaseID) as Promise<PluginRegistryInstallResult>,
  import: (path: string) => invokePlugin('Import', path),
  setEnabled: (id: string, enabled: boolean) => invokePlugin('SetEnabled', id, enabled),
  uninstall: (id: string) => invokePlugin('Uninstall', id),
  rollback: (id: string) => invokePlugin('Rollback', id),
  control: (id: string, companion: string, start: boolean) =>
    invokePlugin('Control', id, companion, start),
  pick: (title: string) =>
    callRPC('plugins.pick', () =>
      Dialogs.OpenFile({
        Title: title,
        AllowsMultipleSelection: false,
        Filters: [{ DisplayName: 'Yotta plugin', Pattern: '*.ynp' }],
      }),
    ) as Promise<string>,
}

const ownedKeys = new Set<string>()
export async function loadPluginMessages() {
  const messages = await invokePlugin('Messages')
  for (const locale of ['zh', 'en'] as const) {
    const tree: Record<string, unknown> = Object.create(null)
    for (const [key, value] of Object.entries(messages[locale] ?? {})) {
      if (typeof value !== 'string') continue
      if (i18n.global.te(key, locale) && !ownedKeys.has(key)) continue
      const parts = key.split('.')
      if (parts.some((part) => ['__proto__', 'prototype', 'constructor'].includes(part))) continue
      let current = tree
      for (const part of parts.slice(0, -1)) {
        current[part] ??= Object.create(null)
        current = current[part] as Record<string, unknown>
      }
      current[parts.at(-1)!] = value
      ownedKeys.add(key)
    }
    i18n.global.mergeLocaleMessage(locale, tree)
  }
}
