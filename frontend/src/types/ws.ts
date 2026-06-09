// WebSocket message types. The generated structs carry `type: string`; here we
// narrow the discriminator to string literals so consumers get a proper
// discriminated union (switch on `.type`).
import type {
  WSCommandStreamInit,
  WSCommandStreamChunk,
  WSCommandStatusUpdate,
} from './generated'

// Per-page snapshot payloads (re-exported for consumers of the typed endpoints).
export type {
  WSDashboardSnapshot,
  WSHostSnapshot,
  WSDockerSnapshot,
  WSNetworkSnapshot,
  WSAptSnapshot,
} from './generated'

export type CommandStreamInitMsg = WSCommandStreamInit & { type: 'cmd_stream_init' }
export type CommandStreamChunkMsg = WSCommandStreamChunk & { type: 'cmd_stream' }
export type CommandStatusUpdateMsg = WSCommandStatusUpdate & { type: 'cmd_status_update' }

/** Any message pushed over the live command-stream WebSocket. */
export type CommandStreamMessage =
  | CommandStreamInitMsg
  | CommandStreamChunkMsg
  | CommandStatusUpdateMsg
