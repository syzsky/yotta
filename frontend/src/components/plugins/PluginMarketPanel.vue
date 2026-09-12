<template>
  <main class="plugin-market-browser" data-testid="plugin-market">
    <aside class="plugin-market-rail" :aria-label="t('market.plugins.catalog')">
      <div class="plugin-market-search">
        <div class="flex items-center gap-1">
          <form class="min-w-0 flex-1" role="search" @submit.prevent="search">
            <UInput
              v-model="query"
              icon="i-tabler-search"
              :loading="loading"
              size="sm"
              :placeholder="t('market.plugins.search_placeholder')"
              :aria-label="t('market.plugins.search')"
              class="w-full"
            >
              <template #trailing>
                <UButton
                  v-if="query"
                  icon="i-tabler-x"
                  size="xs"
                  color="neutral"
                  variant="link"
                  :aria-label="t('market.plugins.clear_search')"
                  @click="clearSearch"
                />
              </template>
            </UInput>
          </form>
          <UPopover :content="{ align: 'end', side: 'bottom' }">
            <UButton
              icon="i-tabler-filter"
              size="sm"
              :color="hasDiscoveryFilters ? 'primary' : 'neutral'"
              :variant="hasDiscoveryFilters ? 'soft' : 'ghost'"
              :aria-label="t('market.plugins.filter')"
              data-testid="plugin-market-filter-toggle"
            />
            <template #content>
              <div class="w-80 max-w-[calc(100vw-2rem)] space-y-3 p-3">
                <UFormField :label="t('market.plugins.category')" size="sm">
                  <select v-model="category" class="plugin-market-select" @change="search">
                    <option value="">{{ t('market.plugins.all_categories') }}</option>
                    <option v-for="value in facets.categories" :key="value" :value="value">
                      {{ value }}
                    </option>
                  </select>
                </UFormField>
                <UFormField v-if="facets.tags.length" :label="t('market.plugins.tags')" size="sm">
                  <select v-model="tag" class="plugin-market-select" @change="search">
                    <option value="">{{ t('market.plugins.all_tags') }}</option>
                    <option v-for="value in facets.tags" :key="value" :value="value">
                      {{ value }}
                    </option>
                  </select>
                </UFormField>
                <UFormField :label="t('market.plugins.sort')" size="sm">
                  <select v-model="sort" class="plugin-market-select" @change="search">
                    <option value="updated">{{ t('market.plugins.sort_updated') }}</option>
                    <option value="name">{{ t('market.plugins.sort_name') }}</option>
                  </select>
                </UFormField>
                <UCheckbox
                  v-model="qualityOnly"
                  :label="t('market.plugins.quality_authors_only')"
                  @update:model-value="search"
                />
              </div>
            </template>
          </UPopover>
        </div>
        <div class="mt-3 flex items-center gap-1" :aria-label="t('market.plugins.filter')">
          <button
            v-for="option in statusFilters"
            :key="option.value"
            type="button"
            class="plugin-market-filter"
            :aria-pressed="statusFilter === option.value"
            @click="statusFilter = option.value"
          >
            {{ option.label }}
          </button>
          <span class="ml-auto text-xs tabular-nums text-muted">{{ visibleItems.length }}</span>
        </div>
      </div>

      <div class="min-h-0 flex-1 overflow-y-auto">
        <div v-if="loading" class="space-y-3 p-4">
          <USkeleton v-for="index in 4" :key="index" class="h-16 rounded-md" />
        </div>
        <div v-else-if="failure" role="alert" class="space-y-3 p-4 text-sm text-error">
          <p class="whitespace-pre-wrap">{{ failure }}</p>
          <UButton color="neutral" variant="outline" size="sm" @click="search">
            {{ t('common.retry') }}
          </UButton>
        </div>
        <EmptyState
          v-else-if="!visibleItems.length"
          inset
          icon="i-tabler-plug-connected-x"
          :title="t('market.plugins.empty_title')"
          :description="t('market.plugins.empty_description')"
        />
        <button
          v-for="item in visibleItems"
          v-else
          :key="item.releaseId"
          type="button"
          class="plugin-market-result"
          :class="{ 'is-selected': selected?.releaseId === item.releaseId }"
          :aria-pressed="selected?.releaseId === item.releaseId"
          :data-package-id="item.packageId"
          @click="select(item)"
        >
          <span class="plugin-market-icon">
            <UIcon name="i-tabler-plug-connected" class="size-6" />
          </span>
          <span class="plugin-market-result-body">
            <span class="plugin-market-result-heading">
              <strong class="plugin-market-result-title" :title="item.title">{{
                item.title
              }}</strong>
              <UIcon
                v-if="installedFor(item)"
                :name="hasUpdate(item) ? 'i-tabler-refresh' : 'i-tabler-circle-check'"
                class="size-3.5 shrink-0 text-primary"
              />
            </span>
            <span class="plugin-market-result-summary" :title="item.summary">{{
              item.summary
            }}</span>
            <span class="plugin-market-result-meta">
              <span class="plugin-market-result-author" :title="creator(item)">{{
                creator(item)
              }}</span>
              <UBadge
                v-if="item.creator.qualityAuthor"
                color="info"
                variant="subtle"
                icon="i-tabler-award"
                size="sm"
                class="shrink-0 whitespace-nowrap"
              >
                {{ t('market.plugins.quality_author') }}
              </UBadge>
              <span v-if="item.listing.category" class="plugin-market-result-category">
                {{ item.listing.category }}
              </span>
            </span>
          </span>
        </button>
        <div v-if="nextCursor" class="p-3">
          <UButton block variant="ghost" color="neutral" :loading="loadingMore" @click="loadMore">
            {{ t('market.plugins.load_more') }}
          </UButton>
        </div>
      </div>
    </aside>

    <section class="plugin-market-detail" :aria-label="t('market.plugins.details')">
      <EmptyState
        v-if="!selected"
        inset
        icon="i-tabler-pointer"
        :title="t('market.plugins.select_title')"
        :description="t('market.plugins.select_description')"
      />
      <article v-else class="plugin-market-document">
        <header class="plugin-market-detail-header">
          <span class="plugin-market-detail-icon">
            <UIcon name="i-tabler-plug-connected" class="size-10" />
          </span>
          <div class="min-w-0 flex-1">
            <h2 class="break-words text-2xl font-semibold leading-tight text-highlighted">
              {{ selected.title }}
            </h2>
            <div class="mt-1.5 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted">
              <AccountAvatar
                v-if="selected.creator.picture"
                :picture="selected.creator.picture"
                :name="creator(selected)"
                :user-key="selected.creator.userKey"
              />
              <span class="text-primary">{{ creator(selected) }}</span>
              <UBadge
                v-if="selected.creator.qualityAuthor"
                color="info"
                variant="subtle"
                icon="i-tabler-award"
                size="sm"
              >
                {{ t('market.plugins.quality_author') }}
              </UBadge>
              <span aria-hidden="true">·</span><span>{{ selected.packageVersion }}</span>
            </div>
            <p class="mt-2 max-w-[72ch] whitespace-pre-wrap text-sm leading-6 text-toned">
              {{ selected.summary }}
            </p>
          </div>
          <div class="plugin-market-actions">
            <div class="flex items-center gap-2">
              <UButton
                v-if="!installed || hasUpdate(selected)"
                data-testid="plugin-market-install"
                :icon="installed ? 'i-tabler-refresh' : 'i-tabler-download'"
                :loading="installing"
                :disabled="installing"
                size="sm"
                @click="install"
              >
                {{ t(installed ? 'market.plugins.update' : 'market.plugins.install') }}
              </UButton>
              <UButton
                v-if="installed"
                color="neutral"
                variant="outline"
                icon="i-tabler-settings"
                size="sm"
                :disabled="installing"
                @click="manageInstalled"
              >
                {{ t('market.plugins.manage') }}
              </UButton>
            </div>
            <span v-if="installed" class="text-xs text-muted">
              {{ t('market.plugins.installed_version', { version: installed.version }) }}
            </span>
          </div>
          <p
            v-if="installFailure"
            role="alert"
            class="plugin-market-install-feedback whitespace-pre-wrap text-sm leading-6 text-error"
          >
            {{ installFailure }}
          </p>
          <p
            v-else-if="installNotice"
            role="status"
            class="plugin-market-install-feedback text-sm leading-6 text-warning"
          >
            {{ installNotice }}
          </p>
        </header>

        <div class="plugin-market-tabs" :aria-label="t('market.plugins.details')">
          <button
            type="button"
            :aria-pressed="detailTab === 'overview'"
            @click="detailTab = 'overview'"
          >
            {{ t('market.plugins.overview') }}
          </button>
          <button type="button" :aria-pressed="detailTab === 'nodes'" @click="detailTab = 'nodes'">
            {{ t('market.plugins.nodes') }}
          </button>
          <button
            type="button"
            :aria-pressed="detailTab === 'changes'"
            @click="detailTab = 'changes'"
          >
            {{ t('market.plugins.release_notes') }}
          </button>
        </div>

        <div class="plugin-market-content">
          <div class="plugin-market-main min-w-0">
            <section v-if="detailTab === 'overview'" class="space-y-7">
              <div>
                <h3 class="plugin-market-section-title">{{ t('market.plugins.about_plugin') }}</h3>
                <WorkflowMarketDocument
                  v-if="selected.listing.description"
                  class="mt-3"
                  :content="selected.listing.description"
                />
                <p v-else class="mt-3 text-sm leading-7 text-muted">
                  {{ t('market.plugins.no_description') }}
                </p>
              </div>
              <section v-if="selected.listing.instructions">
                <h3 class="plugin-market-section-title">{{ t('market.plugins.instructions') }}</h3>
                <WorkflowMarketDocument class="mt-3" :content="selected.listing.instructions" />
              </section>
              <section>
                <h3 class="plugin-market-section-title">{{ t('market.plugins.platforms') }}</h3>
                <div class="mt-3 flex flex-wrap gap-2">
                  <UBadge
                    v-for="variant in selected.variants"
                    :key="variant.variantId"
                    color="neutral"
                    variant="outline"
                  >
                    {{ variantLabel(variant) }}
                  </UBadge>
                </div>
              </section>
            </section>

            <section v-else-if="detailTab === 'nodes'" class="space-y-5">
              <h3 class="plugin-market-section-title">{{ t('market.plugins.contained_nodes') }}</h3>
              <p v-if="!selected.nodes.length" class="text-sm text-muted">
                {{ t('market.plugins.no_nodes') }}
              </p>
              <article
                v-for="node in selected.nodes"
                v-else
                :key="node.nodeRef.nodeTypeId"
                class="rounded-lg border border-default p-4"
              >
                <div class="flex flex-wrap items-start justify-between gap-2">
                  <div>
                    <h4 class="text-sm font-semibold text-highlighted">{{ node.name }}</h4>
                    <p class="mt-1 text-sm leading-6 text-toned">{{ node.summary }}</p>
                  </div>
                  <UBadge v-if="node.category" color="neutral" variant="soft">{{
                    node.category
                  }}</UBadge>
                </div>
                <div class="mt-4 grid gap-4 md:grid-cols-2">
                  <div>
                    <p class="text-xs font-medium text-muted">{{ t('market.plugins.inputs') }}</p>
                    <p class="mt-1 text-xs text-toned">{{ portNames(node.inputs) }}</p>
                  </div>
                  <div>
                    <p class="text-xs font-medium text-muted">{{ t('market.plugins.outputs') }}</p>
                    <p class="mt-1 text-xs text-toned">{{ portNames(node.outputs) }}</p>
                  </div>
                </div>
              </article>
            </section>

            <section v-else>
              <h3 class="plugin-market-section-title">{{ selected.packageVersion }}</h3>
              <WorkflowMarketDocument
                class="mt-3"
                :content="selected.releaseNotes || t('market.plugins.no_release_notes')"
              />
            </section>
          </div>

          <aside class="plugin-market-metadata">
            <h3 class="plugin-market-section-title">{{ t('market.plugins.version_info') }}</h3>
            <dl class="mt-4 space-y-4 text-xs">
              <div>
                <dt>{{ t('market.plugins.version') }}</dt>
                <dd>{{ selected.packageVersion }}</dd>
              </div>
              <div>
                <dt>{{ t('market.plugins.package_id') }}</dt>
                <dd class="break-all">{{ selected.packageId }}</dd>
              </div>
              <div v-if="publishedDate">
                <dt>{{ t('market.plugins.published_at') }}</dt>
                <dd>{{ publishedDate }}</dd>
              </div>
              <div>
                <dt>{{ t('market.plugins.creator') }}</dt>
                <dd>{{ creator(selected) }}</dd>
              </div>
              <div v-if="selected.listing.category">
                <dt>{{ t('market.plugins.category') }}</dt>
                <dd>
                  <button
                    type="button"
                    class="plugin-market-link"
                    @click="applyCategory(selected.listing.category)"
                  >
                    {{ selected.listing.category }}
                  </button>
                </dd>
              </div>
              <div v-if="selected.listing.tags.length">
                <dt>{{ t('market.plugins.tags') }}</dt>
                <dd class="mt-1 flex flex-wrap gap-x-2 gap-y-1">
                  <button
                    v-for="value in selected.listing.tags"
                    :key="value"
                    type="button"
                    class="plugin-market-link"
                    @click="applyTag(value)"
                  >
                    #{{ value }}
                  </button>
                </dd>
              </div>
              <div v-if="selected.variants.length">
                <dt>{{ t('market.plugins.download_size') }}</dt>
                <dd>{{ formatBytes(maxDownloadBytes) }}</dd>
              </div>
            </dl>
          </aside>
        </div>
      </article>
    </section>
  </main>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import type {
  NodePackRelease,
  PortProjection,
  RuntimeVariant,
  SearchOptions,
} from '@bindings/github.com/yottaapp/yotta/internal/registryclient/models.js'
import AccountAvatar from '@/components/AccountAvatar.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import WorkflowMarketDocument from '@/components/workflow/WorkflowMarketDocument.vue'
import { isNewerRelease } from '@/app/workflow-library/releaseVersion'
import { errorMessage } from '@/lib/invoke'
import { pluginBackend, type PluginView } from '@/lib/plugins'

const { t, locale } = useI18n()
const router = useRouter()
const query = ref('')
const category = ref('')
const tag = ref('')
const sort = ref('updated')
const qualityOnly = ref(false)
const statusFilter = ref<'all' | 'installed' | 'updates'>('all')
const items = ref<NodePackRelease[]>([])
const installedPlugins = ref<PluginView[]>([])
const facets = ref({ categories: [] as string[], tags: [] as string[] })
const nextCursor = ref('')
const selected = ref<NodePackRelease | null>(null)
const detailTab = ref<'overview' | 'nodes' | 'changes'>('overview')
const loading = ref(false)
const loadingMore = ref(false)
const installing = ref(false)
const failure = ref('')
const installFailure = ref('')
const installNotice = ref('')
let queryGeneration = 0

const statusFilters = computed(() => [
  { value: 'all' as const, label: t('market.plugins.all') },
  { value: 'installed' as const, label: t('market.plugins.installed') },
  { value: 'updates' as const, label: t('market.plugins.update_available') },
])
const hasDiscoveryFilters = computed(() =>
  Boolean(category.value || tag.value || qualityOnly.value || sort.value !== 'updated'),
)
const installedFor = (item: NodePackRelease) =>
  installedPlugins.value.find((plugin) => plugin.id === item.packageId)
const installed = computed(() => (selected.value ? installedFor(selected.value) : undefined))
const hasUpdate = (item: NodePackRelease) => {
  const local = installedFor(item)
  return Boolean(local && isNewerRelease(item.packageVersion, local.version))
}
const visibleItems = computed(() => {
  if (statusFilter.value === 'installed') return items.value.filter(installedFor)
  if (statusFilter.value === 'updates') return items.value.filter((item) => hasUpdate(item))
  return items.value
})
const publishedDate = computed(() => {
  const date = new Date(selected.value?.publishedAt || '')
  return Number.isNaN(date.getTime()) ? '' : date.toLocaleDateString(locale.value)
})
const maxDownloadBytes = computed(() =>
  Math.max(0, ...(selected.value?.variants.map((variant) => variant.downloadBytes) || [])),
)

function creator(item: NodePackRelease): string {
  return item.creator.displayName || item.creator.userKey || t('market.plugins.unnamed_creator')
}
function select(item: NodePackRelease): void {
  selected.value = item
  detailTab.value = 'overview'
  installFailure.value = ''
  installNotice.value = ''
}
function clearSearch(): void {
  query.value = ''
  void search()
}
function applyCategory(value: string): void {
  category.value = value
  void search()
}
function applyTag(value: string): void {
  tag.value = value
  void search()
}
async function search(): Promise<void> {
  await load(false)
}
async function loadMore(): Promise<void> {
  if (!loadingMore.value) await load(true)
}
async function load(append: boolean): Promise<void> {
  const ticket = ++queryGeneration
  if (append) loadingMore.value = true
  else loading.value = true
  failure.value = ''
  try {
    const options: SearchOptions = {
      search: query.value,
      category: category.value,
      tag: tag.value,
      sort: sort.value,
      cursor: append ? nextCursor.value : '',
      limit: 40,
      includeDescendants: true,
      selection: qualityOnly.value ? 'quality-author' : '',
    }
    const [page, local] = await Promise.all([
      pluginBackend.discoverRegistry(options),
      pluginBackend.list(),
    ])
    if (ticket !== queryGeneration) return
    installedPlugins.value = local
    facets.value = {
      categories: page.facets?.categories || [],
      tags: page.facets?.tags || [],
    }
    nextCursor.value = page.nextCursor || ''
    items.value = append
      ? [
          ...items.value,
          ...page.items.filter(
            (item) => !items.value.some((existing) => existing.releaseId === item.releaseId),
          ),
        ]
      : page.items
    if (!append) selected.value = visibleItems.value[0] || null
  } catch (error) {
    if (ticket === queryGeneration) failure.value = errorMessage(error)
  } finally {
    if (ticket === queryGeneration) {
      loading.value = false
      loadingMore.value = false
    }
  }
}
async function install(): Promise<void> {
  const target = selected.value
  if (!target || installing.value || (installedFor(target) && !hasUpdate(target))) return
  installing.value = true
  installFailure.value = ''
  installNotice.value = ''
  try {
    const result = await pluginBackend.installRegistry(target.releaseId)
    installedPlugins.value = await pluginBackend.list()
    if (result.plugin.restartRequired) installNotice.value = t('market.plugins.restart_required')
  } catch (error) {
    installFailure.value = errorMessage(error)
  } finally {
    installing.value = false
  }
}
function manageInstalled(): void {
  void router.push({ path: '/settings', query: { section: 'plugins' } })
}
function portNames(ports: PortProjection[]): string {
  return ports.length
    ? ports.map((port) => port.name || port.id).join(' · ')
    : t('market.plugins.no_ports')
}
function variantLabel(variant: RuntimeVariant): string {
  const platforms = variant.operatingSystems.flatMap((os) =>
    variant.architectures.length ? variant.architectures.map((arch) => `${os}/${arch}`) : [os],
  )
  const runtime = [variant.runtimeFamily, variant.runtimeProfile].filter(Boolean).join(' · ')
  return [...platforms, runtime].filter(Boolean).join(' · ')
}
function formatBytes(value: number): string {
  if (value < 1024) return `${value} B`
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`
  return `${(value / (1024 * 1024)).toFixed(1)} MB`
}

watch(statusFilter, () => {
  if (!visibleItems.value.some((item) => item.releaseId === selected.value?.releaseId)) {
    selected.value = visibleItems.value[0] || null
  }
})
watch(visibleItems, (values) => {
  if (!values.some((item) => item.releaseId === selected.value?.releaseId)) {
    selected.value = values[0] || null
  }
})
onMounted(() => void search())
</script>

<style scoped>
.plugin-market-browser {
  display: grid;
  grid-template-columns: 320px minmax(0, 1fr);
  flex: 1;
  min-height: 0;
  border-top: 1px solid var(--ui-border);
  background: var(--ui-bg);
}
.plugin-market-rail {
  display: flex;
  flex-direction: column;
  min-height: 0;
  border-right: 1px solid var(--ui-border);
  background: var(--ui-bg-muted);
}
.plugin-market-search {
  padding: 16px 14px 12px;
  border-bottom: 1px solid var(--ui-border);
}
.plugin-market-select {
  width: 100%;
  height: 30px;
  border: 1px solid var(--ui-border);
  border-radius: 4px;
  padding: 0 8px;
  background: var(--ui-bg);
  color: var(--ui-text);
  font-size: 12px;
}
.plugin-market-filter {
  padding: 4px 10px;
  border-radius: 4px;
  font-size: 12px;
  color: var(--ui-text-muted);
}
.plugin-market-filter[aria-pressed='true'] {
  background: var(--ui-bg-accented);
  color: var(--ui-text-highlighted);
}
.plugin-market-result {
  display: flex;
  gap: 12px;
  width: 100%;
  height: 72px;
  align-items: center;
  padding: 8px 12px;
  text-align: left;
}
.plugin-market-result:hover {
  background: var(--ui-surface-hover);
}
.plugin-market-result.is-selected {
  background: color-mix(in oklab, var(--ui-primary) 9%, var(--ui-bg-muted));
}
.plugin-market-icon,
.plugin-market-detail-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  border: 1px solid var(--ui-border);
  color: var(--ui-primary);
  background: var(--ui-bg);
}
.plugin-market-icon {
  width: 36px;
  height: 36px;
  border-radius: 8px;
}
.plugin-market-detail-icon {
  width: 64px;
  height: 64px;
  border-radius: 12px;
  background: var(--ui-bg-muted);
}
.plugin-market-result-body {
  display: grid;
  grid-template-rows: repeat(3, 18px);
  gap: 1px;
  flex: 1;
  min-width: 0;
}
.plugin-market-result-heading,
.plugin-market-result-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}
.plugin-market-result-title,
.plugin-market-result-summary,
.plugin-market-result-author {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  line-height: 18px;
}
.plugin-market-result-title {
  flex: 1;
  min-width: 0;
  font-size: 13px;
  font-weight: 600;
  color: var(--ui-text-highlighted);
}
.plugin-market-result-summary {
  font-size: 12px;
  color: var(--ui-text-toned);
}
.plugin-market-result-author {
  flex: 1;
  min-width: 0;
  font-size: 11px;
  color: var(--ui-text-muted);
}
.plugin-market-result-category {
  max-width: 100px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  border: 1px solid var(--ui-border);
  border-radius: 3px;
  padding: 0 4px;
  color: var(--ui-text-toned);
  font-size: 10px;
  line-height: 15px;
}
.plugin-market-detail {
  overflow: hidden;
  min-width: 0;
  min-height: 0;
}
.plugin-market-document {
  display: flex;
  flex-direction: column;
  height: 100%;
  padding: 28px 32px 0;
}
.plugin-market-detail-header {
  display: grid;
  grid-template-columns: 64px minmax(0, 1fr) auto;
  align-items: start;
  gap: 12px 16px;
}
.plugin-market-actions {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 7px;
  padding-top: 2px;
}
.plugin-market-install-feedback {
  grid-column: 2 / -1;
}
.plugin-market-tabs {
  display: flex;
  gap: 24px;
  margin-top: 18px;
  border-bottom: 1px solid var(--ui-border);
}
.plugin-market-tabs button {
  padding: 12px 0;
  font-size: 13px;
  color: var(--ui-text-muted);
  border-bottom: 2px solid transparent;
}
.plugin-market-tabs button[aria-pressed='true'] {
  color: var(--ui-text-highlighted);
  border-bottom-color: var(--ui-primary);
}
.plugin-market-content {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 264px;
  min-height: 0;
  flex: 1;
}
.plugin-market-main {
  width: 100%;
  max-width: 960px;
  justify-self: center;
  overflow-y: auto;
  padding: 28px 40px 40px;
}
.plugin-market-metadata {
  min-height: 0;
  overflow-y: auto;
  border-left: 1px solid var(--ui-border);
  padding: 28px 24px 40px;
}
.plugin-market-metadata dt {
  color: var(--ui-text-muted);
}
.plugin-market-metadata dd {
  margin-top: 3px;
  color: var(--ui-text-toned);
}
.plugin-market-section-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--ui-text-highlighted);
}
.plugin-market-link {
  color: var(--ui-text-toned);
}
.plugin-market-link:hover {
  color: var(--ui-primary);
  text-decoration: underline;
  text-underline-offset: 3px;
}
@media (max-width: 980px) {
  .plugin-market-browser {
    grid-template-columns: 280px minmax(0, 1fr);
  }
  .plugin-market-content {
    grid-template-columns: minmax(0, 1fr);
  }
  .plugin-market-metadata {
    display: none;
  }
  .plugin-market-main {
    padding-inline: 28px;
  }
}
</style>
