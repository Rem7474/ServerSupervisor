export type ParamType = 'string' | 'int' | 'float' | 'bool' | 'duration' | 'csv' | 'select'

export interface ConfigEntry {
  key: string
  setting_key: string
  env_var: string
  label: string
  description: string
  category: string
  type: ParamType
  options?: string[]
  default_value: string
  effective_value: string
  source: 'env' | 'ui' | 'default'
  is_secret: boolean
  is_editable: boolean
  requires_restart: boolean
  has_env_override: boolean
  env_value?: string
  ui_value?: string
  has_conflict: boolean
}

export interface ConfigSummary {
  entries: ConfigEntry[]
  total_params: number
  env_count: number
  ui_count: number
  default_count: number
  conflict_count: number
  categories: string[]
}

export interface UpdateConfigKeyResponse {
  success: boolean
  entry?: ConfigEntry
  warning?: string
  message: string
}

export interface UpdateConfigBulkResponse {
  success: boolean
  summary?: ConfigSummary
  warnings?: string[]
  message: string
}
