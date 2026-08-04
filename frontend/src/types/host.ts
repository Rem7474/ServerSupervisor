// Host domain types. The base shape is generated from the Go model
// (see generated.ts); this file re-exports it and layers a refined status union.
import type { Host as GeneratedHost, DiagnosticIssue as GeneratedDiagnosticIssue } from './generated'

/** Host lifecycle status (server stores it as a plain string). */
export type HostStatus = 'online' | 'offline' | 'warning' | 'unknown'

/** Severity of a DiagnosticIssue (server stores it as a plain string). */
export type DiagnosticSeverity = 'error' | 'warning'

/** DiagnosticIssue with severity narrowed to the known set of values. */
export type DiagnosticIssue = Omit<GeneratedDiagnosticIssue, 'severity'> & { severity: DiagnosticSeverity }

/** Host with the status/diagnostics fields narrowed to their known value sets. */
export type Host = Omit<GeneratedHost, 'status' | 'diagnostics'> & { status: HostStatus; diagnostics: DiagnosticIssue[] }

// Request bodies (generated from the Go request models).
export type { HostRegistration, HostUpdate } from './generated'

// Host-exposure correlation (NPM domains routing to this host + their web-log traffic).
export type { HostExposure, HostExposedDomain } from './generated'
