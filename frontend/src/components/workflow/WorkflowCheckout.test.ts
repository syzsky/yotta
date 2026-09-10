import { createApp, defineComponent, h, nextTick } from 'vue'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import WorkflowCheckout from './WorkflowCheckout.vue'

const mocks = vi.hoisted(() => ({
  account: vi.fn(),
  login: vi.fn(),
  cancelLogin: vi.fn(),
  methods: vi.fn(),
  checkout: vi.fn(),
  status: vi.fn(),
  access: vi.fn(),
  open: vi.fn(),
  native: vi.fn(),
  renew: vi.fn(),
  wallet: vi.fn(),
  authorizeWallet: vi.fn(),
  payWallet: vi.fn(),
  cancelWallet: vi.fn(),
}))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@wailsio/runtime', () => ({ Browser: { OpenURL: mocks.open } }))
vi.mock('@/lib/invoke', () => ({ errorMessage: (error: Error) => error.message }))
vi.mock('@/app/transport/workflow', () => ({
  shopTransport: {
    wallet: mocks.wallet,
    authorizeWallet: mocks.authorizeWallet,
    payWallet: mocks.payWallet,
    cancelWalletAuthorization: mocks.cancelWallet,
    account: mocks.account,
    login: mocks.login,
    cancelLogin: mocks.cancelLogin,
    paymentMethods: mocks.methods,
    checkout: mocks.checkout,
    checkoutState: mocks.status,
    nativePayment: mocks.native,
    renewPaymentSession: mocks.renew,
  },
  workflowTransport: { registryCommerce: mocks.access },
}))
let app: ReturnType<typeof createApp> | undefined
const ready = vi.fn()
async function flush() {
  for (let i = 0; i < 25; i++) await Promise.resolve()
  await nextTick()
}
async function mount() {
  const root = document.createElement('div')
  document.body.append(root)
  app = createApp(WorkflowCheckout, { workflowId: 'work-1', onReady: ready })
  app.use(
    createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/', component: { render: () => null } }],
    }),
  )
  app.component(
    'UButton',
    defineComponent({
      setup:
        (_, { slots, attrs }) =>
        () =>
          h('button', attrs, slots.default?.()),
    }),
  )
  app.mount(root)
  await flush()
  return root
}
beforeEach(() => {
  vi.useFakeTimers()
  vi.clearAllMocks()
  localStorage.clear()
  mocks.account.mockResolvedValue({ user_key: 'buyer-A' })
  mocks.access.mockResolvedValue({ entitled: false })
  mocks.methods.mockResolvedValue([
    { provider: 'test', method: 'redirect', enabled: true, label: 'Test' },
  ])
  mocks.checkout.mockResolvedValue({
    orderNo: 'order-1',
    provider: 'test',
    currency: 'CNY',
    amountCents: 100,
    payUrl: 'https://pay.example.test/session',
  })
  mocks.status.mockResolvedValue({ status: 'paying', deliveryState: 'pending' })
  mocks.native.mockResolvedValue({
    orderNo: 'order-1',
    title: 'Work',
    amountCents: 100,
    currency: 'CNY',
    provider: 'test',
    status: 'pending',
    orderStatus: 'paying',
    canConfirm: false,
    canCancel: true,
    qrCode: '',
    payUrl: 'https://pay.example.test/session',
  })
  mocks.renew.mockResolvedValue({
    orderNo: 'order-1',
    payUrl: 'https://pay.example.test/fresh',
    expiresAt: '2099-01-01T00:00:00Z',
  })
})
afterEach(() => {
  app?.unmount()
  document.body.innerHTML = ''
  vi.useRealTimers()
})

it('bounds paid access waiting and recovers the same order when entitlement arrives', async () => {
  mocks.status.mockResolvedValue({ status: 'fulfilled', deliveryState: 'granted' })
  const root = await mount()
  expect(root.textContent).toContain('workflow.checkout.confirming')
  await vi.advanceTimersByTimeAsync(30000)
  await flush()
  expect(root.textContent).toContain('workflow.checkout.access_delayed')
  expect(root.textContent).toContain('workflow.checkout.order')
  expect(root.textContent).not.toContain('workflow.checkout.cancel')
  expect(root.textContent).not.toContain('workflow.checkout.pay')
  expect(ready).not.toHaveBeenCalled()
  expect(localStorage.getItem('yotta:purchase:buyer-A:work-1')).toContain('order-1')
  const retry = [...root.querySelectorAll('button')].find(
    (item) => item.textContent === 'workflow.checkout.refresh_access',
  )!
  expect(retry).toBeTruthy()
  mocks.access.mockResolvedValue({ entitled: true })
  retry.click()
  await flush()
  expect(mocks.status).toHaveBeenLastCalledWith('order-1', 'sync')
  expect(mocks.checkout).toHaveBeenCalledTimes(1)
  expect(ready).toHaveBeenCalledTimes(1)
  expect(localStorage.getItem('yotta:purchase:buyer-A:work-1')).toBeNull()
})

it('uses the app account and creates one checkout without an intermediate website or order button', async () => {
  const root = await mount()
  expect(mocks.login).not.toHaveBeenCalled()
  expect(mocks.checkout).toHaveBeenCalledTimes(1)
  expect(mocks.open).not.toHaveBeenCalled()
  expect(root.textContent).toContain('workflow.checkout.pay')
  expect(ready).not.toHaveBeenCalled()
  mocks.access.mockResolvedValue({ entitled: true })
  await vi.advanceTimersByTimeAsync(2500)
  await flush()
  expect(ready).toHaveBeenCalledTimes(1)
  await vi.advanceTimersByTimeAsync(10000)
  expect(ready).toHaveBeenCalledTimes(1)
})

it('restores the same purchase intent after closing, without creating a new idempotency key', async () => {
  await mount()
  const original = mocks.checkout.mock.calls[0][1]
  app?.unmount()
  app = undefined
  await mount()
  expect(mocks.checkout).toHaveBeenCalledTimes(1)
  expect(mocks.native).toHaveBeenLastCalledWith('order-1')
  expect(JSON.parse(localStorage.getItem('yotta:purchase:buyer-A:work-1')!).idempotencyKey).toBe(
    original.idempotencyKey,
  )
  expect(localStorage.getItem('yotta:purchase:buyer-A:work-1')).not.toContain('https:')
})

it('does not install or poll a previous buyer order after the account changes', async () => {
  const root = await mount()
  const calls = mocks.status.mock.calls.length
  mocks.account.mockResolvedValue({ user_key: 'buyer-B' })
  mocks.access.mockResolvedValue({ entitled: true })
  await vi.advanceTimersByTimeAsync(2500)
  await flush()
  expect(mocks.status).toHaveBeenCalledTimes(calls)
  expect(ready).not.toHaveBeenCalled()
  expect(root.textContent).toContain('workflow.checkout.account_changed')
})

it('shows the missing payment action and a cancel action rather than pretending an order is paid', async () => {
  mocks.checkout.mockResolvedValue({
    orderNo: 'dev-order',
    provider: 'dev',
    currency: 'CNY',
    amountCents: 100,
    payUrl: 'dev://pay/dev-order',
  })
  mocks.native.mockResolvedValue({
    orderNo: 'dev-order',
    amountCents: 100,
    currency: 'CNY',
    provider: 'dev',
    status: 'pending',
    canConfirm: false,
    payUrl: '',
  })
  const root = await mount()
  expect(root.textContent).toContain('workflow.checkout.missing_action')
  expect(root.textContent).toContain('workflow.checkout.cancel')
  expect(ready).not.toHaveBeenCalled()
  expect(mocks.open).not.toHaveBeenCalled()
})

it('confirms a permitted local payment inside the app and installs only after central access is granted', async () => {
  mocks.native.mockImplementation(async (_order: string, confirm: boolean) => {
    if (confirm) mocks.access.mockResolvedValue({ entitled: true })
    return {
      orderNo: 'order-1',
      amountCents: 100,
      currency: 'CNY',
      provider: 'dev',
      status: confirm ? 'paid' : 'pending',
      canConfirm: !confirm,
      payUrl: '',
    }
  })
  const root = await mount()
  const button = [...root.querySelectorAll('button')].find(
    (item) => item.textContent === 'workflow.checkout.simulate',
  )!
  expect(button).toBeTruthy()
  button.click()
  button.click()
  await flush()
  expect(mocks.native.mock.calls.filter((call) => call[1] === true)).toHaveLength(1)
  expect(mocks.open).not.toHaveBeenCalled()
  expect(ready).toHaveBeenCalledTimes(1)
  expect(localStorage.getItem('yotta:purchase:buyer-A:work-1')).toBeNull()
})

it('renews a provider payment handoff without creating another order', async () => {
  const root = await mount()
  const button = [...root.querySelectorAll('button')].find(
    (item) => item.textContent === 'workflow.checkout.pay',
  )!
  button.click()
  await flush()
  expect(mocks.renew).toHaveBeenCalledWith('order-1')
  expect(mocks.open).toHaveBeenCalledWith('https://pay.example.test/fresh')
  expect(mocks.checkout).toHaveBeenCalledTimes(1)
})

it('keeps a rejected cancellation visible and preserves the order while polling continues', async () => {
  const root = await mount()
  mocks.status.mockRejectedValueOnce(new Error('provider cannot cancel; order remains active'))
  const button = [...root.querySelectorAll('button')].find(
    (item) => item.textContent === 'workflow.checkout.cancel',
  )!
  button.click()
  await flush()
  expect(root.textContent).toContain('provider cannot cancel; order remains active')
  await vi.advanceTimersByTimeAsync(2500)
  await flush()
  expect(root.textContent).toContain('provider cannot cancel; order remains active')
  expect(localStorage.getItem('yotta:purchase:buyer-A:work-1')).toContain('order-1')
  expect(ready).not.toHaveBeenCalled()
})

it('confirms the central amount once with wallet balance and waits for download rights', async () => {
  mocks.native.mockResolvedValue({
    orderNo: 'order-1',
    amountCents: 100,
    currency: 'CNY',
    provider: 'alipay',
    status: 'pending',
    canChooseMethod: true,
  })
  mocks.wallet.mockResolvedValue({ authorized: true, balanceCents: 10000, currency: 'CNY' })
  mocks.payWallet.mockResolvedValue({ orderNo: 'order-1', status: 'fulfilled' })
  const root = await mount()
  const button = [...root.querySelectorAll('button')].find(
    (item) => item.textContent === 'workflow.checkout.wallet_pay',
  )!
  button.click()
  button.click()
  await flush()
  expect(mocks.payWallet).toHaveBeenCalledTimes(1)
  expect(mocks.payWallet).toHaveBeenCalledWith({
    orderNo: 'order-1',
    amountCents: 100,
    currency: 'CNY',
  })
  expect(ready).not.toHaveBeenCalled()
  expect(mocks.open).not.toHaveBeenCalled()
  mocks.status.mockResolvedValue({ status: 'fulfilled' })
  mocks.access.mockResolvedValue({ entitled: true })
  await vi.advanceTimersByTimeAsync(2500)
  await flush()
  expect(ready).toHaveBeenCalledTimes(1)
})
it('authorization alone never pays and is cancelled when the purchase view closes', async () => {
  mocks.native.mockResolvedValue({
    orderNo: 'order-1',
    amountCents: 100,
    currency: 'CNY',
    provider: 'alipay',
    status: 'pending',
    canChooseMethod: true,
  })
  mocks.wallet.mockResolvedValue({ authorized: false })
  mocks.authorizeWallet.mockImplementation(() => new Promise(() => {}))
  const root = await mount()
  const button = [...root.querySelectorAll('button')].find(
    (item) => item.textContent === 'workflow.checkout.wallet_authorize',
  )!
  button.click()
  await flush()
  expect(mocks.authorizeWallet).toHaveBeenCalledTimes(1)
  expect(mocks.payWallet).not.toHaveBeenCalled()
  expect(ready).not.toHaveBeenCalled()
  app?.unmount()
  app = undefined
  expect(mocks.cancelWallet).toHaveBeenCalledTimes(1)
})
