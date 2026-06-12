// NPM domain types — re-exported from the generated Go models (generated.ts).
export type {
  NPMConnection,
  NPMConnectionRequest,
  NPMProxyHost,
  NPMProxyHostUpdateRequest,
} from './generated'

// tygo renders Go struct embeds as nested objects; flatten the embed so
// NPMProxyHostEnriched can be used with direct property access.
import type { NPMProxyHost, NPMProxyHostEnriched as _Generated } from './generated'
export type NPMProxyHostEnriched = Omit<_Generated, 'NPMProxyHost'> & NPMProxyHost
