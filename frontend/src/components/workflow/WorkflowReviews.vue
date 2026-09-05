<template>
  <section data-testid="workflow-reviews" class="space-y-7">
    <USkeleton v-if="!summaryLoaded && loading" class="h-32" />
    <div
      v-if="summaryLoaded"
      class="grid grid-cols-[140px_1fr] items-center gap-7 border-b border-default pb-6"
    >
      <div>
        <p class="text-4xl font-semibold tabular-nums text-highlighted">
          {{ summary.ratingCount ? summary.average.toFixed(1) : '—' }}
        </p>
        <div
          class="mt-2 flex gap-0.5"
          :aria-label="t('workflow.community.average', { score: summary.average.toFixed(1) })"
        >
          <UIcon
            v-for="star in 5"
            :key="star"
            :name="star <= Math.round(summary.average) ? 'i-tabler-star-filled' : 'i-tabler-star'"
            class="size-4 text-warning"
          />
        </div>
        <p class="mt-2 text-xs text-muted">
          {{ t('workflow.community.rating_count', { n: summary.ratingCount }) }}
        </p>
      </div>
      <div class="space-y-2">
        <div
          v-for="star in [5, 4, 3, 2, 1]"
          :key="star"
          class="flex items-center gap-2 text-xs text-muted"
        >
          <span class="w-7">{{ star }} ★</span>
          <div class="h-1.5 min-w-0 flex-1 overflow-hidden rounded-full bg-elevated">
            <div
              class="h-full bg-warning"
              :style="{
                width: summary.ratingCount
                  ? (summary.distribution[star - 1] / summary.ratingCount) * 100 + '%'
                  : '0%',
              }"
            />
          </div>
          <span class="w-6 text-right tabular-nums">{{ summary.distribution[star - 1] }}</span>
        </div>
      </div>
    </div>

    <div class="space-y-3 rounded-lg border border-default p-4">
      <div class="flex items-center justify-between gap-3">
        <h3 class="text-sm font-semibold text-highlighted">
          {{ mine ? t('workflow.community.edit_mine') : t('workflow.community.write') }}
        </h3>
        <span class="text-xs text-muted">{{
          t('workflow.community.version', { version: releaseVersion })
        }}</span>
      </div>
      <p class="text-xs leading-5 text-muted">
        {{ ownWork ? t('workflow.community.author_hint') : t('workflow.community.one_review') }}
      </p>
      <div
        v-if="!ownWork"
        class="flex items-center gap-1"
        role="group"
        :aria-label="t('workflow.community.choose_stars')"
      >
        <button
          v-for="star in 5"
          :key="star"
          type="button"
          :aria-label="t('workflow.community.stars', { n: star })"
          :aria-pressed="stars === star"
          :disabled="busy"
          class="flex size-9 items-center justify-center rounded-md text-warning hover:bg-elevated focus-visible:outline-2 focus-visible:outline-primary"
          @click="setStars(star)"
        >
          <UIcon :name="star <= stars ? 'i-tabler-star-filled' : 'i-tabler-star'" class="size-6" />
        </button>
        <UButton
          v-if="stars"
          color="neutral"
          variant="link"
          size="xs"
          :disabled="busy"
          @click="setStars(0)"
          >{{ t('workflow.community.clear_stars') }}</UButton
        >
      </div>
      <UTextarea
        v-model="content"
        data-testid="review-content"
        :aria-label="t('workflow.community.content_label')"
        :placeholder="t('workflow.community.content_hint')"
        :rows="4"
        maxlength="4000"
        :disabled="busy"
        class="w-full"
        @update:model-value="dirty = true"
      />
      <p v-if="submitFailure" role="alert" class="whitespace-pre-wrap text-sm text-error">
        {{ submitFailure }}
      </p>
      <p v-if="notice" role="status" class="text-sm text-primary">{{ notice }}</p>
      <div class="flex items-center justify-between gap-3">
        <span class="text-xs text-muted">{{ content.length }} / 4000</span>
        <div class="flex gap-2">
          <UButton
            v-if="mine"
            color="neutral"
            variant="ghost"
            :disabled="busy"
            @click="removeMine"
            >{{ t('workflow.community.delete_mine') }}</UButton
          ><UButton
            v-if="viewer.user_key"
            data-testid="review-submit"
            :loading="busy"
            :disabled="(!content.trim() && (!stars || ownWork)) || mineLoading"
            @click="submit"
            >{{ mine ? t('workflow.community.save') : t('workflow.community.submit') }}</UButton
          ><UButton v-else :loading="busy" @click="login">{{
            t('workflow.community.login')
          }}</UButton>
        </div>
      </div>
    </div>

    <div class="flex items-center justify-between">
      <h3 class="text-sm font-semibold text-highlighted">
        {{
          summaryLoaded
            ? t('workflow.community.discussions', { n: summary.discussionCount })
            : t('workflow.community.tab')
        }}
      </h3>
      <UButton
        color="neutral"
        variant="ghost"
        icon="i-tabler-refresh"
        size="xs"
        :loading="loading"
        :aria-label="t('common.refresh')"
        @click="load(false)"
      />
    </div>
    <USkeleton v-if="loading" class="h-24 rounded-lg" />
    <div v-else-if="failure" role="alert" class="space-y-2 text-sm text-error">
      <p class="whitespace-pre-wrap">{{ failure }}</p>
      <UButton color="neutral" variant="outline" @click="load(false)">{{
        t('common.retry')
      }}</UButton>
    </div>
    <p v-else-if="!items.length" class="py-4 text-sm text-muted">
      {{ t('workflow.community.empty') }}
    </p>
    <div v-else class="divide-y divide-default">
      <article
        v-for="review in items"
        :key="review.id"
        class="py-5 first:pt-0"
        data-testid="workflow-review"
      >
        <header class="flex items-start gap-3">
          <AccountAvatar
            :name="review.author.name"
            :user-key="review.author.userKey"
            :picture="review.author.picture"
          />
          <div class="min-w-0 flex-1">
            <div class="flex flex-wrap items-center justify-between gap-2">
              <strong class="text-sm text-highlighted">{{
                review.author.name || t('workflow.community.member')
              }}</strong
              ><time class="text-xs text-muted">{{ formatDate(review.createdAt) }}</time>
            </div>
            <div class="mt-1 flex items-center gap-2">
              <span
                v-if="review.stars"
                class="flex text-warning"
                :aria-label="t('workflow.community.stars', { n: review.stars })"
                ><UIcon
                  v-for="star in 5"
                  :key="star"
                  :name="star <= review.stars ? 'i-tabler-star-filled' : 'i-tabler-star'"
                  class="size-3" /></span
              ><span class="text-xs text-muted">{{ review.releaseVersion }}</span>
            </div>
          </div>
        </header>
        <p class="mt-3 whitespace-pre-wrap break-words text-sm leading-6 text-toned">
          {{ review.content || t('workflow.community.rating_only') }}
        </p>
        <UButton
          size="xs"
          variant="link"
          color="neutral"
          :disabled="busy"
          @click="openReply(review.id)"
          >{{ t('workflow.community.reply') }}</UButton
        >
        <div v-if="review.replies.length" class="ml-4 mt-3 space-y-4 border-l border-default pl-4">
          <div v-for="reply in review.replies" :key="reply.id">
            <div class="flex items-center justify-between gap-2">
              <span class="text-xs font-medium text-highlighted">{{
                reply.author.name || t('workflow.community.member')
              }}</span>
              <div class="flex items-center gap-2">
                <time class="text-xs text-muted">{{ formatDate(reply.createdAt) }}</time
                ><UButton
                  v-if="reply.author.userKey === viewer.user_key"
                  icon="i-tabler-trash"
                  size="xs"
                  color="neutral"
                  variant="ghost"
                  :aria-label="t('common.delete')"
                  :disabled="busy"
                  @click="removeReply(reply.id)"
                />
              </div>
            </div>
            <p class="mt-1 whitespace-pre-wrap break-words text-sm leading-6 text-toned">
              {{ reply.content }}
            </p>
          </div>
        </div>
        <p
          v-if="repliesFailure[review.id]"
          role="alert"
          class="whitespace-pre-wrap text-sm text-error"
        >
          {{ repliesFailure[review.id] }}
        </p>
        <UButton
          v-if="review.repliesCursor"
          color="neutral"
          variant="link"
          size="xs"
          :loading="repliesLoading === review.id"
          @click="loadReplies(review)"
          >{{ t('workflow.community.more_replies') }}</UButton
        >
        <div v-if="replyTarget === review.id" class="mt-3 space-y-2">
          <UTextarea
            v-model="replyContent"
            data-testid="review-reply-content"
            :aria-label="t('workflow.community.reply')"
            :rows="2"
            maxlength="4000"
            :disabled="busy"
            class="w-full"
          />
          <p v-if="replyFailure" role="alert" class="whitespace-pre-wrap text-sm text-error">
            {{ replyFailure }}
          </p>
          <div class="flex justify-end gap-2">
            <UButton color="neutral" variant="ghost" :disabled="busy" @click="replyTarget = ''">{{
              t('common.cancel')
            }}</UButton
            ><UButton
              data-testid="review-reply-submit"
              :loading="busy"
              :disabled="!replyContent.trim()"
              @click="submitReply"
              >{{ t('workflow.community.reply') }}</UButton
            >
          </div>
        </div>
      </article>
    </div>
    <UButton
      v-if="nextCursor"
      color="neutral"
      variant="outline"
      block
      :loading="loadingMore"
      @click="load(true)"
      >{{ t('workflow.market.load_more') }}</UButton
    >
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { communityTransport, shopTransport } from '@/app/transport/workflow'
import type {
  Review,
  Summary,
} from '@bindings/github.com/yottaapp/yotta/internal/communityclient/models.js'
import { errorMessage } from '@/lib/invoke'
import { useConfirm } from '@/composables/useConfirm'
import AccountAvatar from '@/components/AccountAvatar.vue'
const props = defineProps<{
  workflowId: string
  releaseId: string
  releaseVersion: string
  authorKey: string
}>()
const emit = defineEmits<{ summary: [value: Summary] }>()
const { t, locale } = useI18n(),
  { confirm } = useConfirm()
const summary = ref<Summary>({
  average: 0,
  ratingCount: 0,
  discussionCount: 0,
  distribution: [0, 0, 0, 0, 0],
})
const items = ref<Review[]>([]),
  mine = ref<Review | null>(null)
const summaryLoaded = ref(false),
  repliesLoading = ref(''),
  repliesFailure = ref<Record<string, string>>({})
const viewer = ref({ user_key: '', name: '', picture: '', signingIn: false })
const stars = ref(0),
  content = ref(''),
  dirty = ref(false),
  busy = ref(false),
  mineLoading = ref(false)
const loading = ref(false),
  loadingMore = ref(false),
  failure = ref(''),
  submitFailure = ref(''),
  notice = ref(''),
  nextCursor = ref('')
const replyTarget = ref(''),
  replyContent = ref(''),
  replyFailure = ref('')
const ownWork = computed(
  () => Boolean(viewer.value.user_key) && viewer.value.user_key === props.authorKey,
)
let timer: ReturnType<typeof setInterval> | undefined,
  disposed = false,
  refreshing = false,
  generation = 0
function setStars(value: number) {
  stars.value = value
  dirty.value = true
}
const formatDate = (value: string) => new Date(value).toLocaleDateString(locale.value)
async function load(append = false) {
  const ticket = ++generation
  if (append) loadingMore.value = true
  else loading.value = true
  failure.value = ''
  try {
    const page = await communityTransport.list(props.workflowId, append ? nextCursor.value : '')
    if (disposed || ticket !== generation) return
    items.value = append
      ? [
          ...items.value,
          ...page.items.filter((item) => !items.value.some((old) => old.id === item.id)),
        ]
      : page.items
    summary.value = page.summary
    summaryLoaded.value = true
    nextCursor.value = page.nextCursor || ''
    emit('summary', page.summary)
  } catch (error) {
    if (!disposed && ticket === generation) failure.value = errorMessage(error)
  } finally {
    if (ticket === generation) {
      loading.value = false
      loadingMore.value = false
    }
  }
}
async function loadMine() {
  mineLoading.value = true
  try {
    const review = await communityTransport.mine(props.workflowId)
    if (disposed) return
    mine.value = review
    if (!dirty.value) {
      stars.value = review?.stars || 0
      content.value = review?.content || ''
    }
  } catch (error) {
    submitFailure.value = errorMessage(error)
  } finally {
    mineLoading.value = false
  }
}
async function refreshViewer() {
  if (refreshing) return
  refreshing = true
  try {
    const profile = await shopTransport.account()
    if (disposed) return
    const old = viewer.value.user_key
    viewer.value = profile
    if (old !== profile.user_key) {
      if (old) {
        mine.value = null
        stars.value = 0
        content.value = ''
        dirty.value = false
      }
      if (profile.user_key) await loadMine()
    }
  } catch {
    /* Account login owns errors. */
  } finally {
    refreshing = false
  }
}
async function login() {
  busy.value = true
  submitFailure.value = ''
  try {
    await shopTransport.login()
    await refreshViewer()
  } catch (error) {
    submitFailure.value = errorMessage(error)
  } finally {
    busy.value = false
  }
}
async function submit() {
  if (busy.value) return
  busy.value = true
  notice.value = ''
  submitFailure.value = ''
  try {
    mine.value = await communityTransport.save({
      workflowId: props.workflowId,
      releaseId: props.releaseId,
      stars: ownWork.value ? 0 : stars.value,
      content: content.value,
    })
    dirty.value = false
    notice.value = t('workflow.community.saved')
    await load(false)
  } catch (error) {
    submitFailure.value = errorMessage(error)
  } finally {
    busy.value = false
  }
}
async function removeMine() {
  if (busy.value) return
  if (
    !(await confirm({
      title: t('workflow.community.delete_mine'),
      description: t('workflow.community.delete_description'),
      color: 'error',
    }))
  )
    return
  busy.value = true
  notice.value = ''
  submitFailure.value = ''
  try {
    await communityTransport.remove(props.workflowId)
    mine.value = null
    stars.value = 0
    content.value = ''
    dirty.value = false
    await load(false)
  } catch (error) {
    submitFailure.value = errorMessage(error)
  } finally {
    busy.value = false
  }
}
async function openReply(id: string) {
  if (!viewer.value.user_key) {
    await login()
    if (!viewer.value.user_key) return
  }
  replyTarget.value = id
  replyContent.value = ''
  replyFailure.value = ''
}
async function submitReply() {
  if (busy.value) return
  busy.value = true
  replyFailure.value = ''
  try {
    await communityTransport.reply(props.workflowId, replyTarget.value, replyContent.value)
    replyTarget.value = ''
    replyContent.value = ''
    await load(false)
  } catch (error) {
    replyFailure.value = errorMessage(error)
  } finally {
    busy.value = false
  }
}
async function removeReply(id: string) {
  if (busy.value) return
  if (!(await confirm({ title: t('workflow.community.delete_reply'), color: 'error' }))) return
  busy.value = true
  try {
    await communityTransport.removeReply(props.workflowId, id)
    await load(false)
  } catch (error) {
    failure.value = errorMessage(error)
  } finally {
    busy.value = false
  }
}
async function loadReplies(review: Review) {
  if (repliesLoading.value) return
  repliesLoading.value = review.id
  repliesFailure.value[review.id] = ''
  try {
    const page = await communityTransport.replies(
      props.workflowId,
      review.id,
      review.repliesCursor || '',
    )
    if (disposed || !items.value.includes(review)) return
    review.replies.push(
      ...page.items.filter((reply) => !review.replies.some((old) => old.id === reply.id)),
    )
    review.repliesCursor = page.nextCursor || ''
  } catch (error) {
    repliesFailure.value[review.id] = errorMessage(error)
  } finally {
    repliesLoading.value = ''
  }
}
onMounted(() => {
  void load()
  void refreshViewer()
  timer = setInterval(() => void refreshViewer(), 2000)
})
onUnmounted(() => {
  disposed = true
  clearInterval(timer)
})
</script>
