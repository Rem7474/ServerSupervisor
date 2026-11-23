// Untyped JSON payloads (WS snapshot / HTTP) as held by useHostDetail. The
// host-detail child components narrow the few fields they read themselves.
// eslint-disable-next-line @typescript-eslint/no-explicit-any -- intentional display-layer shim shared with useHostDetail's AnyRecord
export type HostRecord = Record<string, any>
