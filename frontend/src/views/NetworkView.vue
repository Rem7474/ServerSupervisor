<template>
  <div>
    <div class="page-header mb-4">
      <h2 class="page-title">Network</h2>
      <div class="text-secondary">Ports exposes et trafic par hote</div>
    </div>

    <WsStatusBar :status="wsStatus" :error="wsError" :retry-count="retryCount" @reconnect="reconnect" />

    <div class="row row-cards mb-4">
      <div class="col-sm-6 col-lg-3">
        <div class="card card-sm">
          <div class="card-body">
            <div class="subheader">Hotes</div>
            <div class="h1 mb-0">{{ hosts.length }}</div>
          </div>
        </div>
      </div>
      <div class="col-sm-6 col-lg-3">
        <div class="card card-sm">
          <div class="card-body">
            <div class="subheader">Conteneurs</div>
            <div class="h1 mb-0">{{ containers.length }}</div>
          </div>
        </div>
      </div>
      <div class="col-sm-6 col-lg-3">
        <div class="card card-sm">
          <div class="card-body">
            <div class="subheader">Ports visibles</div>
            <div class="h1 mb-0">{{ totalPorts }}</div>
          </div>
        </div>
      </div>
      <div class="col-sm-6 col-lg-3">
        <div class="card card-sm">
          <div class="card-body">
            <div class="subheader">Trafic total</div>
            <div class="h1 mb-0">{{ formatBytes(totalRx + totalTx) }}</div>
            <div class="text-secondary small">Rx {{ formatBytes(totalRx) }} / Tx {{ formatBytes(totalTx) }}</div>
          </div>
        </div>
      </div>
    </div>

    <!-- View Mode Toggle -->
    <div class="card mb-4">
      <div class="card-body">
        <div class="btn-group" role="group">
          <input type="radio" class="btn-check" id="viewCards" value="cards" v-model="viewMode" />
          <label class="btn btn-outline-primary" for="viewCards">
            <svg width="18" height="18" fill="currentColor" viewBox="0 0 16 16" style="margin-right: 0.25rem;">
              <path d="M1 1a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1H2a1 1 0 0 1-1-1V1zm10 0a1 1 0 0 1 1-1h2a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1h-2a1 1 0 0 1-1-1V1zM1 11a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1H2a1 1 0 0 1-1-1v-4zm10 0a1 1 0 0 1 1-1h2a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1h-2a1 1 0 0 1-1-1v-4z"/>
            </svg>
            <span class="d-none d-sm-inline">Cards</span>
          </label>
          
          <input type="radio" class="btn-check" id="viewGraph" value="graph" v-model="viewMode" />
          <label class="btn btn-outline-primary" for="viewGraph">
            <svg width="18" height="18" fill="currentColor" viewBox="0 0 16 16" style="margin-right: 0.25rem;">
              <path d="M0 2a2 2 0 0 1 2-2h12a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2H2a2 2 0 0 1-2-2V2zm2.5 7a.5.5 0 0 0-.5.5v1a.5.5 0 0 0 .5.5h1a.5.5 0 0 0 .5-.5v-1a.5.5 0 0 0-.5-.5h-1zm2-4a.5.5 0 0 0-.5.5v5a.5.5 0 0 0 .5.5h1a.5.5 0 0 0 .5-.5v-5a.5.5 0 0 0-.5-.5h-1zm2-2a.5.5 0 0 0-.5.5v8a.5.5 0 0 0 .5.5h1a.5.5 0 0 0 .5-.5V3.5a.5.5 0 0 0-.5-.5h-1zm2-1a.5.5 0 0 0-.5.5v9a.5.5 0 0 0 .5.5h1a.5.5 0 0 0 .5-.5V2.5a.5.5 0 0 0-.5-.5h-1z"/>
            </svg>
            <span class="d-none d-sm-inline">Graph</span>
          </label>
        </div>
      </div>
    </div>

    <!-- Graph View -->
    <div v-if="viewMode === 'graph'" class="card mb-4 network-topology-card">
      <div class="card-header d-flex align-items-center justify-content-between">
        <div>
          <h3 class="card-title mb-1">Network Topology</h3>
          <div class="text-secondary small">Glisser pour reordonner, scroll pour zoomer</div>
        </div>
        <div class="d-flex align-items-center gap-3">
          <div v-if="saveStatus !== 'idle'" class="d-flex align-items-center gap-2">
            <span v-if="saveStatus === 'saving'" class="spinner-border spinner-border-sm"></span>
            <span v-else-if="saveStatus === 'saved'" class="text-success small">✓ Enregistré</span>
            <span v-else-if="saveStatus === 'error'" class="text-danger small">✗ Erreur</span>
          </div>
          <div class="text-secondary small">
            {{ hosts.length }} hotes • {{ totalPorts }} ports publies
          </div>
        </div>
      </div>
      <div class="network-subnav" style="gap: 0.5rem;">
        <button class="btn" :class="networkTab === 'topology' ? 'btn-primary' : 'btn-outline-primary'" @click="networkTab = 'topology'">
          <svg width="16" height="16" fill="currentColor" viewBox="0 0 16 16" style="margin-right: 0.25rem; display: inline;"><path d="M2 2a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2v13.5a.5.5 0 0 1-.777.416L8 13.71l-5.223 2.206A.5.5 0 0 1 2 15.5V2zm2-1a1 1 0 0 0-1 1v12.566l4.723-2.482a.5.5 0 0 1 .554 0L13 14.566V2a1 1 0 0 0-1-1H4z"/></svg>
          Topology
        </button>
        <button class="btn" :class="networkTab === 'config' ? 'btn-primary' : 'btn-outline-primary'" @click="networkTab = 'config'">
          <svg width="16" height="16" fill="currentColor" viewBox="0 0 16 16" style="margin-right: 0.25rem; display: inline;"><path d="M9.405 1.05c-.413-1.4-2.397-1.4-2.81 0l-.1.34a1.464 1.464 0 0 1-2.105.872l-.31-.17c-1.283-.698-2.686.264-2.17 1.655l.119.355a1.464 1.464 0 0 1-1.738 1.738l-.355-.119c-1.39-.516-2.353 1.102-1.656 2.17l.17.31a1.464 1.464 0 0 1-.872 2.105l-.34.1c-1.4.413-1.4 2.397 0 2.81l.34.1a1.464 1.464 0 0 1 .872 2.105l-.17.31c-.697 1.283.264 2.686 1.655 2.17l.355-.119a1.464 1.464 0 0 1 1.738 1.738l-.119.355c-.516 1.39 1.102 2.353 2.17 1.656l.31-.17a1.464 1.464 0 0 1 2.105.872l.1.34c.413 1.4 2.397 1.4 2.81 0l.1-.34a1.464 1.464 0 0 1 2.105-.872l.31.17c1.283.697 2.686-.264 2.17-1.655l-.119-.355a1.464 1.464 0 0 1 1.738-1.738l.355.119c1.39.516 2.353-1.102 1.656-2.17l-.17-.31a1.464 1.464 0 0 1 .872-2.105l.34-.1c1.4-.413 1.4-2.397 0-2.81l-.34-.1a1.464 1.464 0 0 1-.872-2.105l.17-.31c.697-1.283-.264-2.686-1.655-2.17l-.355.119a1.464 1.464 0 0 1-1.738-1.738l.119-.355c.516-1.39-1.102-2.353-2.17-1.656l-.31.17a1.464 1.464 0 0 1-2.105-.872l-.1-.34zM8 10.93a2.929 2.929 0 1 1 0-5.86 2.929 2.929 0 0 1 0 5.858z"/></svg>
          Configuration
        </button>
        <button class="btn" :class="networkTab === 'auto' ? 'btn-primary' : 'btn-outline-primary'" @click="networkTab = 'auto'">
          <svg width="16" height="16" fill="currentColor" viewBox="0 0 16 16" style="margin-right: 0.25rem; display: inline;"><path d="M8 4.754a3.246 3.246 0 1 0 0 6.492 3.246 3.246 0 0 0 0-6.492zM5.754 8a2.246 2.246 0 1 1 4.492 0 2.246 2.246 0 0 1-4.492 0z"/><path d="M9.796 1.343c-.527-1.79-3.065-1.79-3.592 0l-.094.319a.873.873 0 0 1-1.255.52l-.292-.16c-1.64-.892-3.433.902-2.54 2.541l.159.292a.873.873 0 0 1-.52 1.255l-.319.094c-1.79.527-1.79 3.065 0 3.592l.319.094a.873.873 0 0 1 .52 1.255l-.16.292c-.892 1.64.901 3.434 2.541 2.54l.292-.159a.873.873 0 0 1 1.255.52l.094.319c.527 1.79 3.065 1.79 3.592 0l.094-.319a.873.873 0 0 1 1.255-.52l.292.16c1.64.893 3.434-.902 2.54-2.541l-.159-.292a.873.873 0 0 1 .52-1.255l.319-.094c1.79-.527 1.79-3.065 0-3.592l-.319-.094a.873.873 0 0 1-.52-1.255l.16-.292c.893-1.64-.902-3.433-2.541-2.54l-.292.159a.873.873 0 0 1-1.255-.52l-.094-.319zm-2.633.283c.246-.835 1.428-.835 1.674 0l.094.319a1.873 1.873 0 0 0 2.693 1.115l.291-.16c.764-.415 1.6.42 1.184 1.185l-.159.292a1.873 1.873 0 0 0 1.116 2.692l.318.094c.835.246.835 1.428 0 1.674l-.319.094a1.873 1.873 0 0 0-1.115 2.693l.16.291c.415.764-.42 1.6-1.185 1.184l-.291-.159a1.873 1.873 0 0 0-2.693 1.116l-.094.318c-.246.835-1.428.835-1.674 0l-.094-.319a1.873 1.873 0 0 0-2.692-1.115l-.292.16c-.764.415-1.6-.42-1.184-1.185l.159-.291A1.873 1.873 0 0 0 1.945 8.93l-.319-.094c-.835-.246-.835-1.428 0-1.674l.319-.094A1.873 1.873 0 0 0 3.06 4.377l-.16-.292c-.415-.764.42-1.6 1.185-1.184l.292.159a1.873 1.873 0 0 0 2.692-1.115l.094-.319z"/></svg>
          Auto
          <span v-if="inferredLinks.length > 0" class="badge bg-blue-lt ms-1">{{ inferredLinks.length }}</span>
        </button>
      </div>
      <div class="card-body network-topology-body">
        <div v-if="networkTab === 'config'" class="network-config">
          <div class="network-config-row">
            <div class="network-config-item">
              <label class="form-label">Reverse proxy</label>
              <input v-model="rootNodeName" type="text" class="form-control form-control-sm" placeholder="Ex: Nginx Proxy Manager" />
            </div>
            <div class="network-config-item">
              <label class="form-label">IP du proxy</label>
              <input v-model="rootNodeIp" type="text" class="form-control form-control-sm" placeholder="Ex: 192.168.1.10" />
            </div>
            <div class="network-config-item">
              <label class="form-label">Exclure ports (global)</label>
              <input v-model="excludedPortsText" type="text" class="form-control form-control-sm" placeholder="Ex: 22, 2375, 9000" />
              <div class="text-secondary small">Liste separee par virgules</div>
            </div>
          </div>
          <div class="network-config-item">
            <label class="form-label">Nom des services (port=nom)</label>
            <textarea v-model="servicePortMapText" rows="2" class="form-control form-control-sm" placeholder="80=Nginx Proxy Manager&#10;3000=Vaultwarden"></textarea>
          </div>
          <div class="network-config-item mt-3">
            <div class="d-flex align-items-center justify-content-between mb-2">
              <label class="form-label mb-0">Services exposes via proxy</label>
              <button class="btn btn-outline-light btn-sm" @click="addServiceRow">
                Ajouter un service
              </button>
            </div>
            <div class="table-responsive network-config-table">
              <table class="table table-sm table-vcenter">
                <thead>
                  <tr>
                    <th>Nom</th>
                    <th>Domaine</th>
                    <th>Chemin</th>
                    <th>Port interne</th>
                    <th>Port externe</th>
                    <th>Host</th>
                    <th>Tags</th>
                    <th></th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="service in networkServices" :key="service.id">
                    <td><input v-model="service.name" class="form-control form-control-sm" placeholder="Ex: Vaultwarden" /></td>
                    <td><input v-model="service.domain" class="form-control form-control-sm" placeholder="vault.example.com" /></td>
                    <td><input v-model="service.path" class="form-control form-control-sm" placeholder="/" /></td>
                    <td><input v-model.number="service.internalPort" type="number" class="form-control form-control-sm" placeholder="3000" /></td>
                    <td><input v-model.number="service.externalPort" type="number" class="form-control form-control-sm" placeholder="443" /></td>
                    <td>
                      <select v-model="service.hostId" class="form-select form-select-sm">
                        <option value="">Choisir...</option>
                        <option v-for="h in hosts" :key="h.id" :value="h.id">
                          {{ h.name || h.hostname || h.ip_address || h.id }}
                        </option>
                      </select>
                    </td>
                    <td><input v-model="service.tags" class="form-control form-control-sm" placeholder="auth, admin" /></td>
                    <td class="text-end">
                      <button class="btn btn-sm btn-outline-danger" @click="removeServiceRow(service.id)">Supprimer</button>
                    </td>
                  </tr>
                  <tr v-if="networkServices.length === 0">
                    <td colspan="8" class="text-secondary text-center py-3">Aucun service configure</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
          <div class="network-config-item mt-4">
            <div class="d-flex align-items-center justify-content-between mb-2">
              <label class="form-label mb-0">Ports decouverts par host</label>
              <div class="text-secondary small">Nommer, masquer, lier au proxy</div>
            </div>
            <div class="network-discovered">
              <div v-for="host in hosts" :key="host.id" class="network-host-block">
                <div class="network-host-header">
                  <div class="fw-semibold">{{ host.name || host.hostname || host.ip_address || host.id }}</div>
                  <div class="text-secondary small">{{ host.ip_address || 'IP inconnue' }}</div>
                </div>
                <div class="table-responsive network-config-table">
                  <table class="table table-sm table-vcenter">
                    <thead>
                      <tr>
                        <th>Port interne</th>
                        <th>Proto</th>
                        <th>Nom</th>
                        <th>Domaine</th>
                        <th>Chemin</th>
                        <th>Afficher</th>
                        <th>Lier proxy</th>
                      </tr>
                    </thead>
                    <tbody>
                      <tr v-for="port in discoveredPortsByHost[host.id] || []" :key="port.key">
                        <td class="fw-semibold">{{ port.port }}</td>
                        <td class="text-secondary text-uppercase">{{ port.protocol }}</td>
                        <td>
                          <input v-model="getPortSetting(host.id, port.port).name" class="form-control form-control-sm" placeholder="Ex: Vaultwarden" />
                        </td>
                        <td>
                          <input v-model="getPortSetting(host.id, port.port).domain" class="form-control form-control-sm" placeholder="vault.example.com" />
                        </td>
                        <td>
                          <input v-model="getPortSetting(host.id, port.port).path" class="form-control form-control-sm" placeholder="/" />
                        </td>
                        <td>
                          <label class="form-check">
                            <input
                              :id="`port-enabled-${host.id}-${port.port}`"
                              v-model="getPortSetting(host.id, port.port).enabled"
                              class="form-check-input"
                              type="checkbox"
                            />
                            <span class="form-check-label">Afficher</span>
                          </label>
                        </td>
                        <td>
                          <label class="form-check form-switch">
                            <input
                              :id="`port-proxy-${host.id}-${port.port}`"
                              v-model="getPortSetting(host.id, port.port).linkToProxy"
                              class="form-check-input"
                              type="checkbox"
                            />
                            <span class="form-check-label">Proxy</span>
                          </label>
                        </td>
                      </tr>
                      <tr v-if="(discoveredPortsByHost[host.id] || []).length === 0">
                        <td colspan="7" class="text-secondary text-center py-3">Aucun port detecte</td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </div>
            </div>
          </div>
        </div>
        <div v-else-if="networkTab === 'auto'" class="network-config">
          <div class="network-config-item">
            <div class="d-flex align-items-center justify-content-between mb-2">
              <label class="form-label mb-0">Liens decouverts automatiquement</label>
              <span v-if="inferredLinks.length === 0" class="text-secondary small">Aucun lien detecte</span>
              <span v-else class="text-secondary small">{{ inferredLinks.length }} lien(s)</span>
            </div>
            <div v-if="inferredLinks.length === 0" class="text-secondary text-center py-4">
              Les connexions entre conteneurs seront deduites des reseaux Docker, variables d'environnement et configs Traefik.
            </div>
            <div v-else class="table-responsive network-config-table">
              <table class="table table-sm table-vcenter">
                <thead>
                  <tr>
                    <th>Conteneur source</th>
                    <th></th>
                    <th>Conteneur cible</th>
                    <th>Type</th>
                    <th>Confiance</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="link in inferredLinks" :key="`${link.source_container_name}|${link.target_container_name}|${link.link_type}`">
                    <td class="fw-semibold text-truncate" :title="link.source_container_name">{{ link.source_container_name }}</td>
                    <td class="text-center text-secondary">→</td>
                    <td class="fw-semibold text-truncate" :title="link.target_container_name">{{ link.target_container_name }}</td>
                    <td>
                      <span v-if="link.link_type === 'network'" class="badge bg-blue-lt text-blue">Network</span>
                      <span v-else-if="link.link_type === 'env_ref'" class="badge bg-orange-lt text-orange">Env Ref</span>
                      <span v-else-if="link.link_type === 'proxy'" class="badge bg-cyan-lt text-cyan">Proxy</span>
                      <span v-else class="badge bg-secondary-lt text-secondary">{{ link.link_type }}</span>
                    </td>
                    <td>
                      <div class="d-flex align-items-center gap-1">
                        <div class="progress flex-grow-1" style="height: 4px;">
                          <div class="progress-bar" :style="{ width: (link.confidence || 0) + '%', backgroundColor: link.confidence >= 80 ? '#10b981' : link.confidence >= 60 ? '#f59e0b' : '#ef4444' }"></div>
                        </div>
                        <span class="text-secondary small" style="min-width: 30px;">{{ link.confidence || 0 }}%</span>
                      </div>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
        <div v-else ref="graphSurfaceRef" class="network-graph-surface" :style="{ height: graphHeight }">
          <div class="network-topology-toolbar">
            <div class="d-flex align-items-center justify-content-between flex-wrap gap-3">
              <div>
                <div class="text-secondary small mb-1" style="font-weight: 600;">Légende</div>
                <div class="d-flex gap-2 flex-wrap" style="font-size: 0.85rem;">
                  <div class="d-flex align-items-center gap-1">
                    <div style="width: 3px; height: 20px; background: #4f46e5; border-radius: 2px;"></div>
                    <span class="text-secondary">Conteneur</span>
                  </div>
                  <div class="d-flex align-items-center gap-1">
                    <div style="width: 3px; height: 20px; background: #8b5cf6; border-radius: 2px;"></div>
                    <span class="text-secondary">Service</span>
                  </div>
                  <div class="d-flex align-items-center gap-1">
                    <div style="width: 3px; height: 20px; background: #06b6d4; border-radius: 2px;"></div>
                    <span class="text-secondary" v-if="showProxyLinks">Proxy</span>
                    <span class="text-secondary" v-else>Port interne</span>
                  </div>
                </div>
              </div>
              <div>
                <label class="form-check form-switch m-0">
                  <input v-model="showProxyLinks" class="form-check-input" type="checkbox" />
                  <span class="form-check-label">{{ showProxyLinks ? 'Masquer' : 'Afficher' }} Proxy → Service</span>
                </label>
              </div>
            </div>
          </div>
          <NetworkGraph
            :data="graphHosts"
            :root-label="rootNodeName"
            :root-ip="rootNodeIp"
            :service-map="servicePortMap"
            :excluded-ports="excludedPorts"
            :services="combinedServices"
            :host-port-overrides="hostPortOverrides"
            :show-proxy-links="showProxyLinks"
            @host-click="handleHostClick"
          />
        </div>
      </div>
    </div>

    <!-- Cards View (Original) -->
    <template v-if="viewMode === 'cards'">
      <div class="card mb-4">
        <div class="card-body">
          <div class="row g-3">
            <div class="col-md-6 col-lg-3">
              <input v-model="search" type="text" class="form-control" placeholder="Rechercher un port, conteneur, image..." />
            </div>
            <div class="col-md-6 col-lg-3">
              <select v-model="protocolFilter" class="form-select">
                <option value="">Tous les protocoles</option>
                <option value="tcp">TCP</option>
                <option value="udp">UDP</option>
              </select>
            </div>
            <div class="col-md-6 col-lg-3">
              <select v-model="hostFilter" class="form-select">
                <option value="">Tous les hotes</option>
                <option v-for="h in hosts" :key="h.id" :value="h.id">
                  {{ h.name || h.hostname || h.id }}
                </option>
              </select>
            </div>
            <div class="col-md-6 col-lg-3">
              <label class="form-check form-switch">
                <input v-model="onlyPublished" class="form-check-input" type="checkbox" />
                <span class="form-check-label">Ports publies seulement</span>
              </label>
            </div>
          </div>
        </div>
      </div>

      <div class="card mb-4">
        <div class="table-responsive">
          <table class="table table-vcenter card-table">
            <thead>
              <tr>
                <th>Hote</th>
                <th>Conteneur</th>
                <th>Image</th>
                <th>Port hote</th>
                <th>Port conteneur</th>
                <th>Proto</th>
                <th>Bind</th>
                <th>Etat</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in portRows" :key="row.key">
                <td>
                  <router-link :to="`/hosts/${row.host_id}`" class="text-decoration-none">
                    {{ row.host_name || row.host_id }}
                  </router-link>
                </td>
                <td class="fw-semibold">{{ row.container_name }}</td>
                <td>
                  <div>{{ row.image }}</div>
                  <div class="text-secondary small"><code>{{ row.image_tag || '-' }}</code></div>
                </td>
                <td class="fw-semibold">{{ row.host_port || '-' }}</td>
                <td class="text-secondary">{{ row.container_port || '-' }}</td>
                <td class="text-secondary text-uppercase">{{ row.protocol || '-' }}</td>
                <td class="text-secondary small font-monospace">{{ row.host_ip || '-' }}</td>
                <td>
                  <span :class="row.state === 'running' ? 'badge bg-green-lt text-green' : 'badge bg-secondary-lt text-secondary'">
                    {{ row.state || 'unknown' }}
                  </span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-if="portRows.length === 0" class="text-center text-secondary py-4">
          Aucun port visible
        </div>
      </div>

      <div class="card">
        <div class="card-header">
          <h3 class="card-title">Trafic par hote</h3>
        </div>
        <div class="table-responsive">
          <table class="table table-vcenter card-table">
            <thead>
              <tr>
                <th>Hote</th>
                <th>IP</th>
                <th>Rx</th>
                <th>Tx</th>
                <th>Statut</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="h in hosts" :key="h.id">
                <td>
                  <router-link :to="`/hosts/${h.id}`" class="fw-semibold text-decoration-none">
                    {{ h.name || h.hostname || h.id }}
                  </router-link>
                </td>
                <td class="text-secondary">{{ h.ip_address }}</td>
                <td>{{ formatBytes(h.network_rx_bytes || 0) }}</td>
                <td>{{ formatBytes(h.network_tx_bytes || 0) }}</td>
                <td>
                  <span :class="h.status === 'online' ? 'badge bg-green-lt text-green' : h.status === 'warning' ? 'badge bg-yellow-lt text-yellow' : 'badge bg-red-lt text-red'">
                    {{ h.status || 'unknown' }}
                  </span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-if="hosts.length === 0" class="text-center text-secondary py-4">
          Aucun hote trouve
        </div>
      </div>
    </template>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, watch, watchEffect } from 'vue'
import { useRouter } from 'vue-router'
import { useWebSocket } from '../composables/useWebSocket'
import WsStatusBar from '../components/WsStatusBar.vue'
import NetworkGraph from '../components/NetworkGraph.vue'
import apiClient from '../api'

const router = useRouter()
const hosts = ref([])
const containers = ref([])
const networks = ref([])
const search = ref('')
const protocolFilter = ref('')
const hostFilter = ref('')
const onlyPublished = ref(true)
const viewMode = ref(localStorage.getItem('networkViewMode') || 'cards')
const networkTab = ref('topology')
const rootNodeName = ref('Infrastructure')
const rootNodeIp = ref('')
const showProxyLinks = ref(true)
const servicePortMapText = ref('')
const excludedPortsText = ref('')
const networkServices = ref([])
const hostPortConfig = ref([])
const topologyConfigLoaded = ref(false)
const inferredLinks = ref([])
const saveStatus = ref('idle') // 'idle' | 'saving' | 'saved' | 'error'
const graphSurfaceRef = ref(null)
const graphHeight = ref('auto')
const auth = useAuthStore()

// Save view mode to localStorage only (local UI preference)
watch(viewMode, (newMode) => {
  localStorage.setItem('networkViewMode', newMode)
})

// Debounced save function (500ms debounce)
let saveTimeout = null
const debouncedSave = () => {
  if (saveTimeout) clearTimeout(saveTimeout)
  saveTimeout = setTimeout(async () => {
    await saveTopologyConfig()
  }, 500)
}

// Watch for changes and trigger save
watch(rootNodeName, () => debouncedSave())
watch(rootNodeIp, () => debouncedSave())
watch(showProxyLinks, () => debouncedSave())
watch(servicePortMapText, () => debouncedSave())
watch(excludedPortsText, () => debouncedSave())
watch(networkServices, () => debouncedSave(), { deep: true })
watch(hostPortConfig, () => debouncedSave(), { deep: true })

// Load topology configuration from database
async function loadTopologyConfig() {
  try {
    const res = await apiClient.getTopologyConfig()
    if (res.data) {
      const cfg = res.data
      rootNodeName.value = cfg.root_label || 'Infrastructure'
      rootNodeIp.value = cfg.root_ip || ''
      showProxyLinks.value = cfg.show_proxy_links !== false
      networkServices.value = cfg.manual_services ? JSON.parse(cfg.manual_services) : []
      servicePortMapText.value = cfg.service_map && cfg.service_map !== '{}' ? cfg.service_map : ''
      excludedPortsText.value = (cfg.excluded_ports || []).join(', ')
      if (cfg.host_overrides) {
        try {
          hostPortConfig.value = JSON.parse(cfg.host_overrides)
        } catch {
          hostPortConfig.value = []
        }
      }
      topologyConfigLoaded.value = true
    }
  } catch (e) {
    console.warn('Failed to load topology config from server:', e)
    topologyConfigLoaded.value = true
  }
}

// Save topology configuration to database
async function saveTopologyConfig() {
  if (!topologyConfigLoaded.value) return // Don't save until fully loaded
  try {
    saveStatus.value = 'saving'
    const config = {
      root_label: rootNodeName.value,
      root_ip: rootNodeIp.value,
      excluded_ports: excludedPorts.value,
      service_map: servicePortMapText.value || '{}',
      show_proxy_links: showProxyLinks.value,
      host_overrides: JSON.stringify(hostPortConfig.value),
      manual_services: JSON.stringify(networkServices.value)
    }
    await apiClient.saveTopologyConfig(config)
    saveStatus.value = 'saved'
    // Auto-reset to idle after 3 seconds
    setTimeout(() => {
      if (saveStatus.value === 'saved') saveStatus.value = 'idle'
    }, 3000)
  } catch (e) {
    console.warn('Failed to save topology config:', e)
    saveStatus.value = 'error'
    setTimeout(() => {
      if (saveStatus.value === 'error') saveStatus.value = 'idle'
    }, 3000)
  }
}

const servicePortMap = computed(() => {
  const map = {}
  const lines = servicePortMapText.value.split(/\r?\n|,/).map(line => line.trim()).filter(Boolean)
  for (const line of lines) {
    const [portRaw, ...nameParts] = line.split(/[=:]/)
    const port = Number(portRaw?.trim())
    const name = nameParts.join(':').trim()
    if (!port || !name) continue
    map[port] = name
  }
  return map
})

const excludedPorts = computed(() => {
  const values = excludedPortsText.value.split(/\s*,\s*/).map(entry => Number(entry.trim())).filter(Boolean)
  return Array.from(new Set(values))
})

const discoveredPortsByHost = computed(() => {
  const map = {}
  for (const container of containers.value) {
    const mappings = container.port_mappings || []
    for (const mapping of mappings) {
      const hostId = container.host_id
      if (!hostId) continue

      const portNumber = mapping.container_port || mapping.host_port || 0
      if (!portNumber) continue

      const protocol = (mapping.protocol || 'tcp').toLowerCase()
      if (!map[hostId]) map[hostId] = []
      const key = `${portNumber}-${protocol}`
      if (map[hostId].some(entry => entry.key === key)) continue

      map[hostId].push({ key, port: portNumber, protocol })
    }
  }

  for (const host of hosts.value) {
    if (!map[host.id]) map[host.id] = []
  }

  return map
})

const hostPortOverrides = computed(() => {
  const overrides = {}
  for (const entry of hostPortConfig.value) {
    if (!entry.hostId) continue
    const excludedPortsList = []
    const portMap = {}
    for (const [port, settings] of Object.entries(entry.ports || {})) {
      const portNumber = Number(port)
      if (!settings?.enabled) excludedPortsList.push(portNumber)
      if (settings?.name) portMap[portNumber] = settings.name
    }
    overrides[entry.hostId] = { excludedPorts: excludedPortsList, portMap }
  }
  return overrides
})

const portRows = computed(() => {
  const rows = []
  for (const container of containers.value) {
    const mappings = container.port_mappings || []
    for (const mapping of mappings) {
      const hostPort = Number(mapping.host_port || 0)
      const isPublished = hostPort > 0
      if (onlyPublished.value && !isPublished) continue

      rows.push({
        key: `${container.id}-${mapping.raw}`,
        host_id: container.host_id,
        host_name: container.hostname,
        container_name: container.name,
        image: container.image,
        image_tag: container.image_tag,
        state: container.state,
        host_port: hostPort,
        container_port: mapping.container_port,
        protocol: mapping.protocol,
        host_ip: mapping.host_ip,
        raw: mapping.raw,
      })
    }
  }

  const query = search.value.trim().toLowerCase()
  return rows.filter((row) => {
    const matchHost = !hostFilter.value || row.host_id === hostFilter.value
    const matchProto = !protocolFilter.value || row.protocol === protocolFilter.value
    const matchSearch =
      !query ||
      row.container_name?.toLowerCase().includes(query) ||
      row.image?.toLowerCase().includes(query) ||
      row.image_tag?.toLowerCase().includes(query) ||
      row.host_name?.toLowerCase().includes(query) ||
      String(row.host_port || '').includes(query) ||
      String(row.container_port || '').includes(query) ||
      row.protocol?.toLowerCase().includes(query) ||
      row.host_ip?.toLowerCase().includes(query)

    return matchHost && matchProto && matchSearch
  })
})

const totalPorts = computed(() => portRows.value.length)
const totalRx = computed(() => hosts.value.reduce((sum, h) => sum + (h.network_rx_bytes || 0), 0))
const totalTx = computed(() => hosts.value.reduce((sum, h) => sum + (h.network_tx_bytes || 0), 0))

const combinedServices = computed(() => {
  const linkedServices = []
  for (const entry of hostPortConfig.value) {
    if (!entry.hostId) continue
    for (const [port, settings] of Object.entries(entry.ports || {})) {
      if (!settings?.linkToProxy) continue
      const portNumber = Number(port)
      if (!portNumber) continue
      const name = settings.name || `Port ${portNumber}`
      const domain = settings.domain || ''
      const path = settings.path || '/'
      linkedServices.push({
        id: `linked-${entry.hostId}-${portNumber}`,
        name,
        domain,
        path,
        internalPort: portNumber,
        externalPort: null,
        hostId: entry.hostId,
        tags: 'proxy'
      })
    }
  }
  return [...networkServices.value, ...linkedServices]
})


const graphHosts = computed(() => {
  const portsByHost = new Map()
  for (const container of containers.value) {
    const mappings = container.port_mappings || []
    for (const mapping of mappings) {
      const hostId = container.host_id
      if (!hostId) continue

      const portNumber = mapping.host_port || mapping.container_port || 0
      if (!portNumber) continue

      const protocol = (mapping.protocol || 'tcp').toLowerCase()
      const key = `${portNumber}-${protocol}`

      if (!portsByHost.has(hostId)) {
        portsByHost.set(hostId, new Map())
      }

      const hostPorts = portsByHost.get(hostId)
      if (!hostPorts.has(key)) {
        hostPorts.set(key, {
          port: portNumber,
          protocol,
          containers: []
        })
      }

      const entry = hostPorts.get(key)
      entry.containers.push(container.name)
    }
  }

  return hosts.value.map((host) => {
    const hostPorts = portsByHost.get(host.id)
    return {
      ...host,
      ports: hostPorts ? Array.from(hostPorts.values()) : []
    }
  })
})

function formatBytes(bytes) {
  if (!bytes && bytes !== 0) return '-'
  if (bytes < 1024) return `${bytes} B`
  const units = ['KB', 'MB', 'GB', 'TB']
  let value = bytes / 1024
  let idx = 0
  while (value >= 1024 && idx < units.length - 1) {
    value /= 1024
    idx += 1
  }
  return `${value.toFixed(1)} ${units[idx]}`
}

function getPortSetting(hostId, portNumber) {
  const entry = getHostPortEntry(hostId)
  const key = String(portNumber)
  // Return a safe fallback if not yet initialized (ensureHostPortConfig handles init)
  return entry.ports[key] ?? { name: '', domain: '', path: '/', enabled: true, linkToProxy: false }
}

function ensureHostPortConfig() {
  const known = new Set(hostPortConfig.value.map((item) => item.hostId))
  for (const host of hosts.value) {
    if (known.has(host.id)) continue
    hostPortConfig.value.push({ hostId: host.id, ports: {} })
  }
  // Pre-initialize all discovered ports
  for (const [hostId, ports] of Object.entries(discoveredPortsByHost.value)) {
    const entry = getHostPortEntry(hostId)
    for (const port of ports) {
      const portKey = String(port.port)
      if (!entry.ports[portKey]) {
        entry.ports[portKey] = { name: '', domain: '', path: '/', enabled: true, linkToProxy: false }
      }
    }
  }
}

function getHostPortEntry(hostId) {
  let entry = hostPortConfig.value.find((item) => item.hostId === hostId)
  if (!entry) {
    entry = { hostId, ports: {} }
    hostPortConfig.value.push(entry)
  }
  if (!entry.ports) entry.ports = {}
  return entry
}

function addServiceRow() {
  networkServices.value.push({
    id: `svc-${Date.now()}-${Math.floor(Math.random() * 1000)}`,
    name: '',
    domain: '',
    path: '/',
    internalPort: null,
    externalPort: null,
    hostId: '',
    tags: ''
  })
}

function removeServiceRow(serviceId) {
  networkServices.value = networkServices.value.filter((service) => service.id !== serviceId)
}

function handleHostClick(hostId) {
  router.push(`/hosts/${hostId}`)
}

async function fetchSnapshot() {
  try {
    const res = await apiClient.getNetworkSnapshot()
    hosts.value = res.data?.hosts || []
    containers.value = res.data?.containers || []
    ensureHostPortConfig()
  } catch (e) {
    // ignore
  }
}

// Setup ResizeObserver with watchEffect to handle dynamic mounting/unmounting
let resizeObserver = null
watchEffect(() => {
  if (resizeObserver) {
    resizeObserver.disconnect()
    resizeObserver = null
  }
  if (graphSurfaceRef.value) {
    resizeObserver = new ResizeObserver(() => {
      const rect = graphSurfaceRef.value?.getBoundingClientRect()
      if (rect) {
        const availableHeight = window.innerHeight - rect.top - 20
        graphHeight.value = Math.max(400, availableHeight) + 'px'
      }
    })
    resizeObserver.observe(graphSurfaceRef.value)
  }
})

const { wsStatus, wsError, retryCount, reconnect } = useWebSocket('/api/v1/ws/network', (payload) => {
  if (payload.type !== 'network') return
  hosts.value = payload.hosts || []
  containers.value = payload.containers || []
  networks.value = payload.networks || []
  inferredLinks.value = payload.links || []
  
  // Config is loaded only via REST API (loadTopologyConfig), not from WebSocket
  
  ensureHostPortConfig()
})

onMounted(async () => {
  // Load topology config from server first
  await loadTopologyConfig()
  // Then fetch snapshot to populate real hosts/containers
  await fetchSnapshot()
})

onUnmounted(() => {
  if (resizeObserver) {
    resizeObserver.disconnect()
  }
})
</script>

<style scoped>
.network-topology-card {
  overflow: hidden;
}

.network-subnav {
  display: flex;
  gap: 8px;
  padding: 14px 18px 0;
  background: rgba(15, 23, 42, 0.45);
}

.network-topology-body {
  height: auto;
  min-height: 600px;
  padding: 0;
  display: flex;
  flex-direction: column;
}

.network-config {
  padding: 16px 18px 24px;
  border-bottom: 1px solid rgba(148, 163, 184, 0.2);
  background: rgba(15, 23, 42, 0.45);
  overflow-y: auto;
  max-height: calc(100vh - 260px);
}

.network-config-row {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
  margin-bottom: 12px;
  align-items: start;
}

@media (max-width: 1400px) {
  .network-config-row {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 900px) {
  .network-config-row {
    grid-template-columns: 1fr;
  }
}

.network-config-item .form-label {
  font-size: 12px;
  color: #cbd5f5;
}

.network-config-item textarea,
.network-config-item input:not([type="checkbox"]):not([type="radio"]) {
  background: rgba(15, 23, 42, 0.7);
  border: 1px solid rgba(148, 163, 184, 0.4);
  color: #e2e8f0;
}

.network-config-table {
  border: 1px solid rgba(148, 163, 184, 0.2);
  border-radius: 12px;
  overflow: hidden;
  background: rgba(15, 23, 42, 0.55);
}

.network-config-table table {
  margin: 0;
}

.network-config-table thead th {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.4px;
  color: #94a3b8;
  background: rgba(15, 23, 42, 0.7);
  border-bottom: 1px solid rgba(148, 163, 184, 0.2);
}

.network-config-table tbody td {
  border-top: 1px solid rgba(148, 163, 184, 0.1);
  vertical-align: middle;
}

.network-config-table .form-control,
.network-config-table .form-select {
  background: rgba(15, 23, 42, 0.6);
  border: 1px solid rgba(148, 163, 184, 0.3);
  color: #e2e8f0;
}

.network-config-item textarea::placeholder,
.network-config-item input::placeholder {
  color: rgba(226, 232, 240, 0.55);
}

.network-graph-surface {
  flex: 1;
  min-height: 400px;
  padding: 16px 18px 18px;
  display: flex;
  flex-direction: column;
  overflow-y: auto;
  overflow-x: hidden;
}

.network-topology-toolbar {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 12px;
  padding: 0 4px 12px;
  color: #cbd5f5;
}

@media (max-width: 991px) {
  .network-topology-body {
    min-height: 420px;
  }

  .network-graph-surface {
    height: 52vh;
  }

  .network-config-row {
    grid-template-columns: 1fr;
  }
}
</style>
