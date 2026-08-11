// Shared shape for components/common/TimeRangePicker.vue's v-model, used by
// every composable that wires a time range into an API call (useTraffic,
// useBot, useNetworkFlowsHistory-backed components). Kept out of the .vue
// file so composables don't have to import types from a component.

export interface TimeRangePreset {
  value: string
  label: string
}

// `period` is only meaningful when mode==='preset'; from/to are ISO 8601 UTC
// strings, only set when mode==='custom'.
export interface TimeRangeModel {
  mode: 'preset' | 'custom'
  period: string
  from: string | null
  to: string | null
}
