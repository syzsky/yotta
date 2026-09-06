import { createI18n } from 'vue-i18n'
import zh from './zh'

export type Locale = 'zh' | 'en'
export const LOCALES: Locale[] = ['zh', 'en']

export const i18n = createI18n({
  legacy: false,
  globalInjection: true,
  locale: 'zh' as Locale,
  fallbackLocale: 'zh' as Locale,
  // 静默 fallback：缺键时自动回 zh，控制台不刷屏
  fallbackWarn: false,
  missingWarn: false,
  messages: { zh, en: {} as typeof zh },
})

// Load the optional locale only when selected, retaining plugin messages.
let requestedLocale: Locale = 'zh'
let english: Promise<void> | undefined
export async function setLocale(loc: Locale): Promise<boolean> {
  requestedLocale = loc
  if (loc === 'en') {
    english ??= import('./en')
      .then((module) => {
        i18n.global.mergeLocaleMessage('en', module.default)
      })
      .catch((error) => {
        english = undefined
        throw error
      })
    try {
      await english
    } catch {
      return false
    }
  }
  if (requestedLocale === loc) i18n.global.locale.value = loc
  return true
}
