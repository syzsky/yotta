export function validReleaseVersion(value: string): boolean {
  return (
    value === value.trim() &&
    value.length <= 128 &&
    /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/.test(value)
  )
}
