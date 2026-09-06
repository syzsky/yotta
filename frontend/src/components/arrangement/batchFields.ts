export interface BatchField {
  id: string
  label: string
  kind: 'text' | 'number' | 'boolean' | 'icon' | 'select'
  required?: boolean
  options?: Array<{ label: string; value: string }>
}
