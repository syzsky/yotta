<!-- Operate: VS Code-like discovery rail and a left-aligned extension detail page.
     Inherit Yotta tokens. Real versions, authors and install state; no invented ratings. -->
<template>
  <main class="market-browser" data-testid="workflow-market">
    <aside class="market-rail" :aria-label="t('workflow.market.catalog')">
      <div class="market-search">
        <form role="search" @submit.prevent="search">
          <UInput
            v-model="query"
            icon="i-tabler-search"
            :loading="loading"
            size="sm"
            :placeholder="t('workflow.market.search_placeholder')"
            :aria-label="t('workflow.market.search')"
            class="w-full"
          >
            <template #trailing
              ><UButton
                v-if="query"
                icon="i-tabler-x"
                size="xs"
                color="neutral"
                variant="link"
                :aria-label="t('workflow.market.clear_search')"
                @click="clearSearch"
            /></template>
          </UInput>
        </form>
        <div class="mt-2 grid grid-cols-2 gap-2">
          <UFormField :label="t('workflow.market.category')" size="sm"
            ><AdaptiveSelect
              v-model="categorySelection"
              :items="categoryItems"
              data-testid="market-category"
              size="sm"
              width-mode="fill"
          /></UFormField>
          <UFormField :label="t('workflow.market.sort')" size="sm"
            ><AdaptiveSelect
              v-model="sort"
              :items="sortItems"
              data-testid="market-sort"
              size="sm"
              width-mode="fill"
              @update:model-value="search"
          /></UFormField>
        </div>
        <UFormField
          v-if="facets.tags.length"
          :label="t('workflow.market.tags')"
          size="sm"
          class="mt-2"
        >
          <AdaptiveSelect
            v-model="tagSelection"
            :items="tagItems"
            data-testid="market-tag"
            :aria-label="t('workflow.market.tags')"
            size="sm"
            width-mode="fill"
          />
        </UFormField>
        <div class="mt-3 flex items-center gap-1" :aria-label="t('workflow.market.filter')">
          <button
            v-for="option in filters"
            :key="option.value"
            type="button"
            :aria-pressed="filter === option.value"
            class="market-filter"
            @click="filter = option.value"
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
          <UButton color="neutral" variant="outline" size="sm" @click="search">{{
            t('common.retry')
          }}</UButton>
        </div>
        <EmptyState
          v-else-if="!visibleItems.length"
          inset
          icon="i-tabler-world-search"
          :title="t('workflow.market.empty_title')"
          :description="t('workflow.market.empty_description')"
        />
        <button
          v-for="item in visibleItems"
          v-else
          :key="item.releaseId"
          type="button"
          class="market-result"
          :class="{ 'is-selected': selected?.releaseId === item.releaseId }"
          :aria-pressed="selected?.releaseId === item.releaseId"
          @click="select(item)"
        >
          <span class="market-workflow-icon"
            ><WorkflowMarketIcon :name="marketIcon(item)" class="size-6"
          /></span>
          <span class="market-result-body">
            <span class="market-result-heading">
              <strong class="market-result-title" :title="item.title">{{ item.title }}</strong>
              <UIcon
                v-if="installationFor(item)"
                :name="hasUpdate(item) ? 'i-tabler-refresh' : 'i-tabler-circle-check'"
                class="size-3.5 shrink-0 text-primary"
                :aria-label="
                  t(
                    hasUpdate(item)
                      ? 'workflow.market.update_available'
                      : 'workflow.market.installed',
                  )
                "
              />
            </span>
            <span class="market-result-summary" :title="item.summary">{{ item.summary }}</span>
            <span class="market-result-meta">
              <span class="market-result-author" :title="creator(item)">{{ creator(item) }}</span>
              <span
                v-if="item.listing?.category"
                class="market-result-category"
                :title="t('workflow.market.category') + ': ' + item.listing.category"
              >
                <UIcon name="i-tabler-folder" class="size-3 shrink-0" /><span class="truncate">{{
                  item.listing.category
                }}</span>
              </span>
            </span>
          </span>
        </button>
        <div v-if="nextCursor" class="p-3">
          <UButton block variant="ghost" color="neutral" :loading="loadingMore" @click="loadMore">{{
            t('workflow.market.load_more')
          }}</UButton>
        </div>
      </div>
    </aside>
    <section class="market-detail" :aria-label="t('workflow.market.details')">
      <EmptyState
        v-if="!selected"
        inset
        icon="i-tabler-pointer"
        :title="t('workflow.market.select_title')"
        :description="t('workflow.market.select_description')"
      />
      <article v-else class="market-document">
        <header class="market-detail-header">
          <span class="market-detail-icon"
            ><WorkflowMarketIcon :name="marketIcon(selected)" class="size-10"
          /></span>
          <div class="min-w-0 flex-1">
            <h2 class="break-words text-2xl font-semibold leading-tight text-highlighted">
              {{ selected.title }}
            </h2>
            <div
              class="market-author-row mt-1.5 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted"
            >
              <AccountAvatar
                data-testid="market-creator-avatar"
                v-if="selected.creator.picture"
                :picture="selected.creator.picture"
                :name="creator(selected)"
                :user-key="selected.creator.userKey"
              />
              <span class="text-primary">{{ creator(selected) }}</span
              ><span aria-hidden="true">·</span><span>{{ selected.releaseVersion }}</span>
              <span aria-hidden="true">·</span
              ><span
                class="inline-flex items-center gap-1"
                :title="t('workflow.market.download_count_hint')"
                ><UIcon name="i-tabler-download" class="size-3.5" />{{
                  t('workflow.market.download_count', { n: selected.downloadCount ?? 0 })
                }}</span
              >
            </div>
            <p class="mt-2 max-w-[72ch] whitespace-pre-wrap text-sm leading-6 text-toned">
              {{ selected.summary }}
            </p>
          </div>
          <div class="market-header-actions">
            <div class="flex items-center gap-2">
              <UButton
                v-if="!installed || hasUpdate(selected)"
                data-testid="market-install"
                :icon="installed ? 'i-tabler-refresh' : 'i-tabler-download'"
                :loading="installing && installingReleaseId === selected.releaseId"
                :disabled="installing"
                size="sm"
                @click="install"
              >
                {{ t(installed ? 'workflow.market.update' : 'workflow.market.install') }}
              </UButton>
              <UButton
                v-if="installed"
                data-testid="market-open"
                icon="i-tabler-arrow-up-right"
                color="neutral"
                variant="outline"
                :disabled="installing"
                size="sm"
                @click="openInstalled"
                >{{ t('workflow.market.open_installed') }}</UButton
              >
            </div>
            <span v-if="installed" role="status" class="text-xs text-muted">{{
              t('workflow.market.installed_version', { version: installed.releaseVersion })
            }}</span>
          </div>
          <p
            v-if="installFailure"
            role="alert"
            class="market-install-feedback whitespace-pre-wrap text-sm leading-6 text-error"
          >
            {{ installFailure }}
          </p>
        </header>
        <div class="market-content-tabs" :aria-label="t('workflow.market.details')">
          <button
            type="button"
            :aria-pressed="detailTab === 'reviews'"
            @click="detailTab = 'reviews'"
          >
            {{ t('workflow.community.tab')
            }}<span v-if="reviewSummary?.ratingCount" class="ml-1 text-xs text-muted"
              >({{ reviewSummary.average.toFixed(1) }})</span
            >
          </button>
          <button
            type="button"
            :aria-pressed="detailTab === 'overview'"
            @click="detailTab = 'overview'"
          >
            {{ t('workflow.market.overview') }}
          </button>
          <button type="button" :aria-pressed="detailTab === 'history'" @click="showHistory">
            {{ t('workflow.market.history') }}
          </button>
          <button
            type="button"
            :aria-pressed="detailTab === 'changes'"
            @click="detailTab = 'changes'"
          >
            {{ t('workflow.market.release_notes') }}
          </button>
        </div>
        <div class="market-content">
          <div class="market-main-content min-w-0">
            <section v-if="detailTab === 'overview'" class="space-y-7">
              <div>
                <h3 class="market-section-title">{{ t('workflow.market.about_workflow') }}</h3>
                <WorkflowMarketDocument
                  v-if="selected.listing?.description"
                  class="mt-3"
                  :content="selected.listing.description"
                />
                <p v-else class="mt-3 text-sm leading-7 text-muted">
                  {{ t('workflow.market.no_description') }}
                </p>
              </div>
              <section v-if="selected.listing?.instructions">
                <h3 class="market-section-title">{{ t('workflow.market.instructions') }}</h3>
                <WorkflowMarketDocument class="mt-3" :content="selected.listing.instructions" />
              </section>
              <section v-if="selected.examples.length">
                <h3 class="market-section-title">{{ t('workflow.market.examples') }}</h3>
                <div v-for="example in selected.examples" :key="example.title" class="mt-4">
                  <h4 class="text-sm font-medium text-highlighted">{{ example.title }}</h4>
                  <p class="mt-1 text-sm leading-6 text-toned">{{ example.description }}</p>
                </div>
              </section>
              <div v-if="selected.screenshots.length" class="space-y-4">
                <img
                  v-for="shot in selected.screenshots"
                  :key="shot.url"
                  :src="shot.url"
                  :alt="shot.alt"
                  loading="lazy"
                  class="max-w-full rounded-lg border border-default"
                />
              </div>
            </section>
            <section v-else-if="detailTab === 'changes'">
              <h3 class="market-section-title">{{ selected.releaseVersion }}</h3>
              <p class="mt-3 whitespace-pre-wrap text-sm leading-7 text-toned">
                {{ selected.releaseNotes || t('workflow.market.no_release_notes') }}
              </p>
            </section>
            <section v-else-if="detailTab === 'history'" class="space-y-5">
              <USkeleton v-if="historyLoading" class="h-24" />
              <p v-else-if="historyFailure" role="alert" class="text-sm text-error">
                {{ historyFailure }}
              </p>
              <article
                v-for="release in history"
                v-else
                :key="release.releaseId"
                class="border-b border-default pb-5"
              >
                <div class="flex items-center justify-between gap-3">
                  <h3 class="market-section-title">{{ release.releaseVersion }}</h3>
                  <time class="text-xs text-muted">{{
                    new Date(release.publishedAt).toLocaleDateString(locale)
                  }}</time>
                </div>
                <WorkflowMarketDocument
                  class="mt-3"
                  :content="release.releaseNotes || t('workflow.market.no_release_notes')"
                />
              </article>
            </section>
            <WorkflowReviews
              v-show="detailTab === 'reviews'"
              :key="selected.workflowId"
              :workflow-id="selected.workflowId"
              :release-id="installed?.releaseId || selected.releaseId"
              :release-version="installed?.releaseVersion || selected.releaseVersion"
              :author-key="selected.creator.userKey"
              @summary="reviewSummary = $event"
            />
          </div>
          <aside class="market-metadata">
            <section v-if="selected.dependencies?.length" class="mb-5">
              <h3 class="market-section-title">{{ t('workflow.market.dependencies') }}</h3>
              <p
                v-for="pack in selected.dependencies"
                :key="pack.packageId"
                class="mt-2 break-words text-xs text-toned"
              >
                {{ pack.packageId }} · {{ pack.packageVersion }}
              </p>
            </section>
            <h3 class="market-section-title">{{ t('workflow.market.version_info') }}</h3>
            <dl class="mt-4 space-y-4 text-xs">
              <div>
                <dt>{{ t('workflow.market.version') }}</dt>
                <dd>{{ selected.releaseVersion }}</dd>
              </div>
              <div v-if="publishedDate">
                <dt>{{ t('workflow.market.published_at') }}</dt>
                <dd>{{ publishedDate }}</dd>
              </div>
              <div>
                <dt>{{ t('workflow.market.creator') }}</dt>
                <dd>{{ creator(selected) }}</dd>
              </div>
              <div v-if="selected.listing?.category" class="market-taxonomy-group">
                <dt class="market-taxonomy-label">{{ t('workflow.market.category') }}</dt>
                <dd>
                  <button
                    type="button"
                    class="market-category"
                    @click="applyCategory(selected.listing.category)"
                  >
                    <UIcon name="i-tabler-folder" class="size-3.5 shrink-0" /><span>{{
                      selected.listing.category
                    }}</span>
                  </button>
                </dd>
              </div>
              <div v-if="selected.listing?.tags?.length" class="market-taxonomy-group">
                <dt class="market-taxonomy-label">{{ t('workflow.market.tags') }}</dt>
                <dd class="market-tag-list">
                  <button
                    v-for="value in selected.listing.tags"
                    :key="value"
                    type="button"
                    class="market-tag"
                    @click="applyTag(value)"
                  >
                    <span class="market-tag-hash" aria-hidden="true">#</span
                    ><span class="market-tag-text">{{ value }}</span>
                  </button>
                </dd>
              </div>
              <div v-if="selected.facts?.verified">
                <dt>{{ t('workflow.market.requirements') }}</dt>
                <dd>
                  {{ t('workflow.market.target_count', { n: selected.facts.targetProfileCount }) }}
                </dd>
                <dd>
                  {{ t('workflow.market.dependency_count', { n: selected.facts.dependencyCount }) }}
                </dd>
                <dd v-if="selected.facts.credentialCount">
                  {{ t('workflow.market.credential_count', { n: selected.facts.credentialCount }) }}
                </dd>
              </div>
              <div v-if="selected.facts?.verified">
                <dt>{{ t('workflow.market.resources') }}</dt>
                <dd>
                  {{ t('workflow.market.resource_count', { n: selected.facts.resourceCount }) }}
                </dd>
              </div>
            </dl>
          </aside>
        </div>
      </article>
    </section>
  </main>
</template>

<script setup lang="ts">
import { isNewerRelease } from '@/app/workflow-library/releaseVersion'
import WorkflowMarketIcon from './WorkflowMarketIcon.vue'
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { workflowTransport, type RegistryWorkflowReleaseView } from '@/app/transport/workflow'
import { shopTransport } from '@/app/transport/shop'
import { errorMessage } from '@/lib/invoke'
import EmptyState from '@/components/common/EmptyState.vue'
import WorkflowMarketDocument from './WorkflowMarketDocument.vue'
import AccountAvatar from '@/components/AccountAvatar.vue'
import AdaptiveSelect from '@/components/common/AdaptiveSelect.vue'
import WorkflowReviews from './WorkflowReviews.vue'
import type { Summary as ReviewSummary } from '@bindings/github.com/yottaapp/yotta/internal/communityclient/models.js'

const { t, locale } = useI18n()
const router = useRouter()
const reviewSummary = ref<ReviewSummary | null>(null)
const categoryItems = computed(() => [
  { label: t('workflow.market.all_categories'), value: 'all' },
  ...facets.value.categories.map((value) => ({ label: value, value: 'v:' + value })),
])
const tagItems = computed(() => [
  { label: t('workflow.market.all_tags'), value: 'all' },
  ...facets.value.tags.map((value) => ({ label: value, value: 'v:' + value })),
])
const sortItems = computed(() => [
  { label: t('workflow.market.sort_updated'), value: 'updated' },
  { label: t('workflow.market.sort_name'), value: 'name' },
])
const categorySelection = computed({
  get: () => (category.value ? 'v:' + category.value : 'all'),
  set: (value: string) => {
    category.value = value === 'all' ? '' : value.slice(2)
    void search()
  },
})
const tagSelection = computed({
  get: () => (tag.value ? 'v:' + tag.value : 'all'),
  set: (value: string) => {
    tag.value = value === 'all' ? '' : value.slice(2)
    void search()
  },
})
function clearSearch() {
  query.value = ''
  void search()
}
const query = ref(''),
  category = ref(''),
  tag = ref(''),
  sort = ref('updated'),
  nextCursor = ref('')
const emit = defineEmits<{ installed: [] }>()
const installingReleaseId = ref('')
const lastInstallIssue = ref({ releaseId: '', message: '' })
const items = ref<RegistryWorkflowReleaseView[]>([])
const selected = ref<RegistryWorkflowReleaseView | null>(null)
const loading = ref(false),
  loadingMore = ref(false),
  installing = ref(false)
const failure = ref(''),
  installFailure = ref(''),
  filter = ref('all'),
  detailTab = ref('overview')
const facets = ref({ categories: [] as string[], tags: [] as string[] })
const history = ref<RegistryWorkflowReleaseView[]>([]),
  historyLoading = ref(false),
  historyFailure = ref('')
const installations = ref<Awaited<ReturnType<typeof shopTransport.installations>>>([])
let queryGeneration = 0
const filters = computed(() => [
  { value: 'all', label: t('workflow.market.all') },
  { value: 'installed', label: t('workflow.market.installed') },
  { value: 'updates', label: t('workflow.market.update_available') },
])
const installationFor = (item: RegistryWorkflowReleaseView) =>
  installations.value.find((entry) => entry.workflowId === item.workflowId)
const installed = computed(() => (selected.value ? installationFor(selected.value) : undefined))
const hasUpdate = (item: RegistryWorkflowReleaseView) => {
  const local = installationFor(item)
  return Boolean(local && isNewerRelease(item.releaseVersion, local.releaseVersion))
}
const visibleItems = computed(() =>
  filter.value === 'installed'
    ? items.value.filter(installationFor)
    : filter.value === 'updates'
      ? items.value.filter((item) => hasUpdate(item))
      : items.value,
)
const creator = (item: RegistryWorkflowReleaseView) =>
  item.creator.displayName || t('workflow.market.unnamed_creator')
const marketIcon = (item: RegistryWorkflowReleaseView) =>
  /^i-tabler-[a-z0-9-]+$/.test(item.listing?.icon || '') ? item.listing.icon! : 'i-tabler-route'
const publishedDate = computed(() => {
  const date = new Date(selected.value?.publishedAt || '')
  return Number.isNaN(date.getTime()) ? '' : date.toLocaleDateString(locale.value)
})
function select(item: RegistryWorkflowReleaseView) {
  if (item.workflowId !== selected.value?.workflowId) reviewSummary.value = null
  selected.value = item
  detailTab.value = 'overview'
  installFailure.value =
    lastInstallIssue.value.releaseId === item.releaseId ? lastInstallIssue.value.message : ''
  history.value = []
  void refreshCreator()
}
watch(visibleItems, (values) => {
  if (!values.some((item) => item.releaseId === selected.value?.releaseId)) {
    if (values[0]) select(values[0])
    else {
      selected.value = null
      history.value = []
      detailTab.value = 'overview'
      installFailure.value = ''
    }
  }
})
async function search() {
  await load(false)
}
function applyCategory(value?: string) {
  category.value = value || ''
  void search()
}
function applyTag(value: string) {
  tag.value = value
  void search()
}
async function loadMore() {
  if (!loadingMore.value) await load(true)
}
async function load(append: boolean) {
  const ticket = ++queryGeneration
  if (append) loadingMore.value = true
  else loading.value = true
  failure.value = ''
  try {
    const local = await shopTransport.installations()
    if (filter.value !== 'all' && !local.length) {
      items.value = []
      nextCursor.value = ''
      return
    }
    const page = await shopTransport.discover({
      search: query.value,
      category: category.value,
      tag: tag.value,
      sort: sort.value,
      cursor: append ? nextCursor.value : '',
      limit: 40,
      workflowIds: filter.value === 'all' ? undefined : local.map((item) => item.workflowId),
    })
    if (ticket !== queryGeneration) return
    installations.value = local
    facets.value = { categories: page.facets?.categories || [], tags: page.facets?.tags || [] }
    nextCursor.value = page.nextCursor || ''
    items.value = append
      ? [
          ...items.value,
          ...page.items.filter(
            (item) => !items.value.some((old) => old.releaseId === item.releaseId),
          ),
        ]
      : page.items
    if (!append) {
      selected.value = visibleItems.value[0] ?? null
      detailTab.value = 'overview'
      installFailure.value = ''
    }
  } catch (error) {
    if (ticket === queryGeneration) failure.value = errorMessage(error)
  } finally {
    if (ticket === queryGeneration) {
      loading.value = false
      loadingMore.value = false
    }
  }
}
async function showHistory() {
  detailTab.value = 'history'
  const id = selected.value?.workflowId
  if (!id) return
  history.value = []
  historyLoading.value = true
  historyFailure.value = ''
  try {
    const result = await shopTransport.history(id)
    if (selected.value?.workflowId === id) history.value = result
  } catch (error) {
    if (selected.value?.workflowId === id) historyFailure.value = errorMessage(error)
  } finally {
    if (selected.value?.workflowId === id) historyLoading.value = false
  }
}
async function openInstalled() {
  const local = installed.value
  if (local && !installing.value) {
    try {
      await router.push('/workflows/' + local.workflowId + '/edit')
    } catch (error) {
      installFailure.value = errorMessage(error)
    }
  }
}
async function install() {
  const target = selected.value
  if (!target || installing.value || (installed.value && !hasUpdate(target))) return
  installing.value = true
  installingReleaseId.value = target.releaseId
  installFailure.value = ''
  lastInstallIssue.value = { releaseId: '', message: '' }
  let completed = false
  try {
    await workflowTransport.installRegistryWorkflow(target.releaseId)
    completed = true
    emit('installed')
    const current = await shopTransport.installations()
    if (!marketDisposed) installations.value = current
  } catch (error) {
    const message =
      (completed ? t('workflow.market.installed_refresh_failed') + '\n' : '') + errorMessage(error)
    lastInstallIssue.value = { releaseId: target.releaseId, message }
    if (selected.value?.releaseId === target.releaseId) installFailure.value = message
  } finally {
    installing.value = false
    installingReleaseId.value = ''
  }
}
let creatorRefreshing = false,
  marketDisposed = false
async function refreshCreator() {
  const target = selected.value
  if (!target || loading.value || creatorRefreshing || marketDisposed) return
  const generation = queryGeneration
  creatorRefreshing = true
  try {
    const page = await shopTransport.discover({
      workflowIds: [target.workflowId],
      search: '',
      category: '',
      tag: '',
      sort: 'updated',
      cursor: '',
      limit: 1,
    })
    if (marketDisposed || generation !== queryGeneration) return
    const current = page.items[0]?.creator
    if (!current || current.userKey !== target.creator.userKey) return
    for (const item of [...items.value, ...history.value])
      if (item.creator.userKey === current.userKey) item.creator = { ...current }
    if (selected.value?.creator.userKey === current.userKey) selected.value.creator = { ...current }
  } catch {
    /* Keep the last known public profile; browsing remains usable. */
  } finally {
    creatorRefreshing = false
  }
}
onMounted(() => {
  void search()
  window.addEventListener('focus', refreshCreator)
})
onUnmounted(() => {
  marketDisposed = true
  queryGeneration++
  window.removeEventListener('focus', refreshCreator)
})
watch(filter, () => void search())
</script>

<style scoped>
.market-select {
  width: 100%;
  height: 30px;
  border: 1px solid var(--ui-border);
  border-radius: 4px;
  padding: 0 8px;
  background: var(--ui-bg);
  color: var(--ui-text);
  font-size: 12px;
}
.market-category {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 0;
  color: var(--ui-text);
  font-size: 12px;
  line-height: 20px;
  text-align: left;
}
.market-tag-list {
  display: flex;
  flex-wrap: wrap;
  gap: 3px 10px;
}
.market-tag {
  display: inline-flex;
  align-items: baseline;
  gap: 1px;
  padding: 0;
  font-size: 12px;
  line-height: 20px;
  color: var(--ui-text-toned);
  text-align: left;
}
.market-tag-hash {
  font-size: 10px;
  line-height: inherit;
  color: var(--ui-text-muted);
}
.market-tag-text {
  overflow-wrap: anywhere;
  min-width: 0;
}
.market-category:hover,
.market-tag:hover {
  color: var(--ui-primary);
  text-decoration: underline;
  text-underline-offset: 3px;
}
.market-header-actions {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 7px;
  padding-top: 2px;
  position: sticky;
  top: 0;
}
.market-install-feedback {
  grid-column: 2 / -1;
}
.market-author-row :deep(.account-avatar) {
  width: 20px;
  height: 20px;
  font-size: 10px;
}
.market-browser {
  display: grid;
  grid-template-columns: 320px minmax(0, 1fr);
  flex: 1;
  min-height: 0;
  border-top: 1px solid var(--ui-border);
  background: var(--ui-bg);
}
.market-rail {
  display: flex;
  flex-direction: column;
  min-height: 0;
  border-right: 1px solid var(--ui-border);
  background: var(--ui-bg-muted);
}
.market-search {
  padding: 16px 14px 12px;
  border-bottom: 1px solid var(--ui-border);
}
.market-filter {
  padding: 4px 10px;
  border-radius: 4px;
  font-size: 12px;
  color: var(--ui-text-muted);
}
.market-filter[aria-pressed='true'] {
  background: var(--ui-bg-accented);
  color: var(--ui-text-highlighted);
}
.market-result {
  display: flex;
  gap: 12px;
  width: 100%;
  height: 72px;
  align-items: center;
  padding: 8px 12px;
  text-align: left;
}
.market-result-body {
  display: grid;
  grid-template-rows: repeat(3, 18px);
  gap: 1px;
  flex: 1;
  min-width: 0;
}
.market-result-heading {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}
.market-result-title {
  flex: 1;
  min-width: 0;
  font-size: 13px;
  font-weight: 600;
  color: var(--ui-text-highlighted);
}
.market-result-title,
.market-result-summary,
.market-result-author {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  line-height: 18px;
}
.market-result-summary {
  font-size: 12px;
  color: var(--ui-text-toned);
}
.market-result-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.market-result-author {
  flex: 1;
  min-width: 0;
  font-size: 11px;
  color: var(--ui-text-muted);
}
.market-result-category {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  flex-shrink: 0;
  max-width: 100px;
  padding: 0 4px;
  border: 1px solid var(--ui-border);
  border-radius: 3px;
  background: var(--ui-bg-elevated);
  color: var(--ui-text-toned);
  font-size: 10px;
  line-height: 15px;
}
.market-result:hover {
  background: var(--ui-surface-hover);
}
.market-result.is-selected {
  background: color-mix(in oklab, var(--ui-primary) 9%, var(--ui-bg-muted));
}
.market-workflow-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  flex-shrink: 0;
  color: var(--ui-primary);
  background: var(--ui-bg);
  border: 1px solid var(--ui-border);
  border-radius: 8px;
}
.market-detail {
  overflow: hidden;
  min-height: 0;
  min-width: 0;
}
.market-document {
  display: flex;
  flex-direction: column;
  height: 100%;
  padding: 28px 32px 0;
}
.market-detail-header {
  display: grid;
  grid-template-columns: 64px minmax(0, 1fr) auto;
  align-items: start;
  gap: 12px 16px;
}
.market-detail-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 64px;
  height: 64px;
  flex-shrink: 0;
  border: 1px solid var(--ui-border);
  border-radius: 12px;
  color: var(--ui-primary);
  background: var(--ui-bg-muted);
}
.market-content-tabs {
  display: flex;
  gap: 24px;
  margin-top: 18px;
  border-bottom: 1px solid var(--ui-border);
}
.market-content-tabs button {
  padding: 12px 0;
  font-size: 13px;
  color: var(--ui-text-muted);
  border-bottom: 2px solid transparent;
}
.market-content-tabs button[aria-pressed='true'] {
  color: var(--ui-text-highlighted);
  border-bottom-color: var(--ui-primary);
}
.market-content {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 264px;
  min-height: 0;
  flex: 1;
}
.market-main-content {
  width: 100%;
  max-width: 960px;
  justify-self: center;
  overflow-y: auto;
  padding: 28px 40px 40px;
}
.market-detail-header,
.market-content-tabs {
  flex-shrink: 0;
}
.market-detail-header {
  max-height: 40%;
  overflow-y: auto;
}
.market-section-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--ui-text-highlighted);
}
.market-metadata {
  border-left: 1px solid var(--ui-border);
  padding: 28px 24px 40px;
  min-height: 0;
  overflow-y: auto;
}
.market-metadata dt {
  color: var(--ui-text-muted);
}
.market-metadata dd {
  margin-top: 5px;
  overflow-wrap: anywhere;
  color: var(--ui-text);
}
button:focus-visible {
  outline: 2px solid var(--ui-primary);
  outline-offset: -2px;
}
@media (max-width: 1100px) {
  .market-browser {
    grid-template-columns: 280px minmax(0, 1fr);
  }
  .market-document {
    height: auto;
    min-height: 100%;
    padding: 28px 24px;
  }
  .market-detail {
    overflow-y: auto;
  }
  .market-detail-header {
    max-height: none;
    overflow: visible;
  }
  .market-main-content {
    overflow: visible;
    padding: 20px 0 0;
  }
  .market-content {
    overflow: visible;
    flex: none;
    grid-template-columns: 1fr;
    gap: 28px;
  }
  .market-metadata {
    border-left: 0;
    border-top: 1px solid var(--ui-border);
    padding: 20px 0 0;
  }
  .market-metadata dl {
    display: flex;
    flex-wrap: wrap;
    gap: 24px;
  }
}
@media (max-width: 700px) {
  .market-browser {
    grid-template-columns: 1fr;
    grid-template-rows: 280px minmax(500px, auto);
    overflow-y: auto;
  }
  .market-rail {
    max-height: 300px;
    border-right: 0;
    border-bottom: 1px solid var(--ui-border);
  }
  .market-detail {
    overflow: visible;
  }
  .market-document {
    height: auto;
    padding: 24px 20px;
  }
  .market-detail-header {
    grid-template-columns: 48px minmax(0, 1fr) auto;
    gap: 12px;
  }
  .market-detail-icon {
    width: 48px;
    height: 48px;
    border-radius: 8px;
  }
  .market-detail-icon :deep(svg) {
    width: 28px;
    height: 28px;
  }
}
</style>
