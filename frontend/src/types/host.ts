// Host domain types. The base shape is generated from the Go model
// (see generated.ts); this file re-exports it and layers a refined status union.
import type { Host as GeneratedHost } from './generated'

/** Host lifecycle status (server stores it as a plain string). */
export type HostStatus = 'online' | 'offline' | 'warning' | 'unknown'

/** Host with the status field narrowed to the known set of values. */
export type Host = Omit<GeneratedHost, 'status'> & { status: HostStatus }
