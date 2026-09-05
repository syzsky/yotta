export function validReleaseVersion(value: string): boolean {
  return (
    value === value.trim() &&
    value.length <= 128 &&
    /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/.test(value)
  )
}

export function isNewerRelease(candidate: string, installed: string): boolean {
  if (!validReleaseVersion(candidate) || !validReleaseVersion(installed)) return false
  const left = candidate.split('.').map(BigInt),
    right = installed.split('.').map(BigInt)
  for (let i = 0; i < 3; i++) if (left[i] !== right[i]) return left[i] > right[i]
  return false
}

export function nextRelease(version: string, component: 0 | 1 | 2 = 2): string {
  if (!validReleaseVersion(version)) return '1.0.0'
  const parts = version.split('.').map(BigInt)
  parts[component]++
  for (let i = component + 1; i < 3; i++) parts[i] = 0n
  return parts.join('.')
}
