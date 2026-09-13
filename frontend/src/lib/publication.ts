import { isNewerRelease, nextRelease } from '@/app/workflow-library/releaseVersion'

export function publicationVersion(published: string, rejected?: string): string {
  if (rejected && (!published || isNewerRelease(rejected, published))) return rejected
  return published ? nextRelease(published) : '1.0.0'
}

export interface SalesDraft {
  paid: boolean
  price: string
  licenseRef: string
  revision: number
  available: boolean
  configured: boolean
}

export function priceCents(price: string): number | null {
  if (!/^\d+(\.\d{1,2})?$/.test(price.trim())) return null
  const [whole, fraction = ''] = price.trim().split('.')
  const cents = Number(whole) * 100 + Number(fraction.padEnd(2, '0'))
  return Number.isSafeInteger(cents) && cents > 0 && cents <= 2147483647 ? cents : null
}

export function validSalesDraft(draft: SalesDraft): boolean {
  if (!draft.paid && !draft.configured) return true
  return (!draft.paid || priceCents(draft.price) !== null) && validLicenseRef(draft.licenseRef)
}

export function validLicenseRef(value: string): boolean {
  return value.trim().length > 0 && new TextEncoder().encode(value.trim()).length <= 500
}

export function publicationSales(draft: SalesDraft) {
  if (!draft.paid && !draft.configured) return undefined
  if (!validSalesDraft(draft)) throw new Error('Invalid sales draft')
  return {
    priceCents: draft.paid ? priceCents(draft.price)! : 0,
    currency: 'CNY',
    available: draft.available,
    licenseRef: draft.licenseRef.trim(),
    revision: draft.revision,
  }
}
