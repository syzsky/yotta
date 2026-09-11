<script setup lang="ts">
import { reactive, ref } from 'vue'
import BaseModal from '@/components/common/BaseModal.vue'
import { useI18n } from 'vue-i18n'
import { communityTransport } from '@/app/transport/workflow'
import { errorMessage } from '@/lib/invoke'
import type { Report } from '@bindings/github.com/yottaapp/yotta/internal/communityclient/models.js'
const props = defineProps<{ workflowId: string; releaseId: string }>()
const { t } = useI18n()
const open = ref(false),
  busy = ref(false),
  failure = ref(''),
  reports = ref<Report[]>([])
const draft = reactive({ id: '', reason: 'outdated', description: '', originalWork: '' })
async function begin() {
  draft.id = crypto.randomUUID()
  draft.description = ''
  draft.originalWork = ''
  failure.value = ''
  reports.value = []
  open.value = true
  try {
    reports.value = (await communityTransport.reports(props.workflowId)).items.filter(
      (v) => v.workflowId === props.workflowId,
    )
  } catch (e) {
    failure.value = errorMessage(e)
  }
}
async function submit() {
  busy.value = true
  failure.value = ''
  try {
    const result = await communityTransport.submitReport({
      ...draft,
      workflowId: props.workflowId,
      releaseId: props.releaseId,
    })
    reports.value = [result, ...reports.value.filter((v) => v.id !== result.id)]
    draft.id = crypto.randomUUID()
    draft.description = ''
    draft.originalWork = ''
  } catch (e) {
    failure.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}
</script>
<template>
  <UButton color="neutral" variant="ghost" icon="i-tabler-flag" @click="begin">{{
    t('workflow.community.report')
  }}</UButton
  ><BaseModal v-model:open="open" :title="t('workflow.community.report')" :dismissible="!busy"
    ><template #body
      ><form class="space-y-4" @submit.prevent="submit">
        <UFormField :label="t('workflow.community.report_reason')"
          ><USelect
            v-model="draft.reason"
            :items="
              ['plagiarism', 'outdated', 'other'].map((value) => ({
                label: t(`workflow.community.report_${value}`),
                value,
              }))
            "
            class="w-full" /></UFormField
        ><UFormField
          v-if="draft.reason === 'plagiarism'"
          :label="t('workflow.community.report_original')"
          required
          ><UInput
            v-model="draft.originalWork"
            required
            maxlength="1000"
            class="w-full" /></UFormField
        ><UFormField
          :label="t('workflow.community.report_description')"
          :required="draft.reason === 'other'"
          ><UTextarea
            v-model="draft.description"
            :required="draft.reason === 'other'"
            maxlength="4000"
            class="w-full" /></UFormField
        ><UAlert v-if="failure" color="error" :title="failure" />
        <div class="flex justify-end">
          <UButton type="submit" :loading="busy">{{
            t('workflow.community.report_submit')
          }}</UButton>
        </div>
      </form>
      <section v-if="reports.length" class="mt-6 space-y-3 border-t border-default pt-4">
        <h3 class="text-sm font-medium">{{ t('workflow.community.report_history') }}</h3>
        <article v-for="item in reports" :key="item.id" class="space-y-1 text-sm">
          <div class="flex justify-between gap-2">
            <span>{{ t(`workflow.community.report_${item.reason}`) }}</span
            ><span class="text-muted">{{ t(`workflow.community.report_${item.state}`) }}</span>
          </div>
          <p v-if="item.result" class="whitespace-pre-wrap">{{ item.result }}</p>
        </article>
      </section></template
    ></BaseModal
  >
</template>
