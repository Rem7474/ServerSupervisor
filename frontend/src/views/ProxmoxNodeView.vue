<template>
  <div>
    <div
      v-if="loading"
      class="text-center py-5 text-muted"
    >
      Chargement...
    </div>
    <div
      v-else-if="error"
      class="alert alert-danger"
    >
      {{ error }}
    </div>
    <div v-else-if="node">
      <!-- Header -->
      <div class="page-header mb-4">
        <div class="page-pretitle">
          <router-link
            to="/"
            class="text-decoration-none"
          >
            Dashboard
          </router-link>
          <span class="text-muted mx-1">/</span>
          <router-link
            to="/proxmox"
            class="text-decoration-none"
          >
            Proxmox VE
          </router-link>
          <span class="text-muted mx-1">/</span>
          <span>{{ node.node_name }}</span>
        </div>
        <div class="d-flex align-items-center gap-3 flex-wrap">
          <h2 class="page-title mb-0">
            {{ node.node_name }}
          </h2>
          <span
            v-if="node.status === 'online'"
            class="status status-lime"
          >
            <span class="status-dot status-dot-animated" />
            <span data-translation-id="online">En ligne</span>
          </span>
          <span
            v-else
            class="status status-red"
          >
            <span class="status-dot status-dot-animated" />
            <span data-translation-id="offline">Hors ligne</span>
          </span>
          <span
            v-if="nodeCpuTempCurrent > 0"
            class="badge"
            :class="tempBadgeClass(nodeCpuTempCurrent)"
          >
            CPU TEMP {{ nodeCpuTempCurrent.toFixed(1) }}°C
          </span>
          <span
            v-if="nodeFanRPMCurrent > 0"
            class="badge bg-blue-lt text-blue"
          >
            FAN {{ nodeFanRPMCurrent.toFixed(0) }} RPM
          </span>
        </div>
        <div class="text-secondary">
          {{ node.cluster_name || 'Nœud standalone' }} · PVE {{ node.pve_version || 'N/A' }} · {{ node.ip_address }}
        </div>
      </div>

      <!-- Shared sensor source mapping (CPU temp + fan RPM) -->
      <div class="card mb-3">
        <div class="card-body d-flex flex-wrap align-items-center gap-2">
          <div class="subheader mb-0 me-2">
            Source capteurs nœud (CPU + ventilateurs)
          </div>
          <select
            v-model="sensorSourceHostId"
            class="form-select form-select-sm proxmox-source-select"
          >
            <option value="">
              Aucune (capteurs locaux du nœud)
            </option>
            <option
              v-for="candidate in sensorSourceCandidates"
              :key="candidate.id"
              :value="candidate.id"
            >
              {{ candidate.hostname || candidate.name }} ({{ candidate.ip_address }})
            </option>
          </select>
          <button
            class="btn btn-sm btn-primary"
            :disabled="sensorSourceSaving || sensorSourceLoading"
            @click="saveSensorSource"
          >
            <span
              v-if="sensorSourceSaving"
              class="spinner-border spinner-border-sm me-1"
            />
            Enregistrer
          </button>
          <span
            v-if="sensorSourceMsg"
            :class="['small', sensorSourceOk ? 'text-success' : 'text-danger']"
          >{{ sensorSourceMsg }}</span>
          <span
            v-else-if="sensorSourceHostName"
            class="small text-muted"
          >Actuel: {{ sensorSourceHostName }}</span>
        </div>
      </div>

      <!-- Compact node stats (static + live in one card) -->
      <div class="card mb-4">
        <div class="card-body position-relative">
          <div class="row g-4 align-items-start">
            <!-- CPU -->
            <div class="col-6 col-sm-4 col-lg">
              <div class="subheader mb-1">
                CPU
              </div>
              <div class="h3 mb-1">
                {{ (node.cpu_usage * 100).toFixed(1) }}%
              </div>
              <div class="progress progress-xs mb-1">
                <div
                  class="progress-bar"
                  :class="cpuColor(node.cpu_usage)"
                  :style="`width:${(node.cpu_usage*100).toFixed(1)}%`"
                />
              </div>
              <div class="text-muted small">
                {{ node.cpu_count }} cœurs
              </div>
            </div>

            <!-- CPU Temp (from mapped source host) -->
            <div class="col-6 col-sm-4 col-lg">
              <div class="subheader mb-1">
                CPU TEMP
              </div>
              <div
                class="h3 mb-1"
                :class="tempColor(nodeCpuTempCurrent)"
              >
                {{ nodeCpuTempCurrent > 0 ? `${nodeCpuTempCurrent.toFixed(1)}°C` : '—' }}
              </div>
              <div class="text-muted small">
                {{ sensorSourceHostName ? `Source: ${sensorSourceHostName}` : 'Source non configurée' }}
              </div>
            </div>

            <!-- Fan RPM (from mapped source host) -->
            <div class="col-6 col-sm-4 col-lg">
              <div class="subheader mb-1">
                FAN RPM
              </div>
              <div class="h3 mb-1 text-blue">
                {{ nodeFanRPMCurrent > 0 ? `${nodeFanRPMCurrent.toFixed(0)} RPM` : '—' }}
              </div>
              <div class="text-muted small">
                {{ sensorSourceHostName ? `Source: ${sensorSourceHostName}` : 'Source non configurée' }}
              </div>
            </div>

            <!-- RAM -->
            <div class="col-6 col-sm-4 col-lg">
              <div class="subheader mb-1">
                RAM
              </div>
              <div class="h3 mb-1">
                {{ formatBytes(node.mem_used) }}
              </div>
              <div class="progress progress-xs mb-1">
                <div
                  class="progress-bar"
                  :class="ramColor(node.mem_used, node.mem_total)"
                  :style="`width:${memPct(node)}%`"
                />
              </div>
              <div class="text-muted small">
                / {{ formatBytes(node.mem_total) }}
              </div>
            </div>

            <!-- Uptime -->
            <div class="col-6 col-sm-4 col-lg">
              <div class="subheader mb-1">
                Uptime
              </div>
              <div class="h3 mb-0">
                {{ formatUptime(node.uptime) }}
              </div>
            </div>

            <!-- Guests -->
            <div class="col-6 col-sm-4 col-lg">
              <div class="subheader mb-1">
                Guests
              </div>
              <div class="h3 mb-0">
                <span class="text-primary">{{ node.vm_count }}</span><span class="text-muted fs-5 ms-1">VM</span>
                <span class="ms-2 text-info">{{ node.lxc_count }}</span><span class="text-muted fs-5 ms-1">LXC</span>
              </div>
            </div>

            <!-- Live data separator -->
            <template v-if="liveStatus">
              <div class="col-auto d-none d-lg-flex align-items-stretch py-1">
                <div class="vr" />
              </div>

              <!-- IO Wait -->
              <div class="col-6 col-sm-4 col-lg">
                <div class="subheader mb-1">
                  IO Wait
                </div>
                <div
                  class="h3 mb-0"
                  :class="liveStatus.wait > 0.2 ? 'text-danger' : liveStatus.wait > 0.05 ? 'text-warning' : 'text-success'"
                >
                  {{ (liveStatus.wait * 100).toFixed(2) }}%
                </div>
                <div class="text-muted small">
                  disque
                </div>
              </div>

              <!-- Swap -->
              <div class="col-6 col-sm-4 col-lg">
                <div class="subheader mb-1">
                  Swap
                </div>
                <div class="h3 mb-1">
                  {{ formatBytes(liveStatus.swap.used) }}
                </div>
                <div
                  v-if="liveStatus.swap.total"
                  class="progress progress-xs mb-1"
                >
                  <div
                    class="progress-bar"
                    :class="ramColor(liveStatus.swap.used, liveStatus.swap.total)"
                    :style="`width:${(liveStatus.swap.used/liveStatus.swap.total*100).toFixed(1)}%`"
                  />
                </div>
                <div class="text-muted small">
                  / {{ formatBytes(liveStatus.swap.total) }}
                </div>
              </div>

              <!-- Rootfs -->
              <div class="col-6 col-sm-4 col-lg">
                <div class="subheader mb-1">
                  Rootfs
                </div>
                <div class="h3 mb-1">
                  {{ formatBytes(liveStatus.rootfs.used) }}
                </div>
                <div class="progress progress-xs mb-1">
                  <div
                    class="progress-bar"
                    :class="storageColor(liveStatus.rootfs.used, liveStatus.rootfs.total)"
                    :style="`width:${(liveStatus.rootfs.used/liveStatus.rootfs.total*100).toFixed(1)}%`"
                  />
                </div>
                <div class="text-muted small">
                  / {{ formatBytes(liveStatus.rootfs.total) }}
                </div>
              </div>
            </template>

            <!-- Live loading placeholder -->
            <div
              v-else-if="liveStatusLoading"
              class="col align-self-center text-muted small"
            >
              <span class="spinner-border spinner-border-sm me-1" />Chargement…
            </div>
          </div>

          <!-- Live refresh timestamp + error (absolute, no added height) -->
          <div class="position-absolute bottom-0 end-0 pb-2 pe-3 d-flex align-items-center gap-2 node-live-meta">
            <span
              v-if="liveStatusError"
              class="text-danger node-live-meta-text"
            >{{ liveStatusError }}</span>
            <span
              v-if="liveStatus"
              class="text-muted node-live-meta-text"
            >
              <span
                v-if="liveStatusLoading"
                class="spinner-border me-1 node-live-meta-spinner"
              />
              Actualisé à {{ liveStatusTime }}
            </span>
          </div>
        </div>
      </div>

      <!-- RRD Charts -->
      <div class="row row-cards mb-4">
        <div class="col-12 col-lg-4">
          <div class="card">
            <div class="card-header d-flex align-items-center justify-content-between">
              <h3 class="card-title mb-0">
                CPU
              </h3>
              <div
                v-if="!rrdLoading"
                class="btn-group btn-group-sm"
              >
                <button
                  v-for="opt in rrdTimeframeOptions"
                  :key="opt.value"
                  :class="rrdTimeframe === opt.value ? 'btn btn-primary' : 'btn btn-outline-secondary'"
                  @click="loadRRD(opt.value)"
                >
                  {{ opt.label }}
                </button>
              </div>
              <span
                v-else
                class="spinner-border spinner-border-sm text-muted"
              />
            </div>
            <div class="card-body proxmox-chart-body">
              <Line
                v-if="rrdCpuChart"
                :data="rrdCpuChart"
                :options="rrdPctOptions"
                class="h-100"
              />
              <div
                v-else
                class="h-100 d-flex align-items-center justify-content-center text-secondary small"
              >
                {{ rrdError || 'Aucune donnée' }}
              </div>
            </div>
          </div>
        </div>
        <div class="col-12 col-lg-4">
          <div class="card">
            <div class="card-header">
              <h3 class="card-title mb-0">
                RAM
              </h3>
            </div>
            <div class="card-body proxmox-chart-body">
              <Line
                v-if="rrdRamChart"
                :data="rrdRamChart"
                :options="rrdRamOptions"
                class="h-100"
              />
              <div
                v-else
                class="h-100 d-flex align-items-center justify-content-center text-secondary small"
              >
                {{ rrdError || 'Aucune donnée' }}
              </div>
            </div>
          </div>
        </div>
        <div class="col-12 col-lg-4">
          <div class="card">
            <div class="card-header">
              <h3 class="card-title mb-0">
                IO Wait
              </h3>
            </div>
            <div class="card-body proxmox-chart-body">
              <Line
                v-if="rrdIowaitChart"
                :data="rrdIowaitChart"
                :options="rrdPctOptions"
                class="h-100"
              />
              <div
                v-else
                class="h-100 d-flex align-items-center justify-content-center text-secondary small"
              >
                {{ rrdError || 'Aucune donnée' }}
              </div>
            </div>
          </div>
        </div>
        <div class="col-12 col-lg-4">
          <div class="card">
            <div class="card-header">
              <h3 class="card-title mb-0">
                Réseau
              </h3>
            </div>
            <div class="card-body proxmox-chart-body">
              <Line
                v-if="rrdNetChart"
                :data="rrdNetChart"
                :options="rrdNetOptions"
                class="h-100"
              />
              <div
                v-else
                class="h-100 d-flex align-items-center justify-content-center text-secondary small"
              >
                {{ rrdError || 'Aucune donnée' }}
              </div>
            </div>
          </div>
        </div>
        <div class="col-12 col-lg-4">
          <div class="card">
            <div class="card-header d-flex align-items-center justify-content-between">
              <h3 class="card-title mb-0">
                Historique temp. CPU (nœud)
              </h3>
              <span class="badge bg-azure-lt text-azure">Période: {{ rrdTimeframeLabel }}</span>
            </div>
            <div class="card-body proxmox-chart-body">
              <Line
                v-if="nodeTempChart"
                :data="nodeTempChart"
                :options="tempChartOptions"
                class="h-100"
              />
              <div
                v-else
                class="h-100 d-flex align-items-center justify-content-center text-secondary small"
              >
                {{ nodeTempLoading ? 'Chargement…' : (nodeTempError || (sensorSourceHostId ? 'Aucune donnée température disponible' : 'Configurez une source capteurs pour ce nœud')) }}
              </div>
            </div>
          </div>
        </div>
        <div class="col-12 col-lg-4">
          <div class="card">
            <div class="card-header d-flex align-items-center justify-content-between">
              <h3 class="card-title mb-0">
                Historique RPM ventilateurs (nœud)
              </h3>
              <span class="badge bg-blue-lt text-blue">Période: {{ rrdTimeframeLabel }}</span>
            </div>
            <div class="card-body proxmox-chart-body">
              <Line
                v-if="nodeFanChart"
                :data="nodeFanChart"
                :options="fanChartOptions"
                class="h-100"
              />
              <div
                v-else
                class="h-100 d-flex align-items-center justify-content-center text-secondary small"
              >
                {{ nodeFanLoading ? 'Chargement…' : (nodeFanError || (sensorSourceHostId ? 'Aucune donnée ventilateur disponible' : 'Configurez une source capteurs pour ce nœud')) }}
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Updates banner (only shown when pending updates exist) -->
      <div
        v-if="node.pending_updates > 0"
        class="alert alert-warning mb-4"
      >
        <div class="d-flex align-items-center gap-3">
          <div>
            <strong>Mises à jour disponibles sur ce nœud :</strong>
            {{ node.pending_updates }} paquet(s) en attente
          </div>
          <div
            v-if="node.last_update_check_at"
            class="ms-auto text-muted small"
          >
            Dernière vérification : {{ formatDate(node.last_update_check_at) }}
          </div>
        </div>
      </div>

      <!-- Tabs + side console -->
      <div class="side-layout">
        <div class="side-main">
          <div class="card">
            <div class="card-header">
              <ul class="nav nav-tabs card-header-tabs proxmox-node-tabs">
                <li class="nav-item">
                  <button
                    class="nav-link"
                    :class="{ active: tab === 'vms' }"
                    @click="tab = 'vms'; loadGuestNetworks()"
                  >
                    VMs <span class="badge bg-azure-lt text-azure ms-1">{{ vms.length }}</span>
                  </button>
                </li>
                <li class="nav-item">
                  <button
                    class="nav-link"
                    :class="{ active: tab === 'lxc' }"
                    @click="tab = 'lxc'; loadGuestNetworks()"
                  >
                    LXC <span class="badge bg-azure-lt text-azure ms-1">{{ lxcs.length }}</span>
                  </button>
                </li>
                <li class="nav-item">
                  <button
                    class="nav-link"
                    :class="{ active: tab === 'storage' }"
                    @click="tab = 'storage'"
                  >
                    Stockage <span class="badge bg-azure-lt text-azure ms-1">{{ node.storages?.length ?? 0 }}</span>
                  </button>
                </li>
                <li class="nav-item">
                  <button
                    class="nav-link"
                    :class="{ active: tab === 'disks' }"
                    @click="tab = 'disks'"
                  >
                    Disques <span class="badge bg-azure-lt text-azure ms-1">{{ node.disks?.length ?? 0 }}</span>
                  </button>
                </li>
                <li class="nav-item">
                  <button
                    class="nav-link"
                    :class="{ active: tab === 'tasks' }"
                    @click="tab = 'tasks'"
                  >
                    Tâches <span class="badge bg-azure-lt text-azure ms-1">{{ node.tasks?.length ?? 0 }}</span>
                    <span
                      v-if="failedTaskCount > 0"
                      class="badge bg-warning ms-1"
                    >{{ failedTaskCount }}</span>
                  </button>
                </li>
                <li class="nav-item">
                  <button
                    class="nav-link"
                    :class="{ active: tab === 'updates' }"
                    @click="tab = 'updates'"
                  >
                    Mises à jour
                    <span
                      v-if="node.pending_updates > 0"
                      class="badge ms-1 bg-warning-lt text-warning"
                    >
                      {{ node.pending_updates }}
                    </span>
                  </button>
                </li>
                <li class="nav-item">
                  <button
                    class="nav-link"
                    :class="{ active: tab === 'services' }"
                    @click="tab = 'services'; loadServices()"
                  >
                    Services
                  </button>
                </li>
                <li class="nav-item">
                  <button
                    class="nav-link"
                    :class="{ active: tab === 'security' }"
                    @click="tab = 'security'; loadNodeSecurityEvents()"
                  >
                    Sécurité <span class="badge bg-azure-lt text-azure ms-1">{{ securityEvents.length }}</span>
                  </button>
                </li>
              </ul>
            </div>

            <!-- VMs tab -->
            <div
              v-if="tab === 'vms'"
              class="table-responsive"
            >
              <table class="table table-vcenter card-table">
                <thead>
                  <tr>
                    <th>VMID</th>
                    <th>Nom</th>
                    <th>Statut</th>
                    <th>IP</th>
                    <th>CPU alloué</th>
                    <th>CPU utilisé</th>
                    <th>RAM allouée</th>
                    <th>RAM utilisée</th>
                    <th>Disque</th>
                    <th>Uptime</th>
                    <th>Tags</th>
                    <th>Hôte lié</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-if="vms.length === 0">
                    <td
                      colspan="12"
                      class="text-center text-muted py-4"
                    >
                      Aucune VM sur ce nœud.
                    </td>
                  </tr>
                  <tr
                    v-for="g in vms"
                    :key="g.id"
                  >
                    <td class="text-muted">
                      {{ g.vmid }}
                    </td>
                    <td class="fw-medium">
                      <router-link
                        :to="`/proxmox/guests/${g.id}?nodeId=${route.params.id}`"
                        class="text-decoration-none"
                      >
                        {{ g.name || '—' }}
                      </router-link>
                    </td>
                    <td><span :class="guestStatusClass(g.status)">{{ g.status }}</span></td>
                    <td>
                      <span
                        v-if="guestNetworksLoading"
                        class="text-muted small"
                      >…</span>
                      <template v-else-if="guestNetworks[g.vmid]?.length">
                        <div
                          v-for="iface in guestNetworks[g.vmid]"
                          :key="iface.name"
                          class="small lh-sm"
                        >
                          <span class="text-muted me-1">{{ iface.name }}</span>
                          <span
                            v-for="ip in iface.ips.filter(i => !i.startsWith('fe80'))"
                            :key="ip"
                          >{{ ip.split('/')[0] }}</span>
                        </div>
                      </template>
                      <span
                        v-else
                        class="text-muted"
                      >—</span>
                    </td>
                    <td>{{ g.cpu_alloc }} vCPU</td>
                    <td>{{ (g.cpu_usage * 100).toFixed(1) }}%</td>
                    <td>{{ formatBytes(g.mem_alloc) }}</td>
                    <td>{{ formatBytes(g.mem_usage) }}</td>
                    <td>{{ formatBytes(g.disk_alloc) }}</td>
                    <td>{{ g.status === 'running' ? formatUptime(g.uptime) : '—' }}</td>
                    <td>
                      <template v-if="g.tags">
                        <span
                          v-for="tag in g.tags.split(';').filter(Boolean)"
                          :key="tag"
                          class="badge bg-blue-lt text-blue me-1"
                        >{{ tag.trim() }}</span>
                      </template>
                    </td>
                    <td>
                      <GuestLinkCell
                        :link="linkForGuest(g)"
                        @confirm="confirmGuestLink(g)"
                        @ignore="ignoreGuestLink(g)"
                        @go="goToHost(linkForGuest(g))"
                      />
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>

            <!-- LXC tab -->
            <div
              v-if="tab === 'lxc'"
              class="table-responsive"
            >
              <table class="table table-vcenter card-table">
                <thead>
                  <tr>
                    <th>CT ID</th>
                    <th>Nom</th>
                    <th>Statut</th>
                    <th>IP</th>
                    <th>CPU alloué</th>
                    <th>CPU utilisé</th>
                    <th>RAM allouée</th>
                    <th>RAM utilisée</th>
                    <th>Disque</th>
                    <th>Uptime</th>
                    <th>Hôte lié</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-if="lxcs.length === 0">
                    <td
                      colspan="11"
                      class="text-center text-muted py-4"
                    >
                      Aucun conteneur LXC sur ce nœud.
                    </td>
                  </tr>
                  <tr
                    v-for="g in lxcs"
                    :key="g.id"
                  >
                    <td class="text-muted">
                      {{ g.vmid }}
                    </td>
                    <td class="fw-medium">
                      <router-link
                        :to="`/proxmox/guests/${g.id}?nodeId=${route.params.id}`"
                        class="text-decoration-none"
                      >
                        {{ g.name || '—' }}
                      </router-link>
                    </td>
                    <td><span :class="guestStatusClass(g.status)">{{ g.status }}</span></td>
                    <td>
                      <span
                        v-if="guestNetworksLoading"
                        class="text-muted small"
                      >…</span>
                      <template v-else-if="guestNetworks[g.vmid]?.length">
                        <div
                          v-for="iface in guestNetworks[g.vmid]"
                          :key="iface.name"
                          class="small lh-sm"
                        >
                          <span class="text-muted me-1">{{ iface.name }}</span>
                          <span
                            v-for="ip in iface.ips.filter(i => !i.startsWith('fe80'))"
                            :key="ip"
                          >{{ ip.split('/')[0] }}</span>
                        </div>
                      </template>
                      <span
                        v-else
                        class="text-muted"
                      >—</span>
                    </td>
                    <td>{{ g.cpu_alloc }}</td>
                    <td>{{ (g.cpu_usage * 100).toFixed(1) }}%</td>
                    <td>{{ formatBytes(g.mem_alloc) }}</td>
                    <td>{{ formatBytes(g.mem_usage) }}</td>
                    <td>{{ formatBytes(g.disk_alloc) }}</td>
                    <td>{{ g.status === 'running' ? formatUptime(g.uptime) : '—' }}</td>
                    <td>
                      <GuestLinkCell
                        :link="linkForGuest(g)"
                        @confirm="confirmGuestLink(g)"
                        @ignore="ignoreGuestLink(g)"
                        @go="goToHost(linkForGuest(g))"
                      />
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>

            <!-- Link action feedback -->
            <div
              v-if="linkMsg"
              class="card-footer py-2"
            >
              <span :class="['small', linkMsgOk ? 'text-success' : 'text-danger']">{{ linkMsg }}</span>
            </div>

            <!-- Disks tab -->
            <div
              v-if="tab === 'disks'"
              class="table-responsive"
            >
              <table class="table table-vcenter card-table">
                <thead>
                  <tr>
                    <th>Périphérique</th>
                    <th>Modèle</th>
                    <th>Type</th>
                    <th>Taille</th>
                    <th>Santé SMART</th>
                    <th>Usure SSD</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-if="!node.disks?.length">
                    <td
                      colspan="6"
                      class="text-center text-muted py-4"
                    >
                      Aucun disque détecté sur ce nœud.
                    </td>
                  </tr>
                  <tr
                    v-for="d in node.disks"
                    :key="d.id"
                  >
                    <td class="fw-medium font-monospace">
                      {{ d.dev_path }}
                    </td>
                    <td>
                      {{ d.model || '—' }}<div class="text-muted small">
                        {{ d.serial }}
                      </div>
                    </td>
                    <td><span class="badge bg-secondary-lt text-secondary text-uppercase">{{ d.disk_type || '?' }}</span></td>
                    <td>{{ formatBytes(d.size_bytes) }}</td>
                    <td>
                      <span
                        v-if="d.health === 'PASSED'"
                        class="badge bg-success-lt text-success"
                      >PASSED</span>
                      <span
                        v-else-if="d.health === 'FAILED'"
                        class="badge bg-danger-lt text-danger"
                      >FAILED</span>
                      <span
                        v-else
                        class="badge bg-secondary-lt text-secondary"
                      >{{ d.health }}</span>
                    </td>
                    <td>
                      <template v-if="d.wearout >= 0">
                        <div class="d-flex align-items-center gap-2">
                          <div class="progress progress-xs flex-grow-1 proxmox-progress-min-60">
                            <div
                              class="progress-bar"
                              :class="wearoutColor(d.wearout)"
                              :style="`width:${d.wearout}%`"
                            />
                          </div>
                          <span class="text-muted small">{{ d.wearout }}%</span>
                        </div>
                      </template>
                      <span
                        v-else
                        class="text-muted"
                      >—</span>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>

            <!-- Tasks tab -->
            <div v-if="tab === 'tasks'">
              <div class="table-responsive">
                <table class="table table-vcenter card-table">
                  <thead>
                    <tr>
                      <th>Type</th>
                      <th>Objet</th>
                      <th>Utilisateur</th>
                      <th>Début</th>
                      <th>Durée</th>
                      <th>Statut</th>
                      <th />
                    </tr>
                  </thead>
                  <tbody>
                    <template v-if="!node.tasks?.length">
                      <tr>
                        <td
                          colspan="7"
                          class="text-center text-muted py-4"
                        >
                          Aucune tâche récente pour ce nœud.
                        </td>
                      </tr>
                    </template>
                    <tr
                      v-for="t in node.tasks"
                      v-else
                      :key="t.id"
                      :class="activeUpid === t.upid ? 'table-active' : ''"
                    >
                      <td><span class="badge bg-azure-lt text-azure font-monospace">{{ t.task_type }}</span></td>
                      <td class="text-muted">
                        {{ t.object_id || '—' }}
                      </td>
                      <td class="text-muted small">
                        {{ t.user_name }}
                      </td>
                      <td class="text-muted small">
                        {{ formatDate(t.start_time) }}
                      </td>
                      <td class="text-muted small">
                        {{ taskDuration(t) }}
                      </td>
                      <td>
                        <span
                          v-if="t.status === 'running'"
                          class="badge bg-blue-lt text-blue"
                        >En cours</span>
                        <span
                          v-else-if="t.exit_status === 'OK'"
                          class="badge bg-success-lt text-success"
                        >OK</span>
                        <span
                          v-else-if="t.exit_status"
                          class="badge bg-danger-lt text-danger"
                          :title="t.exit_status"
                        >Erreur</span>
                        <span
                          v-else
                          class="badge bg-secondary-lt text-secondary"
                        >{{ t.status }}</span>
                      </td>
                      <td>
                        <button
                          class="btn btn-sm btn-ghost-secondary"
                          title="Voir les logs"
                          @click="startPollingTask(t.upid, {action: t.task_type, label: t.object_id})"
                        >
                          <svg
                            xmlns="http://www.w3.org/2000/svg"
                            class="icon icon-sm"
                            width="16"
                            height="16"
                            viewBox="0 0 24 24"
                            stroke-width="2"
                            stroke="currentColor"
                            fill="none"
                          ><path
                            stroke="none"
                            d="M0 0h24v24H0z"
                            fill="none"
                          /><path d="M4 6l16 0" /><path d="M4 12l16 0" /><path d="M4 18l12 0" /></svg>
                        </button>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>

            <!-- Updates tab -->
            <div
              v-if="tab === 'updates'"
              class="card-body"
            >
              <!-- Apt update action bar -->
              <div class="d-flex align-items-center gap-2 mb-3 flex-wrap">
                <button
                  class="btn btn-outline-secondary"
                  :disabled="aptRefreshing"
                  @click="triggerAptRefresh"
                >
                  <span
                    v-if="aptRefreshing"
                    class="spinner-border spinner-border-sm me-1"
                  />
                  apt update
                </button>
                <span
                  v-if="aptRefreshMsg"
                  :class="['small', aptRefreshOk ? 'text-success' : 'text-danger']"
                >{{ aptRefreshMsg }}</span>
              </div>

              <div
                v-if="node.pending_updates === 0"
                class="text-center text-muted py-3"
              >
                <div class="mb-1">
                  Aucune mise à jour en attente détectée.
                </div>
                <div
                  v-if="node.last_update_check_at"
                  class="small"
                >
                  Dernière vérification : {{ formatDate(node.last_update_check_at) }}
                </div>
                <div
                  v-else
                  class="small"
                >
                  Données non encore disponibles (prochain cycle de polling).
                </div>
              </div>
              <div v-else>
                <div class="d-flex align-items-center gap-3 mb-3">
                  <div class="h2 mb-0">
                    {{ node.pending_updates }}
                  </div>
                  <div>
                    <div class="fw-medium">
                      paquet(s) en attente de mise à jour
                    </div>
                    <div
                      v-if="node.last_update_check_at"
                      class="text-muted small"
                    >
                      Détecté le {{ formatDate(node.last_update_check_at) }}
                    </div>
                  </div>
                </div>
                <div class="alert alert-info mb-0">
                  Ces informations proviennent du cache apt du nœud Proxmox (lecture seule).
                  Pour appliquer les mises à jour, connectez-vous directement au nœud.
                </div>
              </div>
            </div>

            <!-- Services tab -->
            <div v-if="tab === 'services'">
              <div class="card-header d-flex align-items-center gap-2 flex-wrap">
                <div class="btn-group btn-group-sm">
                  <button
                    :class="servicesFilter === 'active' ? 'btn btn-primary' : 'btn btn-outline-secondary'"
                    @click="servicesFilter = 'active'"
                  >
                    Actifs
                  </button>
                  <button
                    :class="servicesFilter === 'all' ? 'btn btn-primary' : 'btn btn-outline-secondary'"
                    @click="servicesFilter = 'all'"
                  >
                    Tous
                  </button>
                </div>
                <button
                  class="btn btn-sm btn-outline-secondary ms-2"
                  :disabled="servicesLoading"
                  @click="loadServices"
                >
                  <span
                    v-if="servicesLoading"
                    class="spinner-border spinner-border-sm me-1"
                  />
                  {{ servicesLoading ? 'Chargement...' : '↻ Actualiser' }}
                </button>
                <span
                  v-if="svcActionMsg"
                  :class="['small ms-2', svcActionOk ? 'text-success' : 'text-danger']"
                >{{ svcActionMsg }}</span>
              </div>
              <div
                v-if="servicesError"
                class="card-body pb-0"
              >
                <div class="alert alert-danger mb-0">
                  {{ servicesError }}
                </div>
              </div>
              <div
                v-if="!services.length && !servicesLoading && !servicesError"
                class="card-body"
              >
                <div class="text-secondary small">
                  Cliquez sur "Actualiser" pour charger les services du nœud Proxmox.
                </div>
              </div>
              <div
                v-if="filteredServices.length"
                class="table-responsive"
              >
                <table class="table table-vcenter table-hover card-table mb-0">
                  <thead>
                    <tr>
                      <th>Service</th>
                      <th>État</th>
                      <th>Sous-état</th>
                      <th>Démarrage</th>
                      <th>Description</th>
                      <th>Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr
                      v-for="svc in filteredServices"
                      :key="svc.name"
                    >
                      <td class="font-monospace small fw-medium">
                        {{ svc.name }}
                      </td>
                      <td><span :class="svcStateClass(svc['active-state'])">{{ svc['active-state'] || svc.state }}</span></td>
                      <td class="text-secondary small">
                        {{ svc['sub-state'] || '—' }}
                      </td>
                      <td class="text-secondary small">
                        {{ svc['unit-state'] || '—' }}
                      </td>
                      <td
                        class="text-secondary small text-truncate proxmox-service-desc"
                        :title="svc.desc"
                      >
                        {{ svc.desc || '—' }}
                      </td>
                      <td class="text-nowrap">
                        <div class="btn-group btn-group-sm">
                          <button
                            v-if="svc['active-state'] !== 'active'"
                            class="btn btn-outline-success"
                            title="Démarrer"
                            @click="svcAction(svc.name, 'start')"
                          >
                            Start
                          </button>
                          <button
                            v-if="svc['active-state'] === 'active'"
                            class="btn btn-outline-danger"
                            title="Arrêter"
                            @click="svcAction(svc.name, 'stop')"
                          >
                            Stop
                          </button>
                          <button
                            class="btn btn-outline-secondary"
                            title="Redémarrer"
                            @click="svcAction(svc.name, 'restart')"
                          >
                            Restart
                          </button>
                          <button
                            class="btn btn-outline-secondary"
                            title="Recharger"
                            @click="svcAction(svc.name, 'reload')"
                          >
                            Reload
                          </button>
                        </div>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
              <div
                v-if="servicesError"
                class="card-footer text-muted small"
              >
                Lecture : Sys.Audit requis · Actions (start/stop/restart/reload) : Sys.Modify requis sur le token API.
              </div>
            </div>

            <!-- Security tab -->
            <div v-if="tab === 'security'">
              <div class="card-header d-flex align-items-center gap-2 flex-wrap">
                <select
                  v-model="securityService"
                  class="form-select proxmox-security-service-select"
                >
                  <option value="rotate">
                    Rotation (3 services)
                  </option>
                  <option value="pveproxy">
                    pveproxy
                  </option>
                  <option value="sshd">
                    sshd
                  </option>
                  <option value="pvedaemon">
                    pvedaemon
                  </option>
                  <option value="">
                    Tous les services
                  </option>
                </select>
                <input
                  v-model="securitySearch"
                  type="text"
                  class="form-control proxmox-security-search"
                  placeholder="Filtre syslog (ex: failed, denied, apparmor)"
                >
                <button
                  class="btn btn-sm btn-outline-secondary"
                  :disabled="securityEventsLoading"
                  @click="loadNodeSecurityEvents"
                >
                  <span
                    v-if="securityEventsLoading"
                    class="spinner-border spinner-border-sm me-1"
                  />
                  Rechercher
                </button>
              </div>
              <div
                v-if="securityEventsError"
                class="card-body pb-0"
              >
                <div class="alert alert-danger mb-0">
                  {{ securityEventsError }}
                </div>
              </div>
              <div
                v-if="securityEventsLoading"
                class="card-body text-muted small"
              >
                <span class="spinner-border spinner-border-sm me-1" />Chargement des événements de sécurité…
              </div>
              <div
                v-else-if="securityEvents.length"
                class="table-responsive"
              >
                <table class="table table-vcenter card-table">
                  <thead>
                    <tr>
                      <th>Date</th>
                      <th>Niveau</th>
                      <th>Tag</th>
                      <th>Message</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr
                      v-for="(item, idx) in securityEvents"
                      :key="item.id || `${item.parsedTimeMs || item.time || 't'}-${idx}`"
                    >
                      <td class="text-muted small">
                        {{ formatSyslogTime(item) }}
                      </td>
                      <td>
                        <span
                          class="badge text-uppercase"
                          :class="syslogLevelBadgeClass(item)"
                        >{{ item.parsedLevel || item.pri || item.level || '—' }}</span>
                      </td>
                      <td class="font-monospace small">
                        {{ item.parsedTag || item.tag || item.ident || '—' }}
                      </td>
                      <td class="small">
                        {{ item.parsedMsg || item.msg || item.t || '—' }}
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
              <div
                v-else
                class="card-body text-center text-muted py-4"
              >
                Aucun événement de sécurité trouvé pour ce filtre.
              </div>
            </div>

            <!-- Storage tab -->
            <div
              v-if="tab === 'storage'"
              class="table-responsive"
            >
              <table class="table table-vcenter card-table">
                <thead>
                  <tr>
                    <th>Stockage</th>
                    <th>Type</th>
                    <th>Total</th>
                    <th>Utilisé</th>
                    <th>Disponible</th>
                    <th>Utilisation</th>
                    <th>Partagé</th>
                    <th>Statut</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-if="!node.storages?.length">
                    <td
                      colspan="8"
                      class="text-center text-muted py-4"
                    >
                      Aucun stockage sur ce nœud.
                    </td>
                  </tr>
                  <tr
                    v-for="s in node.storages"
                    :key="s.id"
                  >
                    <td class="fw-medium">
                      {{ s.storage_name }}
                    </td>
                    <td><span class="badge bg-secondary-lt text-secondary">{{ s.storage_type }}</span></td>
                    <td>{{ formatBytes(s.total) }}</td>
                    <td>{{ formatBytes(s.used) }}</td>
                    <td>{{ formatBytes(s.avail) }}</td>
                    <td>
                      <div class="d-flex align-items-center gap-2">
                        <div class="progress progress-xs flex-grow-1 proxmox-progress-min-80">
                          <div
                            class="progress-bar"
                            :class="storageColor(s.used, s.total)"
                            :style="`width:${storagePct(s)}%`"
                          />
                        </div>
                        <span class="text-muted small">{{ storagePct(s) }}%</span>
                      </div>
                    </td>
                    <td>
                      <span
                        v-if="s.shared"
                        class="badge bg-azure-lt text-azure"
                      >Oui</span>
                      <span
                        v-else
                        class="text-muted"
                      >—</span>
                    </td>
                    <td>
                      <span
                        v-if="s.active && s.enabled"
                        class="badge bg-success-lt text-success"
                      >Actif</span>
                      <span
                        v-else
                        class="badge bg-danger-lt text-danger"
                      >Inactif</span>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div> <!-- /side-main -->
        <CommandLogPanel
          :command="liveTask"
          :show="showConsole"
          title="Logs tâche PVE"
          empty-text="Cliquez sur 'Logs' dans une tâche pour suivre l'exécution"
          wrapper-class="side-panel"
          @open="showConsole = true"
          @close="closeConsole"
        />
      </div> <!-- /side-layout -->
    </div> <!-- /v-else-if node -->
  </div>
</template>

<script setup>
import { ref, computed, shallowRef, onMounted, onUnmounted, defineAsyncComponent, defineComponent, h } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import CommandLogPanel from '../components/CommandLogPanel.vue'
import api from '../api'

const Line = defineAsyncComponent(async () => {
  const [{ Line }, { Chart: ChartJS, LineElement, PointElement, LinearScale, CategoryScale, Filler, Tooltip }] = await Promise.all([
    import('vue-chartjs'),
    import('chart.js'),
  ])
  ChartJS.register(LineElement, PointElement, LinearScale, CategoryScale, Filler, Tooltip)
  return Line
})

// Inline component — renders the "Hôte lié" cell without a separate file.
const GuestLinkCell = defineComponent({
  props: { link: { type: Object, default: null } },
  emits: ['confirm', 'ignore', 'go'],
  setup(props, { emit }) {
    return () => {
      const link = props.link
      if (!link) return h('span', { class: 'text-muted small' }, '—')
      if (link.status === 'suggested') {
        return h('div', { class: 'd-flex align-items-center gap-1' }, [
          h('span', { class: 'badge bg-warning-lt text-warning' }, 'Suggéré'),
          h('span', { class: 'text-muted small' }, link.host_hostname || link.host_name),
          h('button', { class: 'btn btn-xs btn-success ms-1', onClick: () => emit('confirm') }, '✓'),
          h('button', { class: 'btn btn-xs btn-outline-secondary', onClick: () => emit('ignore') }, '✗'),
        ])
      }
      if (link.status === 'confirmed') {
        return h('div', { class: 'd-flex align-items-center gap-1' }, [
          h('span', { class: 'badge bg-success-lt text-success' }, 'Lié'),
          h('button', {
            class: 'btn btn-xs btn-outline-primary ms-1',
            onClick: () => emit('go'),
            title: 'Voir la fiche hôte',
          }, link.host_hostname || link.host_name),
        ])
      }
      return h('span', { class: 'text-muted small' }, '—')
    }
  },
})

const route = useRoute()
const router = useRouter()
const node = ref(null)
const loading = ref(true)
const error = ref('')
const tab = ref('vms')

// guest_id → link object (loaded after node data)
const guestLinks = ref({})
const linkMsg = ref('')
const linkMsgOk = ref(false)

const sensorSourceCandidates = ref([])
const sensorSourceHostId = ref('')
const sensorSourceLoading = ref(false)
const sensorSourceSaving = ref(false)
const sensorSourceMsg = ref('')
const sensorSourceOk = ref(false)
const sensorSourceHostName = computed(() =>
  node.value?.cpu_temp_source_host_name || node.value?.fan_rpm_source_host_name || ''
)

const nodeTempLoading = ref(false)
const nodeTempError = ref('')
const nodeTempChart = shallowRef(null)
const nodeCpuTempCurrent = ref(0)

const nodeFanLoading = ref(false)
const nodeFanError = ref('')
const nodeFanChart = shallowRef(null)
const nodeFanRPMCurrent = ref(0)

// apt refresh
const aptRefreshing = ref(false)
const aptRefreshMsg = ref('')
const aptRefreshOk = ref(false)

// live status (iowait, swap, rootfs) — auto-loaded on mount
const liveStatus = ref(null)
const liveStatusLoading = ref(false)
const liveStatusTime = ref('')
const liveStatusError = ref('')

// RRD charts
const rrdTimeframe = ref('hour')
const rrdTimeframeOptions = [
  { value: 'hour', label: '1h' },
  { value: 'day', label: '24h' },
  { value: 'week', label: '7j' },
  { value: 'month', label: '30j' },
  { value: 'year', label: '1 an' },
]
const rrdTimeframeToHours = {
  hour: 1,
  day: 24,
  week: 24 * 7,
  month: 24 * 30,
  year: 24 * 365,
}
const rrdTimeframeLabel = computed(() =>
  rrdTimeframeOptions.find(opt => opt.value === rrdTimeframe.value)?.label ?? '1h'
)
const rrdCpuChart = shallowRef(null)
const rrdRamChart = shallowRef(null)
const rrdIowaitChart = shallowRef(null)
const rrdNetChart = shallowRef(null)
const rrdLoading = ref(false)
const rrdError = ref('')

const rrdPctOptions = {
  responsive: true, maintainAspectRatio: false,
  plugins: {
    legend: { display: false },
    tooltip: { enabled: true, mode: 'index', intersect: false, backgroundColor: 'rgba(0,0,0,0.8)', titleColor: '#fff', bodyColor: '#fff', borderColor: '#555', borderWidth: 1, padding: 8, displayColors: false,
      callbacks: { label: (ctx) => `${ctx.parsed.y != null ? ctx.parsed.y.toFixed(1) : '—'}%` },
    },
  },
  scales: {
    x: { display: true, grid: { color: 'rgba(255,255,255,0.05)' }, ticks: { color: '#6b7280', maxTicksLimit: 8 } },
    y: { display: true, min: 0, max: 100, grid: { color: 'rgba(255,255,255,0.05)' }, ticks: { color: '#6b7280', callback: (v) => `${v}%` } },
  },
  elements: { point: { radius: 0, hitRadius: 10, hoverRadius: 4 }, line: { tension: 0.3 } },
  interaction: { mode: 'nearest', axis: 'x', intersect: false },
}

const rrdRamOptions = {
  ...rrdPctOptions,
  plugins: {
    ...rrdPctOptions.plugins,
    tooltip: {
      ...rrdPctOptions.plugins.tooltip,
      callbacks: {
        label: (ctx) => {
          const pct = ctx.parsed.y != null ? ctx.parsed.y.toFixed(1) : '—'
          return `${pct}%`
        },
      },
    },
  },
}

function formatBytesPerSec(v) {
  if (v == null) return '—'
  if (v >= 1_000_000) return `${(v / 1_000_000).toFixed(1)} MB/s`
  if (v >= 1_000) return `${(v / 1_000).toFixed(1)} KB/s`
  return `${v.toFixed(0)} B/s`
}

const rrdNetOptions = {
  responsive: true, maintainAspectRatio: false,
  plugins: {
    legend: { display: true, position: 'top', labels: { color: '#6b7280', boxWidth: 10, padding: 8 } },
    tooltip: { enabled: true, mode: 'index', intersect: false, backgroundColor: 'rgba(0,0,0,0.8)', titleColor: '#fff', bodyColor: '#fff', borderColor: '#555', borderWidth: 1, padding: 8,
      callbacks: { label: (ctx) => `${ctx.dataset.label}: ${formatBytesPerSec(ctx.parsed.y)}` },
    },
  },
  scales: {
    x: { display: true, grid: { color: 'rgba(255,255,255,0.05)' }, ticks: { color: '#6b7280', maxTicksLimit: 8 } },
    y: { display: true, min: 0, grid: { color: 'rgba(255,255,255,0.05)' }, ticks: { color: '#6b7280', callback: (v) => formatBytesPerSec(v) } },
  },
  elements: { point: { radius: 0, hitRadius: 10, hoverRadius: 4 }, line: { tension: 0.3 } },
  interaction: { mode: 'nearest', axis: 'x', intersect: false },
}

const tempChartOptions = {
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: { display: false },
    tooltip: {
      enabled: true,
      mode: 'index',
      intersect: false,
      backgroundColor: 'rgba(0,0,0,0.8)',
      titleColor: '#fff',
      bodyColor: '#fff',
      borderColor: '#555',
      borderWidth: 1,
      padding: 8,
      callbacks: {
        label: (ctx) => `${ctx.parsed.y != null ? ctx.parsed.y.toFixed(1) : '—'}°C`,
      },
    },
  },
  scales: {
    x: { display: true, grid: { color: 'rgba(255,255,255,0.05)' }, ticks: { color: '#6b7280', maxTicksLimit: 8 } },
    y: { display: true, grid: { color: 'rgba(255,255,255,0.05)' }, ticks: { color: '#6b7280', callback: (v) => `${v}°C` } },
  },
  elements: { point: { radius: 0, hitRadius: 10, hoverRadius: 4 }, line: { tension: 0.3 } },
  interaction: { mode: 'nearest', axis: 'x', intersect: false },
}

const fanChartOptions = {
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: { display: false },
    tooltip: {
      enabled: true,
      mode: 'index',
      intersect: false,
      backgroundColor: 'rgba(0,0,0,0.8)',
      titleColor: '#fff',
      bodyColor: '#fff',
      borderColor: '#555',
      borderWidth: 1,
      padding: 8,
      callbacks: {
        label: (ctx) => `${ctx.parsed.y != null ? Math.round(ctx.parsed.y) : '—'} RPM`,
      },
    },
  },
  scales: {
    x: { display: true, grid: { color: 'rgba(255,255,255,0.05)' }, ticks: { color: '#6b7280', maxTicksLimit: 8 } },
    y: { display: true, min: 0, grid: { color: 'rgba(255,255,255,0.05)' }, ticks: { color: '#6b7280', callback: (v) => `${v} RPM` } },
  },
  elements: { point: { radius: 0, hitRadius: 10, hoverRadius: 4 }, line: { tension: 0.3 } },
  interaction: { mode: 'nearest', axis: 'x', intersect: false },
}

// PVE task console (side panel + polling)
const showConsole = ref(false)
const liveTask = ref(null)
const activeUpid = ref(null)  // tracks which row is highlighted — separate from display target
let pollTimer = null
let liveStatusTimer = null

// guest network interfaces (live)
const guestNetworks = ref({})       // { [vmid]: [{name, mac, ips}] }
const guestNetworksLoading = ref(false)

async function loadGuestNetworks() {
  if (guestNetworksLoading.value || Object.keys(guestNetworks.value).length > 0) return
  guestNetworksLoading.value = true
  try {
    const res = await api.getProxmoxNodeGuestNetworks(route.params.id)
    guestNetworks.value = res.data ?? {}
  } catch { /* non-bloquant */ }
  finally { guestNetworksLoading.value = false }
}

// services
const services = ref([])
const servicesLoading = ref(false)
const servicesError = ref('')
const servicesFilter = ref('active')
const svcActionMsg = ref('')
const svcActionOk = ref(false)

// node syslog security events
const securityEvents = ref([])
const securityEventsLoading = ref(false)
const securityEventsError = ref('')
const securitySearch = ref('')
const securityService = ref('rotate')

function mergeAndRankSyslogLines(groups, maxLines = 200) {
  const flat = groups.flatMap(g => Array.isArray(g) ? g : []).map(normalizeSyslogEntry)
  const uniq = new Map()
  for (const item of flat) {
    const key = `${item.parsedTimeMs ?? item.time ?? ''}|${item.parsedTag ?? item.tag ?? ''}|${item.parsedMsg ?? item.msg ?? item.t ?? ''}`
    if (!uniq.has(key)) uniq.set(key, item)
  }
  const ranked = [...uniq.values()].sort((a, b) => {
    const ta = Number(a?.parsedTimeMs ?? a?.time ?? 0)
    const tb = Number(b?.parsedTimeMs ?? b?.time ?? 0)
    if (ta !== tb) return tb - ta
    return Number(b?.n ?? 0) - Number(a?.n ?? 0)
  })
  return ranked.slice(0, maxLines)
}

const SYSLOG_MONTHS = {
  Jan: 0, Feb: 1, Mar: 2, Apr: 3, May: 4, Jun: 5,
  Jul: 6, Aug: 7, Sep: 8, Oct: 9, Nov: 10, Dec: 11,
}

function guessLevel(text) {
  const v = String(text || '').toLowerCase()
  if (!v) return ''

  if (
    v.includes('successful auth') ||
    v.includes('authentication success') ||
    v.includes('authentication succeeded') ||
    v.includes('login successful')
  ) return 'success'

  // Security-significant auth events are elevated to critical for quick triage.
  if (
    v.includes('authentication failure') ||
    v.includes('failed password') ||
    v.includes('invalid user') ||
    v.includes('too many authentication failures') ||
    v.includes('maximum authentication attempts exceeded') ||
    v.includes('user root@pam msg=authentication failure')
  ) return 'critical'

  if (v.includes('critical') || v.includes('panic') || v.includes('fatal')) return 'critical'
  if (v.includes('error') || v.includes('failed') || v.includes('denied')) return 'error'
  if (v.includes('failure')) return 'error'
  if (v.includes('warn')) return 'warning'
  if (v.includes('info')) return 'info'
  return ''
}

function parseHeaderDate(prefix) {
  const m = /^([A-Z][a-z]{2})\s+(\d{1,2})\s+(\d{2}):(\d{2}):(\d{2})$/.exec(String(prefix || '').trim())
  if (!m) return null
  const month = SYSLOG_MONTHS[m[1]]
  if (month == null) return null
  const now = new Date()
  let year = now.getFullYear()
  let d = new Date(year, month, Number(m[2]), Number(m[3]), Number(m[4]), Number(m[5]))
  if (d.getTime() > now.getTime() + 86_400_000) {
    year -= 1
    d = new Date(year, month, Number(m[2]), Number(m[3]), Number(m[4]), Number(m[5]))
  }
  return d
}

function normalizeSyslogEntry(item) {
  const out = { ...(item || {}) }
  const raw = String(out.t || '')
  if (raw) {
    const m = /^([A-Z][a-z]{2}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2})\s+(\S+)\s+([^\s:]+)(?:\[(\d+)\])?:\s*(.*)$/.exec(raw)
    if (m) {
      const parsedDate = parseHeaderDate(m[1])
      if (parsedDate) out.parsedTimeMs = parsedDate.getTime()
      if (!out.parsedTag) out.parsedTag = m[3]
      const pidSuffix = m[4] ? `[${m[4]}]` : ''
      const message = (m[5] || '').trim()
      out.parsedMsg = message || `${m[2]} ${m[3]}${pidSuffix}`
      out.parsedLevel = out.level || guessLevel(out.parsedMsg)
    } else {
      out.parsedMsg = out.msg || raw
      out.parsedLevel = out.level || guessLevel(out.parsedMsg)
      out.parsedTag = out.tag || out.ident || ''
    }
  } else {
    out.parsedMsg = out.msg || ''
    out.parsedLevel = out.level || guessLevel(out.parsedMsg)
    out.parsedTag = out.tag || out.ident || ''
  }

  if (!out.parsedTimeMs && out.time) {
    const rawTime = out.time
    const ms = typeof rawTime === 'number'
      ? (rawTime < 1_000_000_000_000 ? rawTime * 1000 : rawTime)
      : Date.parse(rawTime)
    if (Number.isFinite(ms)) out.parsedTimeMs = ms
  }

  return out
}

const vms = computed(() => node.value?.guests?.filter(g => g.guest_type === 'vm') ?? [])
const lxcs = computed(() => node.value?.guests?.filter(g => g.guest_type === 'lxc') ?? [])
const failedTaskCount = computed(() =>
  (node.value?.tasks ?? []).filter(t => t.status === 'stopped' && t.exit_status && t.exit_status !== 'OK').length
)
const filteredServices = computed(() => {
  if (servicesFilter.value === 'all') return services.value
  return services.value.filter(s => s['active-state'] === 'active' || s.state === 'running')
})

async function load() {
  loading.value = true
  error.value = ''
  try {
    const requestedTab = String(route.query.tab || '')
    if (requestedTab === 'vms' || requestedTab === 'lxc' || requestedTab === 'storage' || requestedTab === 'disks' || requestedTab === 'tasks' || requestedTab === 'updates' || requestedTab === 'services' || requestedTab === 'security') {
      tab.value = requestedTab
    }
    const res = await api.getProxmoxNode(route.params.id)
    node.value = res.data
    sensorSourceHostId.value = node.value?.cpu_temp_source_host_id || node.value?.fan_rpm_source_host_id || ''
    await loadSensorSourceCandidates()
    await loadGuestLinks()
    // fire-and-forget: live status + RRD charts load in parallel
    loadLiveStatus()
    loadRRD('hour')
    if (tab.value === 'security') {
      loadNodeSecurityEvents()
    }
  } catch (e) {
    error.value = e?.response?.data?.error || 'Erreur lors du chargement.'
  } finally {
    loading.value = false
  }
}

async function loadNodeCpuTempHistory(hours = rrdTimeframeToHours[rrdTimeframe.value] ?? 24) {
  nodeTempLoading.value = true
  nodeTempError.value = ''
  nodeTempChart.value = null
  nodeCpuTempCurrent.value = 0

  try {
    const sourceHostId = sensorSourceHostId.value || node.value?.cpu_temp_source_host_id || node.value?.fan_rpm_source_host_id
    if (!sourceHostId) {
      return
    }

    const res = await api.getProxmoxNodeCpuTempHistory(route.params.id, hours)
    const points = (Array.isArray(res.data) ? res.data : []).filter(p => Number(p?.cpu_temperature) > 0)
    if (!points.length) {
      return
    }

    const labels = points.map(p => {
      const d = new Date(p.timestamp)
      if (hours <= 24) {
        return d.toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' })
      }
      return d.toLocaleDateString('fr-FR', { day: '2-digit', month: '2-digit' })
    })
    const data = points.map(p => Number(p.cpu_temperature))
    nodeCpuTempCurrent.value = data[data.length - 1] || 0
    nodeTempChart.value = {
      labels,
      datasets: [{
        data,
        borderColor: '#ef4444',
        backgroundColor: 'rgba(239,68,68,0.12)',
        fill: true,
        tension: 0.3,
        spanGaps: true,
      }],
    }
  } catch (e) {
    nodeTempError.value = e?.response?.data?.error || 'Erreur lors du chargement de la température CPU.'
  } finally {
    nodeTempLoading.value = false
  }
}

async function loadNodeFanRPMHistory(hours = rrdTimeframeToHours[rrdTimeframe.value] ?? 24) {
  nodeFanLoading.value = true
  nodeFanError.value = ''
  nodeFanChart.value = null
  nodeFanRPMCurrent.value = 0

  try {
    const sourceHostId = sensorSourceHostId.value || node.value?.fan_rpm_source_host_id || node.value?.cpu_temp_source_host_id
    if (!sourceHostId) {
      return
    }

    const res = await api.getProxmoxNodeFanRPMHistory(route.params.id, hours)
    const points = (Array.isArray(res.data) ? res.data : []).filter(p => Number(p?.fan_rpm) > 0)
    if (!points.length) {
      return
    }

    const labels = points.map(p => {
      const d = new Date(p.timestamp)
      if (hours <= 24) {
        return d.toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' })
      }
      return d.toLocaleDateString('fr-FR', { day: '2-digit', month: '2-digit' })
    })
    const data = points.map(p => Number(p.fan_rpm))
    nodeFanRPMCurrent.value = data[data.length - 1] || 0
    nodeFanChart.value = {
      labels,
      datasets: [{
        data,
        borderColor: '#2563eb',
        backgroundColor: 'rgba(37,99,235,0.12)',
        fill: true,
        tension: 0.3,
        spanGaps: true,
      }],
    }
  } catch (e) {
    nodeFanError.value = e?.response?.data?.error || 'Erreur lors du chargement des RPM ventilateurs.'
  } finally {
    nodeFanLoading.value = false
  }
}

async function loadSensorSourceCandidates() {
  sensorSourceLoading.value = true
  try {
    const res = await api.getProxmoxNodeSensorSourceCandidates(route.params.id)
    sensorSourceCandidates.value = Array.isArray(res.data) ? res.data : []
  } catch {
    sensorSourceCandidates.value = []
  } finally {
    sensorSourceLoading.value = false
  }
}

async function refreshNodeSensorSource() {
  try {
    const res = await api.getProxmoxNode(route.params.id)
    const n = res.data || {}
    if (node.value) {
      node.value.cpu_temp_source_host_id = n.cpu_temp_source_host_id || ''
      node.value.cpu_temp_source_host_name = n.cpu_temp_source_host_name || ''
      node.value.fan_rpm_source_host_id = n.fan_rpm_source_host_id || ''
      node.value.fan_rpm_source_host_name = n.fan_rpm_source_host_name || ''
    }
    sensorSourceHostId.value = n.cpu_temp_source_host_id || n.fan_rpm_source_host_id || ''
  } catch {
    // non-bloquant
  }
}

async function saveSensorSource() {
  sensorSourceSaving.value = true
  sensorSourceMsg.value = ''
  try {
    const target = sensorSourceHostId.value || null
    const res = await api.setProxmoxNodeSensorSource(route.params.id, target)
    if (node.value) {
      node.value.cpu_temp_source_host_id = res.data?.cpu_temp_source_host_id || ''
      node.value.cpu_temp_source_host_name = res.data?.cpu_temp_source_host_name || ''
      node.value.fan_rpm_source_host_id = res.data?.fan_rpm_source_host_id || ''
      node.value.fan_rpm_source_host_name = res.data?.fan_rpm_source_host_name || ''
    }
    sensorSourceHostId.value = res.data?.cpu_temp_source_host_id || res.data?.fan_rpm_source_host_id || ''
    await loadSensorSourceCandidates()
    await loadNodeCpuTempHistory(rrdTimeframeToHours[rrdTimeframe.value] ?? 24)
    await loadNodeFanRPMHistory(rrdTimeframeToHours[rrdTimeframe.value] ?? 24)
    sensorSourceMsg.value = 'Source capteurs mise à jour (CPU + ventilateurs).'
    sensorSourceOk.value = true
  } catch (e) {
    sensorSourceMsg.value = e?.response?.data?.error || 'Erreur lors de la mise à jour.'
    sensorSourceOk.value = false
  } finally {
    sensorSourceSaving.value = false
    setTimeout(() => { sensorSourceMsg.value = '' }, 4000)
  }
}

async function loadGuestLinks() {
  const guests = node.value?.guests ?? []
  if (guests.length === 0) return
  // One request for all links, then index by guest_id — avoids N individual requests.
  try {
    const res = await api.getProxmoxLinks()
    const guestIds = new Set(guests.map(g => g.id))
    const map = {}
    for (const link of res.data ?? []) {
      if (guestIds.has(link.guest_id)) {
        map[link.guest_id] = link
      }
    }
    guestLinks.value = map
  } catch {
    guestLinks.value = {}
  }
}

function linkForGuest(g) {
  return guestLinks.value[g.id] ?? null
}

async function confirmGuestLink(g) {
  const link = linkForGuest(g)
  if (!link) return
  try {
    const res = await api.updateProxmoxLink(link.id, { status: 'confirmed' })
    guestLinks.value = { ...guestLinks.value, [g.id]: res.data }
    await loadSensorSourceCandidates()
    await refreshNodeSensorSource()
    showMsg(`[${g.name}] Lien confirmé.`, true)
  } catch (e) {
    showMsg(e?.response?.data?.error || 'Erreur.', false)
  }
}

async function ignoreGuestLink(g) {
  const link = linkForGuest(g)
  if (!link) return
  try {
    await api.deleteProxmoxLink(link.id)
    const m = { ...guestLinks.value }
    delete m[g.id]
    guestLinks.value = m
    showMsg(`[${g.name}] Suggestion ignorée.`, true)
  } catch (e) {
    showMsg(e?.response?.data?.error || 'Erreur.', false)
  }
}

function goToHost(link) {
  if (link?.host_id) router.push(`/hosts/${link.host_id}`)
}

function showMsg(msg, ok) {
  linkMsg.value = msg
  linkMsgOk.value = ok
  setTimeout(() => { linkMsg.value = '' }, 4000)
}

async function loadRRD(timeframe = rrdTimeframe.value) {
  rrdTimeframe.value = timeframe
  void loadNodeCpuTempHistory(rrdTimeframeToHours[timeframe] ?? 24)
  void loadNodeFanRPMHistory(rrdTimeframeToHours[timeframe] ?? 24)
  rrdLoading.value = true
  rrdError.value = ''
  try {
    const res = await api.getProxmoxNodeRRD(route.params.id, timeframe)
    buildRRDCharts(res.data ?? [], timeframe)
  } catch (e) {
    rrdError.value = e?.response?.data?.error || 'Erreur lors du chargement des métriques.'
    rrdCpuChart.value = null
    rrdRamChart.value = null
    rrdIowaitChart.value = null
    rrdNetChart.value = null
  } finally {
    rrdLoading.value = false
  }
}

function buildRRDCharts(points, timeframe) {
  const labels = points.map(p => {
    const d = new Date(p.time * 1000)
    if (timeframe === 'hour' || timeframe === 'day')
      return d.toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' })
    if (timeframe === 'week')
      return d.toLocaleDateString('fr-FR', { day: '2-digit', month: '2-digit' }) + ' ' + d.toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' })
    return d.toLocaleDateString('fr-FR', { day: '2-digit', month: '2-digit' })
  })

  rrdCpuChart.value = {
    labels,
    datasets: [{
      data: points.map(p => p.cpu != null ? p.cpu * 100 : null),
      borderColor: '#3b82f6', backgroundColor: 'rgba(59,130,246,0.1)',
      fill: true, tension: 0.3, spanGaps: true,
    }],
  }

  // RAM: memused / memtotal are raw bytes from PVE RRD (JSON keys: memused, memtotal)
  const ramData = points.map(p =>
    (p.memused != null && p.memtotal != null && p.memtotal > 0)
      ? (p.memused / p.memtotal) * 100
      : null
  )
  rrdRamChart.value = ramData.some(v => v != null) ? {
    labels,
    datasets: [{
      data: ramData,
      borderColor: '#10b981', backgroundColor: 'rgba(16,185,129,0.1)',
      fill: true, tension: 0.3, spanGaps: true,
    }],
  } : null

  const hasIowait = points.some(p => p.iowait != null)
  rrdIowaitChart.value = hasIowait ? {
    labels,
    datasets: [{
      data: points.map(p => p.iowait != null ? p.iowait * 100 : null),
      borderColor: '#f59e0b', backgroundColor: 'rgba(245,158,11,0.1)',
      fill: true, tension: 0.3, spanGaps: true,
    }],
  } : null

  const hasNet = points.some(p => p.netin != null || p.netout != null)
  rrdNetChart.value = hasNet ? {
    labels,
    datasets: [
      {
        label: 'Entrante',
        data: points.map(p => p.netin ?? null),
        borderColor: '#6366f1', backgroundColor: 'rgba(99,102,241,0.1)',
        fill: true, tension: 0.3, spanGaps: true,
      },
      {
        label: 'Sortante',
        data: points.map(p => p.netout ?? null),
        borderColor: '#ec4899', backgroundColor: 'rgba(236,72,153,0.05)',
        fill: false, tension: 0.3, spanGaps: true,
      },
    ],
  } : null
}

async function loadLiveStatus() {
  liveStatusLoading.value = true
  liveStatusError.value = ''
  try {
    const res = await api.getProxmoxNodeStatus(route.params.id)
    liveStatus.value = res.data
    liveStatusTime.value = new Date().toLocaleTimeString('fr-FR')
  } catch (e) {
    liveStatusError.value = e?.response?.data?.error || `Erreur ${e?.response?.status ?? ''} — vérifiez la connectivité au nœud.`
  } finally {
    liveStatusLoading.value = false
  }
}

async function loadNodeSecurityEvents() {
  if (securityEventsLoading.value) return
  securityEventsLoading.value = true
  securityEventsError.value = ''
  try {
    if (securityService.value === 'rotate') {
      const services = ['pveproxy', 'sshd', 'pvedaemon']
      const calls = services.map(service =>
        api.getProxmoxNodeSyslog(route.params.id, {
          limit: 120,
          search: securitySearch.value,
          service,
        })
      )
      const results = await Promise.allSettled(calls)
      const groups = results
        .filter(r => r.status === 'fulfilled')
        .map(r => Array.isArray(r.value?.data) ? r.value.data : [])

      if (!groups.length) {
        throw new Error('Aucun service syslog accessible (pveproxy, sshd, pvedaemon).')
      }

      securityEvents.value = mergeAndRankSyslogLines(groups, 200)
    } else {
      const res = await api.getProxmoxNodeSyslog(route.params.id, {
        limit: 200,
        search: securitySearch.value,
        service: securityService.value,
      })
      securityEvents.value = mergeAndRankSyslogLines([Array.isArray(res.data) ? res.data : []], 200)
    }
  } catch (e) {
    securityEventsError.value = e?.response?.data?.error || 'Erreur lors du chargement des événements de sécurité.'
    securityEvents.value = []
  } finally {
    securityEventsLoading.value = false
  }
}

function stopPolling() {
  if (pollTimer) clearTimeout(pollTimer)
  pollTimer = null
}

function closeConsole() {
  stopPolling()
  showConsole.value = false
  liveTask.value = null
  activeUpid.value = null
}

async function startPollingTask(upid, { action = '', label = '' } = {}) {
  stopPolling()
  activeUpid.value = upid
  liveTask.value = {
    host_name: node.value?.node_name ?? '',
    module: 'proxmox',
    action: action || upid,
    target: label || '',   // short display label, not the raw UPID
    status: 'running',
    output: '',
  }
  showConsole.value = true

  const poll = async () => {
    try {
      const res = await api.getProxmoxTaskLog(route.params.id, upid)
      const lines = (res.data ?? []).map(l => l.t).join('\n')
      const lastLine = res.data?.[res.data.length - 1]?.t ?? ''
      const done = lastLine.startsWith('TASK OK') || lastLine.startsWith('TASK ERROR')
      const status = done
        ? (lastLine.startsWith('TASK OK') ? 'completed' : 'failed')
        : 'running'
      liveTask.value = { ...liveTask.value, output: lines, status }
      if (!done) pollTimer = setTimeout(poll, 2000)
    } catch {
      pollTimer = setTimeout(poll, 3000)
    }
  }
  await poll()
}

async function triggerAptRefresh() {
  aptRefreshing.value = true
  aptRefreshMsg.value = ''
  try {
    const res = await api.refreshProxmoxNodeApt(route.params.id)
    const upid = res.data?.upid
    aptRefreshMsg.value = upid ? 'Tâche lancée — logs en cours…' : (res.data?.message || 'Tâche lancée.')
    aptRefreshOk.value = true
    if (upid) startPollingTask(upid, { action: 'apt update' })
  } catch (e) {
    aptRefreshMsg.value = e?.response?.data?.error || 'Erreur lors du lancement de apt update.'
    aptRefreshOk.value = false
  } finally {
    aptRefreshing.value = false
    setTimeout(() => { aptRefreshMsg.value = '' }, 6000)
  }
}

function memPct(n) {
  if (!n.mem_total) return 0
  return ((n.mem_used / n.mem_total) * 100).toFixed(1)
}

function storagePct(s) {
  if (!s.total) return 0
  return ((s.used / s.total) * 100).toFixed(1)
}

function cpuColor(usage) {
  if (usage > 0.85) return 'bg-danger'
  if (usage > 0.6) return 'bg-warning'
  return 'bg-success'
}

function tempColor(temp) {
  if (!temp) return 'text-secondary'
  if (temp >= 85) return 'text-danger'
  if (temp >= 70) return 'text-warning'
  return 'text-success'
}

function tempBadgeClass(temp) {
  if (!temp) return 'bg-secondary-lt text-secondary'
  if (temp >= 85) return 'bg-danger-lt text-danger'
  if (temp >= 70) return 'bg-warning-lt text-warning'
  return 'bg-success-lt text-success'
}

function ramColor(used, total) {
  if (!total) return 'bg-secondary'
  const pct = used / total
  if (pct > 0.85) return 'bg-danger'
  if (pct > 0.6) return 'bg-warning'
  return 'bg-success'
}

function storageColor(used, total) {
  if (!total) return 'bg-secondary'
  const pct = used / total
  if (pct > 0.85) return 'bg-danger'
  if (pct > 0.6) return 'bg-warning'
  return 'bg-primary'
}

function guestStatusClass(status) {
  const map = {
    running: 'badge bg-success-lt text-success',
    stopped: 'badge bg-secondary-lt text-secondary',
    paused: 'badge bg-warning-lt text-warning',
  }
  return map[status] ?? 'badge bg-secondary-lt text-secondary'
}

function formatBytes(bytes) {
  if (!bytes) return '0 B'
  const units = ['B', 'Ko', 'Mo', 'Go', 'To']
  let i = 0, v = bytes
  while (v >= 1024 && i < units.length - 1) { v /= 1024; i++ }
  return `${v.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

function formatUptime(seconds) {
  if (!seconds) return '—'
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (d > 0) return `${d}j ${h}h`
  if (h > 0) return `${h}h ${m}m`
  return `${m}m`
}

function formatDate(iso) {
  if (!iso) return '—'
  return new Date(iso).toLocaleString('fr-FR', { dateStyle: 'short', timeStyle: 'short' })
}

function formatSyslogTime(item) {
  const raw = item?.parsedTimeMs ?? item?.time
  if (!raw) return '—'
  const ms = typeof raw === 'number' ? (raw < 1_000_000_000_000 ? raw * 1000 : raw) : Date.parse(raw)
  if (!Number.isFinite(ms)) return '—'
  return new Date(ms).toLocaleString('fr-FR', { dateStyle: 'short', timeStyle: 'medium' })
}

function syslogLevelBadgeClass(item) {
  const raw = String(item?.parsedLevel || item?.pri || item?.level || '').toLowerCase()
  if (raw.includes('critical') || raw.includes('fatal') || raw.includes('panic')) return 'bg-danger-lt text-danger'
  if (raw.includes('error') || raw.includes('err')) return 'bg-danger-lt text-danger'
  if (raw.includes('warning') || raw.includes('warn')) return 'bg-orange-lt text-orange'
  if (raw.includes('success') || raw.includes('ok')) return 'bg-success-lt text-success'
  if (raw.includes('info') || raw.includes('notice')) return 'bg-azure-lt text-azure'
  return 'bg-secondary-lt text-secondary'
}

// wearout for SSD: 100=new, lower=more worn → invert to show wear percentage
function wearoutColor(wearout) {
  // wearout is wear level remaining (100=new). Low value = danger.
  if (wearout < 20) return 'bg-danger'
  if (wearout < 50) return 'bg-warning'
  return 'bg-success'
}

async function loadServices() {
  if (servicesLoading.value) return
  servicesLoading.value = true
  servicesError.value = ''
  try {
    const res = await api.getProxmoxNodeServices(route.params.id)
    services.value = res.data ?? []
  } catch (e) {
    servicesError.value = e?.response?.data?.error || 'Erreur lors du chargement des services.'
  } finally {
    servicesLoading.value = false
  }
}

async function svcAction(name, action) {
  svcActionMsg.value = ''
  try {
    const res = await api.proxmoxNodeServiceAction(route.params.id, name, action)
    const upid = res.data?.upid
    svcActionMsg.value = upid ? `${action} ${name} lancé — logs en cours…` : `${action} ${name} lancé.`
    svcActionOk.value = true
    if (upid) startPollingTask(upid, { action: `service ${action}`, label: name })
    else setTimeout(() => loadServices(), 2000)
  } catch (e) {
    svcActionMsg.value = e?.response?.data?.error || `Erreur lors de ${action} ${name}.`
    svcActionOk.value = false
  }
  setTimeout(() => { svcActionMsg.value = '' }, 6000)
}

function svcStateClass(state) {
  if (state === 'active') return 'badge bg-green-lt text-green'
  if (state === 'failed') return 'badge bg-red-lt text-red'
  if (state === 'activating' || state === 'deactivating') return 'badge bg-yellow-lt text-yellow'
  return 'badge bg-secondary-lt text-secondary'
}

function taskDuration(t) {
  if (!t.start_time) return '—'
  const end = t.end_time ? new Date(t.end_time) : (t.status === 'running' ? new Date() : null)
  if (!end) return '—'
  const secs = Math.floor((end - new Date(t.start_time)) / 1000)
  if (secs < 60) return `${secs}s`
  const m = Math.floor(secs / 60)
  const s = secs % 60
  if (m < 60) return `${m}m ${s}s`
  const h = Math.floor(m / 60)
  return `${h}h ${m % 60}m`
}

onMounted(() => {
  load()
  liveStatusTimer = setInterval(loadLiveStatus, 60_000)
})
onUnmounted(() => {
  stopPolling()
  if (liveStatusTimer) clearInterval(liveStatusTimer)
})
</script>

<style scoped>
.proxmox-node-tabs {
  flex-wrap: nowrap;
  overflow-x: auto;
  overflow-y: hidden;
  -webkit-overflow-scrolling: touch;
}

.proxmox-node-tabs .nav-item {
  flex: 0 0 auto;
}

.proxmox-source-select {
  max-width: 22.5rem;
}

.proxmox-chart-body {
  height: 11rem;
}

.node-live-meta-text {
  font-size: 0.7rem;
}

.node-live-meta-spinner {
  width: 0.65rem;
  height: 0.65rem;
  border-width: 0.1em;
}

.proxmox-progress-min-60 {
  min-width: 60px;
}

.proxmox-progress-min-80 {
  min-width: 80px;
}

.proxmox-service-desc {
  max-width: 240px;
}

.proxmox-security-service-select {
  max-width: 11rem;
}

.proxmox-security-search {
  max-width: 18rem;
}

@media (max-width: 992px) {
  .node-live-meta {
    position: static !important;
    margin-top: 0.75rem;
    padding: 0;
    justify-content: flex-end;
    width: 100%;
  }

  .proxmox-security-service-select,
  .proxmox-security-search {
    max-width: 100%;
    width: 100%;
  }
}

@media (max-width: 768px) {
  .proxmox-node-tabs .nav-link {
    white-space: nowrap;
    padding-inline: 0.6rem;
  }
}
</style>
