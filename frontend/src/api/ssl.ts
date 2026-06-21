import { api } from './client'
import type { SSLCertificate, SSLCertificateRequest, SSLCertificateEvent } from '../types/ssl'

export const sslApi = {
  getSSLCertificates: () => api.get<{ certificates: SSLCertificate[] }>('/v1/ssl/certificates'),
  getSSLCertificate: (id: string, signal?: AbortSignal) => api.get<SSLCertificate>(`/v1/ssl/certificates/${id}`, { signal }),
  createSSLCertificate: (payload: Partial<SSLCertificateRequest>) => api.post('/v1/ssl/certificates', payload),
  updateSSLCertificate: (id: string, payload: Partial<SSLCertificateRequest>) => api.put(`/v1/ssl/certificates/${id}`, payload),
  deleteSSLCertificate: (id: string) => api.delete(`/v1/ssl/certificates/${id}`),
  checkSSLCertificateNow: (id: string) => api.post<SSLCertificate>(`/v1/ssl/certificates/${id}/check-now`),
  getSSLCertificateHistory: (id: string, signal?: AbortSignal) => api.get<{ events: SSLCertificateEvent[] }>(`/v1/ssl/certificates/${id}/history`, { signal }),
}
