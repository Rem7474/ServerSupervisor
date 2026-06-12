// SSL-certificate monitoring domain types — re-exported from the generated Go models.
export type { SSLCertificate, SSLCertificateRequest } from './generated'

// SSLCertificateEvent records one observed certificate version (renewal event).
// Not yet in generated.ts — added manually until the next gen:types run.
export interface SSLCertificateEvent {
  id: number
  certificate_id: string
  serial_number: string
  valid_from?: string
  valid_to?: string
  issuer?: string
  subject?: string
  detected_at: string
}
