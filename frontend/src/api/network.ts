import { api, type JsonObject } from './client'

export const networkApi = {
  getNetworkSnapshot: () => api.get('/v1/network'),
  getTopologySnapshot: () => api.get('/v1/network/topology'),
  getTopologyConfig: () => api.get('/v1/network/config'),
  saveTopologyConfig: (config: JsonObject) => api.put('/v1/network/config', config),
}
