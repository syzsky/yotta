<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Browser } from '@wailsio/runtime'
import { shopTransport, workflowTransport } from '@/app/transport/workflow'
import { errorMessage } from '@/lib/invoke'
import type {
  CheckoutPayment,
  PaymentMethod,
  PaymentView,
} from '@bindings/github.com/yottaapp/yotta/internal/registryclient/models.js'

const props = defineProps<{ workflowId: string }>()
const emit = defineEmits<{ ready: []; close: [] }>()
const { t } = useI18n()
const busy = ref(false),
  failure = ref(''),
  state = ref('preparing')
const methods = ref<PaymentMethod[]>([]),
  provider = ref('')
const payment = ref<CheckoutPayment | null>(null),
  qrImage = ref('')
const paymentView = ref<PaymentView | null>(null)
const wallet = ref<{ authorized: boolean; currency: string; balanceCents: number } | null>(null)
const walletAuthorizing = ref(false)
const paid = ref(false)
let paidSince: number | undefined
let buyer = '',
  key = '',
  intent: { idempotencyKey: string; provider: string; orderNo?: string } | null = null
let disposed = false,
  checking = false,
  completed = false
let actionFailed = false
let timer: ReturnType<typeof setTimeout> | undefined

function confirmAccess() {
  paid.value = true
  paidSince ??= Date.now()
  state.value = Date.now() - paidSince >= 30000 ? 'access_delayed' : 'confirming'
}

function paymentURL() {
  try {
    const url = new URL(
      paymentView.value?.payUrl ||
        payment.value?.paymentSession?.payUrl ||
        payment.value?.payUrl ||
        '',
    )
    return ['https:', 'http:'].includes(url.protocol) ? url.toString() : ''
  } catch {
    return ''
  }
}
function forget() {
  if (key) localStorage.removeItem(key)
  intent = null
}
async function currentBuyer() {
  const profile = await shopTransport.account()
  if (profile.user_key === buyer) return true
  state.value = 'account_changed'
  return false
}
async function check(action: 'status' | 'sync' = 'status') {
  if (disposed || completed || checking || !payment.value) return
  checking = true
  try {
    if (!(await currentBuyer()) || disposed) return
    const result = await shopTransport.checkoutState(payment.value.orderNo, action)
    if (disposed) return
    if (['cancelled', 'canceled', 'expired', 'closed', 'refunded'].includes(result.status)) {
      state.value = 'ended'
      forget()
      return
    }
    if (result.status === 'failed') {
      state.value = 'failed'
      return
    }
    if (['paid', 'fulfilled'].includes(result.status)) confirmAccess()
    const access = await workflowTransport.registryCommerce?.(props.workflowId)
    if (disposed || !(await currentBuyer())) return
    if (access?.entitled) {
      completed = true
      forget()
      state.value = 'complete'
      emit('ready')
      return
    }
    if (paid.value) confirmAccess()
    else state.value = 'waiting'
    if (!actionFailed) failure.value = ''
  } catch (error) {
    if (paid.value && !disposed && state.value !== 'account_changed') confirmAccess()
    if (!disposed && !actionFailed) failure.value = errorMessage(error)
  } finally {
    checking = false
    if (!disposed && !completed && !['ended', 'account_changed', 'failed'].includes(state.value)) {
      timer = setTimeout(() => void check(), state.value === 'access_delayed' ? 10000 : 2500)
    }
  }
}
async function start() {
  if (busy.value || checking) return
  clearTimeout(timer)
  busy.value = true
  failure.value = ''
  actionFailed = false
  state.value = 'preparing'
  try {
    let profile = await shopTransport.account()
    if (!profile.user_key) {
      state.value = 'signing_in'
      profile = await shopTransport.login()
    }
    if (disposed) return
    buyer = profile.user_key
    if (!buyer) {
      state.value = 'account_changed'
      return
    }
    key = `yotta:purchase:${buyer}:${props.workflowId}`
    const access = await workflowTransport.registryCommerce?.(props.workflowId)
    if (disposed || !(await currentBuyer())) return
    if (access?.entitled) {
      forget()
      completed = true
      emit('ready')
      return
    }
    key = `yotta:purchase:${buyer}:${props.workflowId}`
    const saved = localStorage.getItem(key)
    if (saved) {
      try {
        const value = JSON.parse(saved)
        if (
          typeof value.idempotencyKey === 'string' &&
          value.idempotencyKey.length <= 128 &&
          typeof value.provider === 'string'
        )
          intent = {
            idempotencyKey: value.idempotencyKey,
            provider: value.provider,
            orderNo:
              typeof value.orderNo === 'string' && value.orderNo.length <= 128
                ? value.orderNo
                : undefined,
          }
      } catch {
        localStorage.removeItem(key)
      }
    }
    if (intent?.orderNo) {
      await loadPayment(intent.orderNo)
      return
    }
    paid.value = false
    paidSince = undefined
    methods.value = (await shopTransport.paymentMethods()).filter(
      (item) => item.enabled && ['redirect', 'native_qr', 'central'].includes(item.method),
    )
    if (disposed || !(await currentBuyer())) return
    provider.value = intent?.provider || provider.value || methods.value[0]?.provider || ''
    if (!provider.value) {
      state.value = 'unavailable'
      return
    }
    if (!intent && methods.value.length > 1 && !chosen) {
      state.value = 'choosing'
      return
    }
    if (!intent) intent = { idempotencyKey: crypto.randomUUID(), provider: provider.value }
    // Persist only the purchase intent, never user tokens or payment URL secrets.
    localStorage.setItem(key, JSON.stringify(intent))
    payment.value = await shopTransport.checkout(props.workflowId, {
      idempotencyKey: intent.idempotencyKey,
      provider: intent.provider,
    })
    if (disposed || !(await currentBuyer())) return
    intent.orderNo = payment.value.orderNo
    localStorage.setItem(key, JSON.stringify(intent))
    await loadPayment(payment.value.orderNo)
  } catch (error) {
    if (!disposed) {
      failure.value = errorMessage(error)
      state.value = 'failed'
    }
  } finally {
    busy.value = false
  }
}
async function loadPayment(orderNo: string) {
  const view = await shopTransport.nativePayment(orderNo)
  if (disposed || !(await currentBuyer())) return
  paymentView.value = view
  payment.value = {
    orderNo: view.orderNo,
    amountCents: view.amountCents,
    currency: view.currency,
    provider: view.provider,
    method: view.qrCode ? 'native_qr' : 'redirect',
    payUrl: view.payUrl,
    qrCode: view.qrCode,
  }
  if (['cancelled', 'refunded'].includes(view.status)) {
    state.value = 'ended'
    forget()
    return
  }
  if (view.status === 'failed') {
    state.value = 'failed'
    return
  }
  qrImage.value = ''
  if (view.qrCode) {
    const qr = await import('qrcode')
    qrImage.value = await qr.toDataURL(view.qrCode, { width: 240, margin: 2 })
  }
  if (view.canChooseMethod) {
    try {
      wallet.value = await shopTransport.wallet()
    } catch (error) {
      failure.value = errorMessage(error)
    }
    if (disposed || !(await currentBuyer())) return
  }
  if (view.status === 'paid') confirmAccess()
  else state.value = 'waiting'
  void check()
}
let chosen = false
function choose(value: string) {
  provider.value = value
  chosen = true
  void start()
}
async function cancel() {
  if (busy.value) return
  busy.value = true
  failure.value = ''
  actionFailed = false
  try {
    if (payment.value && (await currentBuyer())) {
      const result = await shopTransport.checkoutState(payment.value.orderNo, 'cancel')
      if (!['cancelled', 'canceled', 'closed', 'expired'].includes(result.status)) {
        void check()
        return
      }
      forget()
    }
    if (state.value === 'signing_in') await shopTransport.cancelLogin()
    emit('close')
  } catch (error) {
    actionFailed = true
    failure.value = errorMessage(error)
  } finally {
    busy.value = false
  }
}
async function pay() {
  if (busy.value || !payment.value) return
  busy.value = true
  failure.value = ''
  actionFailed = false
  try {
    if (!(await currentBuyer())) return
    if (paymentView.value?.canConfirm) {
      paymentView.value = await shopTransport.nativePayment(payment.value.orderNo, true)
      if (disposed || !(await currentBuyer())) return
      if (paymentView.value.status === 'paid') confirmAccess()
      clearTimeout(timer)
      await check()
      return
    }
    const session = await shopTransport.renewPaymentSession(payment.value.orderNo)
    if (disposed || !(await currentBuyer())) return
    payment.value.paymentSession = session
    const url = new URL(session.payUrl)
    if (!['https:', 'http:'].includes(url.protocol) || url.username || url.password) return
    await Browser.OpenURL(url.toString())
  } catch (error) {
    actionFailed = true
    failure.value = errorMessage(error)
  } finally {
    busy.value = false
  }
}
async function authorizeWallet() {
  if (busy.value || walletAuthorizing.value) return
  walletAuthorizing.value = true
  failure.value = ''
  try {
    if (!(await currentBuyer())) return
    const result = await shopTransport.authorizeWallet()
    if (!disposed && (await currentBuyer())) wallet.value = result
  } catch (error) {
    if (!disposed) failure.value = errorMessage(error)
  } finally {
    walletAuthorizing.value = false
  }
}
async function payWallet() {
  if (busy.value || !payment.value) return
  busy.value = true
  failure.value = ''
  actionFailed = false
  try {
    if (!(await currentBuyer())) return
    await shopTransport.payWallet({
      orderNo: payment.value.orderNo,
      amountCents: payment.value.amountCents,
      currency: payment.value.currency,
    })
    if (disposed || !(await currentBuyer())) return
    clearTimeout(timer)
    await check()
  } catch (error) {
    if (!disposed) {
      actionFailed = true
      failure.value = errorMessage(error)
    }
    try {
      wallet.value = await shopTransport.wallet()
    } catch {
      wallet.value = null
    }
  } finally {
    busy.value = false
  }
}
function refreshPayment() {
  clearTimeout(timer)
  void check('sync')
}
onMounted(() => {
  void start()
  window.addEventListener('focus', refreshPayment)
})
onUnmounted(() => {
  disposed = true
  if (walletAuthorizing.value) void shopTransport.cancelWalletAuthorization()
  clearTimeout(timer)
  window.removeEventListener('focus', refreshPayment)
})
</script>

<template>
  <section
    class="space-y-4 border-b border-default py-5"
    data-testid="workflow-checkout"
    :aria-label="t('workflow.checkout.title')"
  >
    <div class="flex items-center justify-between gap-4">
      <h3 class="text-base font-semibold">{{ t('workflow.checkout.title') }}</h3>
      <UButton color="neutral" variant="ghost" @click="emit('close')">{{
        t('workflow.checkout.close')
      }}</UButton>
    </div>
    <p role="status" class="text-sm text-muted">{{ t(`workflow.checkout.${state}`) }}</p>
    <p v-if="failure" role="alert" class="text-sm text-error">{{ failure }}</p>
    <div v-if="state === 'choosing'" class="flex flex-wrap gap-3">
      <UButton v-for="method in methods" :key="method.provider" @click="choose(method.provider)">{{
        method.method === 'central' ? t('workflow.checkout.wallet_method') : method.label
      }}</UButton>
    </div>
    <template v-if="payment && !paid && ['waiting', 'failed'].includes(state)">
      <p class="text-lg font-semibold tabular-nums">
        {{ payment.currency }} {{ (payment.amountCents / 100).toFixed(2) }}
      </p>
      <img
        v-if="qrImage"
        :src="qrImage"
        :alt="t('workflow.checkout.qr')"
        width="240"
        height="240"
      />
      <p v-if="paymentView?.canConfirm" class="text-sm text-muted">
        {{ t('workflow.checkout.simulation_note') }}
      </p>
      <p
        v-if="
          !qrImage && !paymentURL() && !paymentView?.canConfirm && !paymentView?.canChooseMethod
        "
        role="alert"
        class="text-sm text-error"
      >
        {{ t('workflow.checkout.missing_action') }}
      </p>
      <div v-if="paymentView?.canChooseMethod" class="space-y-3">
        <template v-if="wallet?.authorized">
          <p class="text-sm text-muted">
            {{
              t('workflow.checkout.wallet_balance', {
                amount: (wallet.balanceCents / 100).toFixed(2),
              })
            }}
          </p>
          <UButton
            :loading="busy"
            :disabled="busy || wallet.balanceCents < payment.amountCents"
            @click="payWallet"
            >{{ t('workflow.checkout.wallet_pay') }}</UButton
          >
          <p v-if="wallet.balanceCents < payment.amountCents" class="text-sm text-muted">
            {{ t('workflow.checkout.wallet_insufficient') }}
          </p>
        </template>
        <UButton
          v-else
          :loading="walletAuthorizing"
          :disabled="busy || walletAuthorizing"
          @click="authorizeWallet"
          >{{ t('workflow.checkout.wallet_authorize') }}</UButton
        >
        <p v-if="walletAuthorizing" class="text-sm text-muted">
          {{ t('workflow.checkout.wallet_waiting') }}
        </p>
      </div>
      <UButton
        v-if="paymentURL() || paymentView?.canConfirm"
        :loading="busy"
        :disabled="busy"
        @click="pay"
        >{{
          t(paymentView?.canConfirm ? 'workflow.checkout.simulate' : 'workflow.checkout.pay')
        }}</UButton
      >
    </template>
    <p v-if="payment" class="text-xs text-muted">
      {{ t('workflow.checkout.order', { order: payment.orderNo }) }}
    </p>
    <div class="flex flex-wrap gap-3">
      <UButton
        v-if="['confirming', 'access_delayed'].includes(state)"
        :loading="busy"
        @click="refreshPayment"
        >{{ t('workflow.checkout.refresh_access') }}</UButton
      >
      <UButton
        v-if="failure || ['unavailable', 'failed', 'ended'].includes(state)"
        :loading="busy"
        @click="start"
        >{{ t('common.retry') }}</UButton
      >
      <UButton
        v-if="payment && !paid && !completed && !['ended', 'account_changed'].includes(state)"
        color="neutral"
        variant="outline"
        :loading="busy"
        @click="cancel"
        >{{ t('workflow.checkout.cancel') }}</UButton
      >
    </div>
  </section>
</template>
