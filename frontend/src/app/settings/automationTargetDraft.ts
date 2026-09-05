import type {
  AndroidAutomationTargetProfile,
  AutomationTargetTypeDescriptor,
  BrowserAutomationTargetProfile,
  DesktopAutomationTargetProfile,
  InstalledApplicationProfile,
  InstalledAutomationTargetProfile,
} from '@/lib/backend'

export type InputBackend = DesktopAutomationTargetProfile['inputBackend'] | ''
export type CaptureBackend = DesktopAutomationTargetProfile['captureBackend'] | ''
export type WindowTitleMatch = DesktopAutomationTargetProfile['windowTitleMatch']
export type WindowSelection = DesktopAutomationTargetProfile['windowSelection']

export interface AutomationTargetDraft {
  slot: string
  label: string
  targetKind: InstalledAutomationTargetProfile['targetKind']
  adapterKind: InstalledAutomationTargetProfile['adapterKind']
  profileVersion: string
  applicationSlot: string
  windowTitle: string
  windowTitleMatch: WindowTitleMatch
  windowSelection: WindowSelection
  windowClass: string
  inputBackend: InputBackend
  captureBackend: CaptureBackend
  mouseCounts360: number
  mouseCalibrationMode: 'active' | 'custom'
  resolveTimeoutMilliseconds: number
  adbSerial: string
  adbProduct: string
  adbModel: string
  adbDevice: string
  androidPackage: string
  browserEndpoint: string
  browserTargetId: string
  browserWebSocketUrl: string
  browserTitle: string
  browserUrl: string
  profile: Record<string, string | number>
  persisted: boolean
}

export function draftFromProfile(target: InstalledAutomationTargetProfile): AutomationTargetDraft {
  const profile = target.profile as Partial<
    DesktopAutomationTargetProfile & AndroidAutomationTargetProfile & BrowserAutomationTargetProfile
  >
  return {
    slot: target.slot,
    label: target.label,
    targetKind: target.targetKind,
    adapterKind: target.adapterKind,
    profileVersion: target.profileVersion,
    applicationSlot: profile.applicationSlot ?? '',
    windowTitle: profile.windowTitle ?? '',
    windowTitleMatch: profile.windowTitleMatch ?? 'exact',
    windowSelection: profile.windowSelection ?? 'unique',
    windowClass: profile.windowClass ?? '',
    inputBackend: profile.inputBackend ?? '',
    captureBackend: profile.captureBackend ?? '',
    mouseCounts360: profile.mouseCounts360 ?? 0,
    mouseCalibrationMode: (profile.mouseCounts360 ?? 0) > 0 ? 'custom' : 'active',
    resolveTimeoutMilliseconds: profile.resolveTimeoutMilliseconds ?? 3000,
    adbSerial: profile.adbSerial ?? '',
    adbProduct: profile.adbProduct ?? '',
    adbModel: profile.adbModel ?? '',
    adbDevice: profile.adbDevice ?? '',
    androidPackage: profile.androidPackage ?? '',
    browserEndpoint: profile.browserEndpoint ?? '',
    browserTargetId: profile.browserTargetId ?? '',
    browserWebSocketUrl: profile.browserWebSocketUrl ?? '',
    browserTitle: profile.browserTitle ?? '',
    browserUrl: profile.browserUrl ?? '',
    profile: editableProfile(profile),
    persisted: true,
  }
}

export function isDesktopTarget(target: AutomationTargetDraft): boolean {
  return target.targetKind === 'desktop-window' && target.adapterKind === 'win32'
}

export function isAndroidTarget(target: AutomationTargetDraft): boolean {
  return target.targetKind === 'android-device' && target.adapterKind === 'android-adb'
}

export function isBrowserTarget(target: AutomationTargetDraft): boolean {
  return target.targetKind === 'browser-cdp' && target.adapterKind === 'browser-cdp'
}

export function targetMetadata(target: AutomationTargetDraft): InstalledAutomationTargetProfile {
  const common = {
    slot: target.slot,
    label: target.label.trim(),
    targetKind: target.targetKind,
    adapterKind: target.adapterKind,
    profileVersion: target.profileVersion,
  }
  if (isDesktopTarget(target)) {
    return {
      ...common,
      profile: {
        applicationSlot: target.applicationSlot,
        windowTitle: target.windowTitle,
        windowTitleMatch: target.windowTitleMatch,
        windowSelection: target.windowSelection,
        windowClass: target.windowClass,
        inputBackend: target.inputBackend as DesktopAutomationTargetProfile['inputBackend'],
        captureBackend: target.captureBackend as DesktopAutomationTargetProfile['captureBackend'],
        mouseCounts360: target.mouseCalibrationMode === 'active' ? 0 : target.mouseCounts360,
        resolveTimeoutMilliseconds: target.resolveTimeoutMilliseconds,
      },
    }
  }
  if (isBrowserTarget(target)) {
    return {
      ...common,
      profile: {
        browserEndpoint: target.browserEndpoint.trim(),
        browserTargetId: target.browserTargetId.trim(),
        browserWebSocketUrl: target.browserWebSocketUrl.trim(),
        browserTitle: target.browserTitle.trim(),
        browserUrl: target.browserUrl.trim(),
        resolveTimeoutMilliseconds: target.resolveTimeoutMilliseconds,
      },
    }
  }
  if (isAndroidTarget(target)) {
    return {
      ...common,
      profile: {
        adbSerial: target.adbSerial.trim(),
        adbProduct: target.adbProduct.trim(),
        adbModel: target.adbModel.trim(),
        adbDevice: target.adbDevice.trim(),
        androidPackage: target.androidPackage.trim(),
        resolveTimeoutMilliseconds: target.resolveTimeoutMilliseconds,
      },
    }
  }
  return { ...common, profile: { ...target.profile } }
}

export function targetDraftComplete(
  target: AutomationTargetDraft,
  applications: InstalledApplicationProfile[],
  targetType: AutomationTargetTypeDescriptor | undefined,
): boolean {
  if (isBrowserTarget(target)) {
    return Boolean(
      target.label.trim() && target.browserEndpoint.trim() && target.browserTargetId.trim(),
    )
  }
  if (isAndroidTarget(target)) {
    return Boolean(target.label.trim() && target.adbSerial && target.androidPackage.trim())
  }
  if (isDesktopTarget(target)) {
    return Boolean(
      target.label.trim() &&
      target.applicationSlot &&
      applications.some((application) => application.slot === target.applicationSlot) &&
      target.windowTitle.trim() &&
      target.windowClass.trim(),
    )
  }
  return Boolean(
    target.label.trim() &&
    targetType &&
    targetType.fields
      .filter((field) => field.required)
      .every((field) => {
        const value = target.profile[field.id]
        if (field.kind === 'installation-slot') {
          return (
            typeof value === 'string' &&
            applications.some((application) => application.slot === value)
          )
        }
        return typeof value === 'number' ? Number.isFinite(value) : Boolean(value?.trim())
      }),
  )
}

export function defaultTargetProfile(
  type: AutomationTargetTypeDescriptor,
): Record<string, string | number> {
  return Object.fromEntries(
    type.fields.map((field) => [
      field.id,
      field.options?.[0] ??
        (field.kind === 'duration-ms' ? 3000 : field.kind === 'integer' ? 0 : ''),
    ]),
  )
}

export function profileFieldOptions(
  targetType: AutomationTargetTypeDescriptor | undefined,
  fieldID: string,
): string[] {
  return targetType?.fields.find((field) => field.id === fieldID)?.options ?? []
}

export function stringProfileValue(target: AutomationTargetDraft, fieldID: string): string {
  const value = target.profile[fieldID]
  return typeof value === 'string' ? value : value == null ? '' : String(value)
}

export function numberProfileValue(target: AutomationTargetDraft, fieldID: string): number {
  const value = target.profile[fieldID]
  return typeof value === 'number' && Number.isFinite(value) ? value : 0
}

function editableProfile(source: Record<string, unknown>): Record<string, string | number> {
  return Object.fromEntries(
    Object.entries(source).filter((entry): entry is [string, string | number] => {
      const value = entry[1]
      return typeof value === 'string' || typeof value === 'number'
    }),
  )
}
