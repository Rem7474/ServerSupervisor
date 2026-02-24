<template>
  <div>
    <div class="page-header mb-4">
      <h2 class="page-title">Docker</h2>
      <div class="text-secondary">Vue globale de tous les conteneurs sur l'infrastructure</div>
    </div>

    <WsStatusBar :status="wsStatus" :error="wsError" :retry-count="retryCount" @reconnect="reconnect" />

    <!-- Tabs -->
    <ul class="nav nav-tabs mb-4">
      <li class="nav-item">
        <a class="nav-link" :class="{ active: activeTab === 'containers' }" href="#" @click.prevent="activeTab = 'containers'">
          Conteneurs
          <span class="badge bg-secondary text-white ms-1">
            {{ filteredContainers.length }}<template v-if="filteredContainers.length !== containers.length"> / {{ containers.length }}</template>
          </span>
        </a>
      </li>
      <li class="nav-item">
        <a class="nav-link" :class="{ active: activeTab === 'compose' }" href="#" @click.prevent="activeTab = 'compose'">
          Projets Compose
          <span class="badge bg-secondary text-white ms-1">
            {{ filteredComposeProjects.length }}<template v-if="filteredComposeProjects.length !== composeProjects.length"> / {{ composeProjects.length }}</template>
          </span>
        </a>
      </li>
    </ul>

    <!-- ===== TAB CONTENEURS ===== -->
    <div v-if="activeTab === 'containers'">
      <div class="card mb-4">
        <div class="card-body">
          <div class="row g-3">
            <div class="col-md-6 col-lg-2">
              <input v-model="search" type="text" class="form-control" placeholder="Rechercher..." />
            </div>
            <div class="col-md-6 col-lg-2">
              <select v-model="hostFilter" class="form-select">
                <option value="">Tous les hôtes</option>
                <option v-for="h in uniqueHosts" :key="h" :value="h">{{ h }}</option>
              </select>
            </div>
            <div class="col-md-6 col-lg-2">
              <select v-model="stateFilter" class="form-select">
                <option value="">Tous les états</option>
                <option value="running">Running</option>
                <option value="restarting">Restarting</option>
                <option value="paused">Paused</option>
                <option value="created">Created</option>
                <option value="exited">Exited</option>
                <option value="dead">Dead</option>
              </select>
            </div>
            <div class="col-md-6 col-lg-2">
              <select v-model="composeFilter" class="form-select">
                <option value="">Tous les conteneurs</option>
                <option value="compose">Docker Compose</option>
                <option value="standalone">Standalone</option>
              </select>
            </div>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="table-responsive">
          <table class="table table-vcenter card-table">
            <thead>
              <tr>
                <th>Nom</th>
                <th>Hôte</th>
                <th>Compose</th>
                <th>Image</th>
                <th>État</th>
                <th>Ports</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="c in filteredContainers" :key="c.id">
                <td class="fw-semibold">{{ c.name }}</td>
                <td>
                  <router-link :to="`/hosts/${c.host_id}`" class="text-decoration-none">
                    {{ c.hostname }}
                  </router-link>
                </td>
                <td>
                  <div v-if="getComposeInfo(c).project" class="small">
                    <div class="text-primary fw-semibold">{{ getComposeInfo(c).project }}</div>
                    <div class="text-secondary">{{ getComposeInfo(c).service }}</div>
                  </div>
                  <span v-else class="text-secondary">-</span>
                </td>
                <td class="small">{{ c.image }}</td>
                <td>
                  <span :class="stateClass(c.state)">{{ c.state }}</span>
                </td>
                <td class="d-none d-sm-table-cell text-secondary small font-monospace">{{ formatContainerPorts(c.ports) }}</td>
                <td class="text-end">
                  <div class="d-flex align-items-center justify-content-end gap-1">
                    <!-- Action buttons: admin/operator only -->
                    <template v-if="canRunDocker">
                      <!-- Start: for stopped containers -->
                      <button
                        v-if="['exited', 'dead', 'created', 'paused'].includes(c.state)"
                        @click="sendContainerAction(c, 'start')"
                        :disabled="!!dockerActionLoading[c.name]"
                        class="btn btn-sm btn-ghost-success"
                        title="Démarrer"
                      >
                        <span v-if="dockerActionLoading[c.name] === 'start'" class="spinner-border spinner-border-sm"></span>
                        <svg v-else xmlns="http://www.w3.org/2000/svg" class="icon icon-sm" width="16" height="16" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none"><path stroke="none" d="M0 0h24v24H0z" fill="none"/><path d="M7 4v16l13 -8z" /></svg>
                      </button>
                      <!-- Stop: for running containers -->
                      <button
                        v-if="c.state === 'running'"
                        @click="sendContainerAction(c, 'stop')"
                        :disabled="!!dockerActionLoading[c.name]"
                        class="btn btn-sm btn-ghost-danger"
                        title="Arrêter"
                      >
                        <span v-if="dockerActionLoading[c.name] === 'stop'" class="spinner-border spinner-border-sm"></span>
                        <svg v-else xmlns="http://www.w3.org/2000/svg" class="icon icon-sm" width="16" height="16" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none"><path stroke="none" d="M0 0h24v24H0z" fill="none"/><rect x="4" y="4" width="16" height="16" rx="2" /></svg>
                      </button>
                      <!-- Restart: for running containers -->
                      <button
                        v-if="c.state === 'running'"
                        @click="sendContainerAction(c, 'restart')"
                        :disabled="!!dockerActionLoading[c.name]"
                        class="btn btn-sm btn-ghost-warning"
                        title="Redémarrer"
                      >
                        <span v-if="dockerActionLoading[c.name] === 'restart'" class="spinner-border spinner-border-sm"></span>
                        <svg v-else xmlns="http://www.w3.org/2000/svg" class="icon icon-sm" width="16" height="16" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none"><path stroke="none" d="M0 0h24v24H0z" fill="none"/><path d="M20 11a8.1 8.1 0 0 0 -15.5 -2m-.5 -4v4h4" /><path d="M4 13a8.1 8.1 0 0 0 15.5 2m.5 4v-4h-4" /></svg>
                      </button>
                      <!-- Logs: always available -->
                      <button
                        @click="sendContainerAction(c, 'logs')"
                        :disabled="!!dockerActionLoading[c.name]"
                        class="btn btn-sm btn-ghost-secondary"
                        title="Voir les logs"
                      >
                        <span v-if="dockerActionLoading[c.name] === 'logs'" class="spinner-border spinner-border-sm"></span>
                        <svg v-else xmlns="http://www.w3.org/2000/svg" class="icon icon-sm" width="16" height="16" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none"><path stroke="none" d="M0 0h24v24H0z" fill="none"/><path d="M4 6l16 0" /><path d="M4 12l16 0" /><path d="M4 18l12 0" /></svg>
                      </button>
                    </template>

                    <!-- Inspect button -->
                    <button
                      @click="inspectTarget = c; inspectTab = 'env'"
                      class="btn btn-sm btn-ghost-secondary"
                      title="Inspecter"
                    >
                      <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-sm" width="16" height="16" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none"><path stroke="none" d="M0 0h24v24H0z" fill="none"/><circle cx="10" cy="10" r="7" /><path d="M21 21l-6 -6" /></svg>
                    </button>

                    <!-- Compose details button -->
                    <button
                      v-if="getComposeInfo(c).project || Object.keys(c.labels || {}).length > 0"
                      @click="selectedContainer = c"
                      class="btn btn-sm btn-ghost-secondary"
                      :title="getComposeInfo(c).project ? 'Infos Compose + Labels' : 'Labels'"
                    >
                      <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-sm" width="16" height="16"
                           viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none">
                        <path stroke="none" d="M0 0h24v24H0z" fill="none"/>
                        <path d="M9 5H7a2 2 0 0 0 -2 2v12a2 2 0 0 0 2 2h10a2 2 0 0 0 2 -2V7a2 2 0 0 0 -2 -2h-2"/>
                        <path d="M9 5a2 2 0 0 0 2 2h2a2 2 0 0 0 2 -2a2 2 0 0 0 -2 -2h-2a2 2 0 0 0 -2 2z"/>
                        <path d="M9 12l.01 0"/><path d="M13 12l2 0"/><path d="M9 16l.01 0"/><path d="M13 16l2 0"/>
                      </svg>
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
      <div v-if="filteredContainers.length === 0" class="text-center text-secondary py-4">
        Aucun conteneur trouvé
      </div>

    </div>

    <!-- ===== TAB PROJETS COMPOSE ===== -->
    <div v-if="activeTab === 'compose'">
      <div class="card mb-4">
        <div class="card-body">
          <div class="row g-3">
            <div class="col-md-6 col-lg-4">
              <input v-model="composeSearch" type="text" class="form-control" placeholder="Rechercher un projet..." />
            </div>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="table-responsive">
          <table class="table table-vcenter card-table">
            <thead>
              <tr>
                <th>Projet</th>
                <th>Hote</th>
                <th>Services</th>
                <th>Fichier de config</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="p in filteredComposeProjects" :key="p.id">
                <td class="fw-semibold">{{ p.name }}</td>
                <td>
                  <router-link :to="`/hosts/${p.host_id}`" class="text-decoration-none">
                    {{ p.hostname }}
                  </router-link>
                </td>
                <td>
                  <div class="d-flex flex-wrap gap-1">
                    <span v-for="svc in p.services" :key="svc" class="badge bg-blue-lt text-blue">
                      {{ svc }}
                    </span>
                    <span v-if="!p.services || p.services.length === 0" class="text-secondary">-</span>
                  </div>
                </td>
                <td class="font-monospace small text-secondary">{{ p.config_file || p.working_dir || '-' }}</td>
                <td class="text-end">
                  <div class="d-flex align-items-center justify-content-end gap-1">
                    <template v-if="canRunDocker">
                      <button
                        @click="sendComposeAction(p, 'compose_up')"
                        :disabled="!!composeActionLoading[p.name]"
                        class="btn btn-sm btn-ghost-success"
                        title="Start (up -d)"
                      >
                        <span v-if="composeActionLoading[p.name] === 'compose_up'" class="spinner-border spinner-border-sm"></span>
                        <svg v-else xmlns="http://www.w3.org/2000/svg" class="icon icon-sm" width="16" height="16" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none"><path stroke="none" d="M0 0h24v24H0z" fill="none"/><path d="M7 4v16l13 -8z" /></svg>
                      </button>
                      <button
                        @click="sendComposeAction(p, 'compose_down')"
                        :disabled="!!composeActionLoading[p.name]"
                        class="btn btn-sm btn-ghost-danger"
                        title="Stop (down)"
                      >
                        <span v-if="composeActionLoading[p.name] === 'compose_down'" class="spinner-border spinner-border-sm"></span>
                        <svg v-else xmlns="http://www.w3.org/2000/svg" class="icon icon-sm" width="16" height="16" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none"><path stroke="none" d="M0 0h24v24H0z" fill="none"/><rect x="4" y="4" width="16" height="16" rx="2" /></svg>
                      </button>
                      <button
                        @click="sendComposeAction(p, 'compose_restart')"
                        :disabled="!!composeActionLoading[p.name]"
                        class="btn btn-sm btn-ghost-warning"
                        title="Redémarrer"
                      >
                        <span v-if="composeActionLoading[p.name] === 'compose_restart'" class="spinner-border spinner-border-sm"></span>
                        <svg v-else xmlns="http://www.w3.org/2000/svg" class="icon icon-sm" width="16" height="16" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none"><path stroke="none" d="M0 0h24v24H0z" fill="none"/><path d="M20 11a8.1 8.1 0 0 0 -15.5 -2m-.5 -4v4h4" /><path d="M4 13a8.1 8.1 0 0 0 15.5 2m.5 4v-4h-4" /></svg>
                      </button>
                      <button
                        @click="sendComposeAction(p, 'compose_logs')"
                        :disabled="!!composeActionLoading[p.name]"
                        class="btn btn-sm btn-ghost-secondary"
                        title="Voir les logs"
                      >
                        <span v-if="composeActionLoading[p.name] === 'compose_logs'" class="spinner-border spinner-border-sm"></span>
                        <svg v-else xmlns="http://www.w3.org/2000/svg" class="icon icon-sm" width="16" height="16" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none"><path stroke="none" d="M0 0h24v24H0z" fill="none"/><path d="M4 6l16 0" /><path d="M4 12l16 0" /><path d="M4 18l12 0" /></svg>
                      </button>
                    </template>
                    <button @click="selectedProject = p" class="btn btn-sm btn-ghost-secondary" title="Config">
                      <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-sm" width="16" height="16" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                        <path stroke="none" d="M0 0h24v24H0z" fill="none"/>
                        <path d="M14 3v4a1 1 0 0 0 1 1h4" />
                        <path d="M17 21h-10a2 2 0 0 1 -2 -2v-14a2 2 0 0 1 2 -2h7l5 5v11a2 2 0 0 1 -2 2z" />
                        <path d="M9 9l1 0" /><path d="M9 13l6 0" /><path d="M9 17l6 0" />
                      </svg>
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-if="filteredComposeProjects.length === 0" class="text-center text-secondary py-4">
          Aucun projet Compose trouvé
        </div>
      </div>
    </div>

    <!-- ===== Console Docker Live (partagée entre les tabs) ===== -->
    <div v-if="dockerLiveCmd" class="card mt-4">
      <div class="card-header d-flex align-items-center justify-content-between" style="background:#1e293b; border-bottom: 1px solid rgba(255,255,255,0.08);">
        <div class="d-flex align-items-center gap-3">
          <svg xmlns="http://www.w3.org/2000/svg" class="icon text-secondary" width="20" height="20" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none"><path stroke="none" d="M0 0h24v24H0z" fill="none"/><path d="M8 9l3 3l-3 3" /><path d="M13 15l3 0" /><path d="M3 4m0 2a2 2 0 0 1 2 -2h14a2 2 0 0 1 2 2v12a2 2 0 0 1 -2 2h-14a2 2 0 0 1 -2 -2z" /></svg>
          <div>
            <span class="fw-semibold text-light">{{ dockerLiveCmd.containerName }}</span>
            <code class="ms-2 small" style="background:rgba(0,0,0,0.3);padding:0.1rem 0.4rem;border-radius:0.25rem;color:#94a3b8;">{{ dockerLiveCmd.action }}</code>
          </div>
          <span
            class="badge"
            :class="{
              'bg-yellow-lt text-yellow': dockerLiveCmd.status === 'running' || dockerLiveCmd.status === 'pending',
              'bg-green-lt text-green': dockerLiveCmd.status === 'completed',
              'bg-red-lt text-red': dockerLiveCmd.status === 'failed'
            }"
          >{{ dockerLiveCmd.status }}</span>
        </div>
        <button class="btn btn-sm btn-ghost-secondary" @click="closeDockerConsole" title="Fermer">
          <svg xmlns="http://www.w3.org/2000/svg" class="icon" width="16" height="16" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none"><path stroke="none" d="M0 0h24v24H0z" fill="none"/><path d="M18 6l-12 12" /><path d="M6 6l12 12" /></svg>
        </button>
      </div>
      <pre
        ref="dockerConsoleOutput"
        class="m-0 font-monospace small"
        style="background:#1e1e2e;color:#cdd6f4;max-height:400px;min-height:120px;overflow-y:auto;white-space:pre-wrap;padding:1rem;border-radius:0 0 0.5rem 0.5rem;"
      >{{ dockerConsoleText || '(en attente de sortie...)' }}</pre>
    </div>

    <!-- Modal conteneur (labels) -->
    <div v-if="selectedContainer" class="modal modal-blur fade show" style="display: block;" @click.self="selectedContainer = null">
      <div class="modal-dialog modal-lg modal-dialog-centered">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">Details Docker Compose</h5>
            <button type="button" class="btn-close" @click="selectedContainer = null"></button>
          </div>
          <div class="modal-body">
            <div class="mb-3">
              <label class="form-label fw-semibold">Conteneur</label>
              <div>{{ selectedContainer.name }}</div>
            </div>
            <div class="mb-3">
              <label class="form-label fw-semibold">Projet Compose</label>
              <div>{{ getComposeInfo(selectedContainer).project || '-' }}</div>
            </div>
            <div class="mb-3">
              <label class="form-label fw-semibold">Service</label>
              <div>{{ getComposeInfo(selectedContainer).service || '-' }}</div>
            </div>
            <div class="mb-3">
              <label class="form-label fw-semibold">Repertoire de travail</label>
              <div class="font-monospace small">{{ getComposeInfo(selectedContainer).workingDir || '-' }}</div>
            </div>
            <div class="mb-3">
              <label class="form-label fw-semibold">Fichiers de configuration</label>
              <div class="font-monospace small">{{ getComposeInfo(selectedContainer).configFiles || '-' }}</div>
            </div>
            <div v-if="Object.keys(selectedContainer.labels || {}).length > 0" class="mb-3">
              <label class="form-label fw-semibold">Labels</label>
              <div class="border rounded p-2 bg-dark small font-monospace" style="max-height: 200px; overflow-y: auto;">
                <div v-for="(value, key) in selectedContainer.labels" :key="key" class="mb-1">
                  <span class="text-muted">{{ key }}:</span> <span class="text-light">{{ value }}</span>
                </div>
              </div>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn" @click="selectedContainer = null">Fermer</button>
          </div>
        </div>
      </div>
    </div>
    <div v-if="selectedContainer" class="modal-backdrop fade show"></div>

    <!-- Modal Inspection (env vars / volumes / networks) -->
    <div v-if="inspectTarget" class="modal modal-blur fade show" style="display: block;" @click.self="inspectTarget = null">
      <div class="modal-dialog modal-lg modal-dialog-centered">
        <div class="modal-content">
          <div class="modal-header">
            <div>
              <h5 class="modal-title">{{ inspectTarget.name }}</h5>
              <div class="text-secondary small">
                {{ inspectTarget.image }}:{{ inspectTarget.image_tag }}
                <span class="ms-2" :class="stateClass(inspectTarget.state)">{{ inspectTarget.state }}</span>
              </div>
            </div>
            <button type="button" class="btn-close" @click="inspectTarget = null"></button>
          </div>
          <div class="modal-body p-0">
            <!-- Tabs -->
            <div class="border-bottom px-3">
              <ul class="nav nav-tabs nav-tabs-alt">
                <li class="nav-item">
                  <a class="nav-link" :class="{ active: inspectTab === 'env' }" href="#" @click.prevent="inspectTab = 'env'">
                    Env Vars
                    <span class="badge bg-azure-lt text-azure ms-1">{{ Object.keys(inspectTarget.env_vars || {}).length }}</span>
                  </a>
                </li>
                <li class="nav-item">
                  <a class="nav-link" :class="{ active: inspectTab === 'volumes' }" href="#" @click.prevent="inspectTab = 'volumes'">
                    Volumes
                    <span class="badge bg-azure-lt text-azure ms-1">{{ (inspectTarget.volumes || []).length }}</span>
                  </a>
                </li>
                <li class="nav-item">
                  <a class="nav-link" :class="{ active: inspectTab === 'networks' }" href="#" @click.prevent="inspectTab = 'networks'">
                    Réseaux
                    <span class="badge bg-azure-lt text-azure ms-1">{{ (inspectTarget.networks || []).length }}</span>
                  </a>
                </li>
              </ul>
            </div>
            <div class="p-3" style="min-height: 200px; max-height: 400px; overflow-y: auto;">
              <!-- Env Vars tab -->
              <div v-if="inspectTab === 'env'">
                <div v-if="Object.keys(inspectTarget.env_vars || {}).length === 0" class="text-secondary text-center py-3">
                  Aucune variable d'environnement (non sensible) disponible
                </div>
                <table v-else class="table table-sm table-vcenter">
                  <thead>
                    <tr>
                      <th>Variable</th>
                      <th>Valeur</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="(val, key) in inspectTarget.env_vars" :key="key">
                      <td class="font-monospace small fw-semibold">{{ key }}</td>
                      <td class="font-monospace small text-secondary" style="word-break: break-all;">{{ val }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
              <!-- Volumes tab -->
              <div v-if="inspectTab === 'volumes'">
                <div v-if="!(inspectTarget.volumes || []).length" class="text-secondary text-center py-3">
                  Aucun volume monté
                </div>
                <ul v-else class="list-unstyled mb-0">
                  <li v-for="vol in inspectTarget.volumes" :key="vol" class="py-1 border-bottom font-monospace small">
                    {{ vol }}
                  </li>
                </ul>
              </div>
              <!-- Networks tab -->
              <div v-if="inspectTab === 'networks'">
                <div v-if="!(inspectTarget.networks || []).length" class="text-secondary text-center py-3">
                  Aucun réseau connecté
                </div>
                <div v-else class="d-flex flex-wrap gap-2 pt-1">
                  <span v-for="net in inspectTarget.networks" :key="net" class="badge bg-blue-lt text-blue fs-6">
                    {{ net }}
                  </span>
                </div>
              </div>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn" @click="inspectTarget = null">Fermer</button>
          </div>
        </div>
      </div>
    </div>
    <div v-if="inspectTarget" class="modal-backdrop fade show"></div>

    <!-- Modal projet compose (raw config) -->
    <div v-if="selectedProject" class="modal modal-blur fade show" style="display: block;" @click.self="selectedProject = null">
      <div class="modal-dialog modal-xl modal-dialog-centered modal-dialog-scrollable">
        <div class="modal-content">
          <div class="modal-header">
            <div>
              <h5 class="modal-title">{{ selectedProject.name }}</h5>
              <div class="text-secondary small font-monospace mt-1">
                {{ selectedProject.config_file || selectedProject.working_dir || '-' }}
              </div>
            </div>
            <button type="button" class="btn-close" @click="selectedProject = null"></button>
          </div>
          <div class="modal-body p-0">
            <div class="row g-0">
              <!-- Infos projet -->
              <div class="col-md-3 border-end p-3">
                <div class="mb-3">
                  <div class="text-secondary small fw-semibold text-uppercase mb-1">Hôte</div>
                  <div>{{ selectedProject.hostname }}</div>
                </div>
                <div class="mb-3">
                  <div class="text-secondary small fw-semibold text-uppercase mb-1">Répertoire</div>
                  <div class="font-monospace small text-break">{{ selectedProject.working_dir || '-' }}</div>
                </div>
                <div class="mb-3">
                  <div class="text-secondary small fw-semibold text-uppercase mb-1">Fichier</div>
                  <div class="font-monospace small text-break">{{ selectedProject.config_file || '-' }}</div>
                </div>
                <div>
                  <div class="text-secondary small fw-semibold text-uppercase mb-1">Services ({{ (selectedProject.services || []).length }})</div>
                  <div class="d-flex flex-wrap gap-1">
                    <span v-for="svc in selectedProject.services" :key="svc" class="badge bg-blue-lt text-blue">{{ svc }}</span>
                    <span v-if="!selectedProject.services || selectedProject.services.length === 0" class="text-secondary small">-</span>
                  </div>
                </div>
              </div>
              <!-- Raw config YAML -->
              <div class="col-md-9">
                <div class="d-flex align-items-center justify-content-between px-3 pt-3 pb-2 border-bottom">
                  <span class="text-secondary small fw-semibold">docker compose config (résolu)</span>
                  <button
                    :class="['btn', 'btn-sm', copied ? 'btn-success' : 'btn-ghost-secondary']"
                    @click="copyConfig(selectedProject.raw_config)">
                    {{ copied ? '✓ Copié' : 'Copier' }}
                  </button>
                </div>
                <pre v-if="selectedProject.raw_config" class="m-0 p-3 small" style="max-height: 60vh; overflow-y: auto; background: #1e1e2e; color: #cdd6f4; border-radius: 0 0 4px 0;">{{ selectedProject.raw_config }}</pre>
                <div v-else class="p-4 text-secondary text-center">
                  Config non disponible (agent trop ancien ou docker compose introuvable)
                </div>
              </div>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn" @click="selectedProject = null">Fermer</button>
          </div>
        </div>
      </div>
    </div>
    <div v-if="selectedProject" class="modal-backdrop fade show"></div>
  </div>
</template>

<script setup>
import { ref, computed, watch, nextTick, onUnmounted } from 'vue'
import { useWebSocket } from '../composables/useWebSocket'
import { useAuthStore } from '../stores/auth'
import WsStatusBar from '../components/WsStatusBar.vue'
import apiClient from '../api'

const auth = useAuthStore()

const containers = ref([])
const composeProjects = ref([])
const search = ref('')
const stateFilter = ref('')
const hostFilter = ref('')
const composeFilter = ref('')
const composeSearch = ref('')
const selectedContainer = ref(null)
const selectedProject = ref(null)
const activeTab = ref(localStorage.getItem('dockerActiveTab') || 'containers')
const copied = ref(false)

// Inspect modal
const inspectTarget = ref(null)
const inspectTab = ref('env')

// Docker console
const dockerLiveCmd = ref(null) // { commandId, containerName, action, status }
const dockerConsoleText = ref('')
const dockerConsoleOutput = ref(null)
let dockerStreamWs = null

const canRunDocker = computed(() => auth.role === 'admin' || auth.role === 'operator')

// Action loading state keyed by container name
const dockerActionLoading = ref({})
// Action loading state keyed by compose project name
const composeActionLoading = ref({})

// Persist active tab to localStorage
watch(activeTab, (newTab) => {
  localStorage.setItem('dockerActiveTab', newTab)
})

const getComposeInfo = (container) => {
  if (!container.labels) return {}
  return {
    project: container.labels['com.docker.compose.project'] || '',
    service: container.labels['com.docker.compose.service'] || '',
    workingDir: container.labels['com.docker.compose.project.working_dir'] || '',
    configFiles: container.labels['com.docker.compose.project.config_files'] || ''
  }
}

const isComposeContainer = (container) => {
  return !!container.labels?.['com.docker.compose.project']
}

const stateClass = (state) => {
  const map = {
    running:    'badge bg-green-lt text-green',
    restarting: 'badge bg-yellow-lt text-yellow',
    paused:     'badge bg-yellow-lt text-yellow',
    created:    'badge bg-blue-lt text-blue',
    exited:     'badge bg-secondary-lt text-secondary',
    dead:       'badge bg-red-lt text-red',
    removing:   'badge bg-orange-lt text-orange',
  }
  return map[state] || 'badge bg-secondary-lt text-secondary'
}

const formatContainerPorts = (raw) => {
  if (!raw) return '-'
  const hostPorts = new Set()
  const matches = raw.matchAll(/(\d+\.\d+\.\d+\.\d+|:::?):(\d+)->/g)
  for (const m of matches) hostPorts.add(m[2])
  return hostPorts.size > 0 ? [...hostPorts].join(', ') : raw.split(',').slice(0, 2).join(', ')
}

const copyConfig = async (text) => {
  if (!text) return
  await navigator.clipboard.writeText(text)
  copied.value = true
  setTimeout(() => { copied.value = false }, 2000)
}

const filteredContainers = computed(() => {
  return containers.value.filter(c => {
    const matchSearch = !search.value ||
      c.name?.toLowerCase().includes(search.value.toLowerCase()) ||
      c.image?.toLowerCase().includes(search.value.toLowerCase()) ||
      getComposeInfo(c).project?.toLowerCase().includes(search.value.toLowerCase())
    const matchState = !stateFilter.value || c.state === stateFilter.value
    const matchCompose = !composeFilter.value ||
      (composeFilter.value === 'compose' && isComposeContainer(c)) ||
      (composeFilter.value === 'standalone' && !isComposeContainer(c))
    const matchHost = !hostFilter.value || c.hostname === hostFilter.value
    return matchSearch && matchState && matchCompose && matchHost
  })
})

const uniqueHosts = computed(() => {
  const seen = new Set()
  return containers.value
    .filter(c => { if (seen.has(c.hostname)) return false; seen.add(c.hostname); return true })
    .map(c => c.hostname)
    .sort()
})

const filteredComposeProjects = computed(() => {
  if (!composeSearch.value) return composeProjects.value
  const q = composeSearch.value.toLowerCase()
  return composeProjects.value.filter(p =>
    p.name?.toLowerCase().includes(q) ||
    p.hostname?.toLowerCase().includes(q) ||
    p.config_file?.toLowerCase().includes(q) ||
    p.working_dir?.toLowerCase().includes(q)
  )
})

// ===== Docker Actions =====

async function sendContainerAction(container, action) {
  if (dockerActionLoading.value[container.name]) return

  if ((action === 'stop' || action === 'restart') &&
      !confirm(`Confirmer : ${action} du conteneur « ${container.name} » ?`)) {
    return
  }

  dockerActionLoading.value = { ...dockerActionLoading.value, [container.name]: action }

  try {
    const res = await apiClient.sendDockerCommand(container.host_id, container.name, action)
    const commandId = res.data.command_id
    connectDockerStream(commandId, container.name, action)
  } catch (err) {
    console.error('Docker action failed:', err)
    alert(`Erreur : ${err.response?.data?.error || err.message}`)
  } finally {
    dockerActionLoading.value = { ...dockerActionLoading.value, [container.name]: null }
  }
}

async function sendComposeAction(project, action) {
  if (composeActionLoading.value[project.name]) return

  if ((action === 'compose_down' || action === 'compose_restart') &&
      !confirm(`Confirmer : ${action.replace('compose_', '')} du projet « ${project.name} » ?`)) {
    return
  }

  composeActionLoading.value = { ...composeActionLoading.value, [project.name]: action }

  try {
    const res = await apiClient.sendDockerCommand(
      project.host_id,
      project.name,
      action,
      project.working_dir || ''
    )
    const commandId = res.data.command_id
    connectDockerStream(commandId, project.name, action)
  } catch (err) {
    console.error('Compose action failed:', err)
    alert(`Erreur : ${err.response?.data?.error || err.message}`)
  } finally {
    composeActionLoading.value = { ...composeActionLoading.value, [project.name]: null }
  }
}

function connectDockerStream(commandId, containerName, action) {
  if (dockerStreamWs) {
    dockerStreamWs.close()
    dockerStreamWs = null
  }

  dockerConsoleText.value = ''
  dockerLiveCmd.value = { commandId, containerName, action, status: 'pending' }

  const token = auth.token
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  const wsUrl = `${proto}://${location.host}/api/v1/ws/apt/stream/${commandId}`

  dockerStreamWs = new WebSocket(wsUrl)

  dockerStreamWs.onopen = () => {
    dockerStreamWs.send(JSON.stringify({ type: 'auth', token }))
  }

  dockerStreamWs.onmessage = (event) => {
    try {
      const msg = JSON.parse(event.data)
      if (msg.type === 'apt_stream_init') {
        if (msg.output) dockerConsoleText.value = msg.output
        if (dockerLiveCmd.value) dockerLiveCmd.value.status = msg.status
      } else if (msg.type === 'apt_stream') {
        dockerConsoleText.value += msg.chunk || ''
        scrollDockerConsole()
      } else if (msg.type === 'apt_status_update') {
        if (dockerLiveCmd.value) dockerLiveCmd.value.status = msg.status
        if (msg.status === 'completed' || msg.status === 'failed') {
          setTimeout(() => {
            if (dockerStreamWs) { dockerStreamWs.close(); dockerStreamWs = null }
          }, 500)
        }
      }
    } catch {}
  }

  dockerStreamWs.onerror = () => {
    if (dockerLiveCmd.value) dockerLiveCmd.value.status = 'failed'
  }
}

function closeDockerConsole() {
  if (dockerStreamWs) { dockerStreamWs.close(); dockerStreamWs = null }
  dockerLiveCmd.value = null
  dockerConsoleText.value = ''
}

async function scrollDockerConsole() {
  await nextTick()
  if (dockerConsoleOutput.value) {
    dockerConsoleOutput.value.scrollTop = dockerConsoleOutput.value.scrollHeight
  }
}

onUnmounted(() => {
  if (dockerStreamWs) dockerStreamWs.close()
})

const { wsStatus, wsError, retryCount, reconnect } = useWebSocket('/api/v1/ws/docker', (payload) => {
  if (payload.type !== 'docker') return
  containers.value = payload.containers || []
  composeProjects.value = payload.compose_projects || []
})
</script>
