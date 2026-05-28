import { api, type JsonObject } from './client'

export const sslApi = {
  getSSLCertificates: () => api.get('/v1/ssl/certificates'),
  getSSLCertificate: (id: string) => api.get(`/v1/ssl/certificates/${id}`),
  createSSLCertificate: (payload: JsonObject) => api.post('/v1/ssl/certificates', payload),
  updateSSLCertificate: (id: string, payload: JsonObject) => api.put(`/v1/ssl/certificates/${id}`, payload),
  deleteSSLCertificate: (id: string) => api.delete(`/v1/ssl/certificates/${id}`),
  checkSSLCertificateNow: (id: string) => api.post(`/v1/ssl/certificates/${id}/check-now`),
}
