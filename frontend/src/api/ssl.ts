import { api } from './client'
import type { SSLCertificate, SSLCertificateRequest } from '../types/ssl'

export const sslApi = {
  getSSLCertificates: () => api.get<{ certificates: SSLCertificate[] }>('/v1/ssl/certificates'),
  getSSLCertificate: (id: string) => api.get<SSLCertificate>(`/v1/ssl/certificates/${id}`),
  createSSLCertificate: (payload: Partial<SSLCertificateRequest>) => api.post('/v1/ssl/certificates', payload),
  updateSSLCertificate: (id: string, payload: Partial<SSLCertificateRequest>) => api.put(`/v1/ssl/certificates/${id}`, payload),
  deleteSSLCertificate: (id: string) => api.delete(`/v1/ssl/certificates/${id}`),
  checkSSLCertificateNow: (id: string) => api.post(`/v1/ssl/certificates/${id}/check-now`),
}
