<template>
  <div class="host-detail-page">
    <div class="page-header mb-3">
      <div class="d-flex flex-column flex-lg-row align-items-lg-center justify-content-between gap-3">
        <div>
          <div class="page-pretitle">
            <router-link to="/" class="text-decoration-none">Dashboard</router-link>
            <span class="text-muted mx-1">/</span>
            <span>Hôte</span>
          </div>
          <h2 class="page-title">{{ host?.name || host?.hostname || 'Chargement...' }}</h2>
          <div class="text-secondary">
            {{ host?.hostname || 'Non connecté' }} — {{ host?.os || 'OS inconnu' }} • {{ host?.ip_address }}
            <span v-if="host?.last_seen">• Dernière activité: <RelativeTime :date="host.last_seen" /></span>
          </div>
        </div>
        <div class="d-flex align-items-center gap-2">
          <button @click="startEdit" class="btn btn-outline-secondary">
            <svg class="icon me-1" width="16" height="16" viewBox="0 0 24 24" stroke="currentColor" fill="none" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"/>
              <path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"/>
            </svg>
            Modifier
          </button>
          <button @click="deleteHost" class="btn btn-outline-danger">
            <svg class="icon me-1" width="16" height="16" viewBox="0 0 24 24" stroke="currentColor" fill="none" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M3 6h18"/><path d="M8 6V4h8v2"/><path d="M19 6l-1 14a2 2 0 01-2 2H8a2 2 0 01-2-2L5 6"/>
            </svg>
            Supprimer
          </button>
          <span v-if="host" :class="hostStatusClass(host.status)">{{ formatHostStatus(host.status) }}</span>
          <span v-if="host?.agent_version" :class="isAgentUpToDate(host.agent_version) ? 'badge bg-green-lt text-green' : 'badge bg-yellow-lt text-yellow'" :title="isAgentUpToDate(host.agent_version) ? 'Agent à jour' : 'Mise à jour disponible'">Agent v{{ host.agent_version }}</span>
        </div>
      </div>
    </div>

    <WsStatusBar :status="wsStatus" :error="wsError" :retry-count="retryCount" @reconnect="reconnect" />

    <div class="host-layout">
      <!-- Colonne gauche: Informations hôte -->
      <div class="host-panel-main">

    <div v-if="isEditing" class="card mb-4">
      <div class="card-header">
        <h3 class="card-title">Modifier l'hôte</h3>
      </div>
      <div class="card-body">
        <form @submit.prevent="saveEdit" class="row g-3">
          <div class="col-md-6">
            <label class="form-label">Nom</label>
            <input v-model="editForm.name" type="text" class="form-control" required />
          </div>
          <div class="col-md-6">
            <label class="form-label">Hostname</label>
            <input v-model="editForm.hostname" type="text" class="form-control" required />
          </div>
          <div class="col-md-6">
            <label class="form-label">Adresse IP</label>
            <input v-model="editForm.ip_address" type="text" class="form-control" required />
          </div>
          <div class="col-md-6">
            <label class="form-label">OS</label>
            <input v-model="editForm.os" type="text" class="form-control" required />
          </div>
          <div v-if="editError" class="col-12">
            <div class="alert alert-danger py-2 mb-0">{{ editError }}</div>
          </div>
          <div class="col-12 d-flex justify-content-end gap-2">
            <button type="button" @click="cancelEdit" class="btn btn-outline-secondary" :disabled="saving">Annuler</button>
            <button type="submit" class="btn btn-primary" :disabled="saving">
              {{ saving ? 'Enregistrement...' : 'Enregistrer' }}
            </button>
          </div>
          <div class="col-12">
            <div class="border-top pt-3 mt-2">
              <div class="d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-2">
                <div>
                  <div class="fw-semibold">API Key agent</div>
                  <div class="text-secondary small">Régénérer la clé pour un hôte existant.</div>
                </div>
                <button type="button" class="btn btn-outline-warning" :disabled="rotateKeyLoading" @click="rotateHostKey">
                  {{ rotateKeyLoading ? 'Rotation...' : 'Régénérer la clé' }}
                </button>
              </div>
              <div v-if="rotateKeyResult" class="alert alert-info mt-3 mb-0" role="alert">
                <div class="fw-semibold mb-2">Nouvelle cle generee</div>
                <div class="text-secondary small mb-2">Copiez-la maintenant, elle ne sera plus affichee.</div>
                <div class="d-flex align-items-center gap-2 mb-2">
                  <div class="bg-dark rounded p-2 flex-fill">
                    <code class="text-light">{{ rotateKeyResult.api_key }}</code>
                  </div>
                  <button type="button" class="btn btn-outline-light" @click="copyRotatedKey">
                    {{ rotateCopiedKey ? 'Copie' : 'Copier' }}
                  </button>
                </div>
                <div class="d-flex align-items-center justify-content-between mb-1">
                  <div class="text-secondary small">Configuration agent :</div>
                  <button type="button" class="btn btn-outline-light btn-sm" @click="copyRotatedConfig">
                    {{ rotateCopiedConfig ? 'Copie' : 'Copier la config' }}
                  </button>
                </div>
                <pre class="bg-dark text-light p-2 rounded small mb-0">{{ rotatedAgentConfig }}</pre>
              </div>
            </div>
          </div>
        </form>
      </div>
    </div>

    <!-- Navigation par onglets -->
    <ul class="nav nav-tabs mb-3">
      <li class="nav-item">
        <a class="nav-link" :class="{ active: activeTab === 'metrics' }" href="#" @click.prevent="activeTab = 'metrics'">Métriques</a>
      </li>
      <li class="nav-item">
        <a class="nav-link" :class="{ active: activeTab === 'docker' }" href="#" @click.prevent="activeTab = 'docker'">
          Docker
          <span v-if="containers.length" class="badge bg-blue-lt text-blue ms-1">{{ containers.length }}</span>
        </a>
      </li>
      <li class="nav-item">
        <a class="nav-link" :class="{ active: activeTab === 'apt' }" href="#" @click.prevent="activeTab = 'apt'">
          APT
          <span v-if="aptStatus?.pending_packages > 0" class="badge bg-yellow-lt text-yellow ms-1">{{ aptStatus.pending_packages }}</span>
        </a>
      </li>
      <li class="nav-item">
        <a class="nav-link" :class="{ active: activeTab === 'commandes' }" href="#" @click.prevent="activeTab = 'commandes'">
          Commandes
          <span v-if="combinedHistoryTotal > 0" class="badge bg-secondary-lt text-secondary ms-1">{{ combinedHistoryTotal }}</span>
        </a>
      </li>
      <template v-if="canRunApt">
        <li class="nav-item">
          <a class="nav-link" :class="{ active: activeTab === 'systeme' }" href="#" @click.prevent="activeTab = 'systeme'">Système</a>
        </li>
        <li class="nav-item">
          <a class="nav-link" :class="{ active: activeTab === 'processus' }" href="#" @click.prevent="activeTab = 'processus'">Processus</a>
        </li>
      </template>
      <li class="nav-item">
        <a class="nav-link" :class="{ active: activeTab === 'planifiees' }" href="#" @click.prevent="switchToTasks()">
          Tâches planifiées
          <span v-if="tasks.length" class="badge bg-secondary-lt text-secondary ms-1">{{ tasks.length }}</span>
        </a>
      </li>
    </ul>

    <!-- ═══ ONGLET MÉTRIQUES ═══ -->
    <div v-show="activeTab === 'metrics'">
    <HostMetricsPanel :hostId="hostId" :metrics="metrics" />

    <!-- Disk Metrics Card - Filesystem Usage -->
    <DiskMetricsCard :hostId="hostId" class="mb-4" />

    <!-- Disk Health Card - SMART Data -->
    <DiskHealthCard :hostId="hostId" class="mb-4" />
    </div><!-- /onglet Métriques -->

    <!-- ═══ ONGLET DOCKER ═══ -->
    <div v-show="activeTab === 'docker'">
    <div class="card mb-4">
      <div class="card-header">
        <h3 class="card-title">Conteneurs Docker <span v-if="containers.length">({{ containers.length }})</span></h3>
      </div>
      <div class="table-responsive">
        <table class="table table-vcenter card-table">
          <thead>
            <tr>
              <th>Nom</th>
              <th>Image</th>
              <th>Tag</th>
              <th>État</th>
              <th>Status</th>
              <th>Ports</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="c in containers" :key="c.id">
              <td class="fw-semibold">{{ c.name }}</td>
              <td class="text-secondary">{{ c.image }}</td>
              <td>
                <code>{{ c.image_tag }}</code>
                <template v-if="containerVersion(c)">
                  <br>
                  <span v-if="containerVersion(c).is_up_to_date" class="badge bg-green-lt text-green mt-1">À jour</span>
                  <span v-else-if="!containerVersion(c).running_version" class="badge bg-secondary-lt text-secondary mt-1">Version inconnue</span>
                  <span v-else class="badge bg-yellow-lt text-yellow mt-1" :title="`Dernière : ${containerVersion(c).latest_version}`">MAJ dispo</span>
                </template>
              </td>
              <td>
                <span :class="c.state === 'running' ? 'badge bg-green-lt text-green' : 'badge bg-secondary-lt text-secondary'">
                  {{ { running: 'En cours', exited: 'Arrêté', paused: 'En pause', created: 'Créé', restarting: 'Redémarrage', dead: 'Mort' }[c.state] || c.state }}
                </span>
              </td>
              <td class="text-secondary small">{{ c.status }}</td>
              <td class="text-secondary small font-monospace">{{ c.ports || '-' }}</td>
            </tr>
            <tr v-if="!containers.length">
              <td colspan="6" class="text-center text-secondary py-4">Aucun conteneur Docker actif sur cet hôte.</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
    </div><!-- /onglet Docker -->

    <!-- ═══ ONGLET APT ═══ -->
    <div v-show="activeTab === 'apt'">
    <div v-if="aptStatus" class="card">
      <div class="card-header d-flex align-items-center justify-content-between">
        <h3 class="card-title">APT - Mises à jour système</h3>
        <div class="btn-group btn-group-sm" v-if="canRunApt">
          <button @click="sendAptCmd('update')" class="btn btn-outline-secondary" :disabled="!!aptCmdLoading">
            <span v-if="aptCmdLoading === 'update'" class="spinner-border spinner-border-sm me-1"></span>
            apt update
          </button>
          <button @click="sendAptCmd('upgrade')" class="btn btn-primary" :disabled="!!aptCmdLoading">
            <span v-if="aptCmdLoading === 'upgrade'" class="spinner-border spinner-border-sm me-1"></span>
            apt upgrade
          </button>
          <button @click="sendAptCmd('dist-upgrade')" class="btn btn-outline-danger" :disabled="!!aptCmdLoading">
            <span v-if="aptCmdLoading === 'dist-upgrade'" class="spinner-border spinner-border-sm me-1"></span>
            apt dist-upgrade
          </button>
        </div>
        <span v-else class="text-secondary small">Mode lecture seule</span>
      </div>
      <div class="card-body">
        <div class="row row-cards">
          <div class="col-md-4">
            <div class="card card-sm">
              <div class="card-body text-center">
                <div class="h2 mb-0" :class="aptStatus.pending_packages > 0 ? 'text-yellow' : 'text-green'">
                  {{ aptStatus.pending_packages }}
                </div>
                <div class="text-secondary small">Paquets en attente</div>
              </div>
            </div>
          </div>
          <div class="col-md-4">
            <div class="card card-sm">
              <div class="card-body text-center">
                <div class="h2 mb-0 text-red">{{ aptStatus.security_updates }}</div>
                <div class="text-secondary small">Mises à jour sécurité</div>
              </div>
            </div>
          </div>
          <div class="col-md-4">
            <div class="card card-sm">
              <div class="card-body text-center">
                <div class="fw-semibold">{{ formatDate(aptStatus.last_update) }}</div>
                <div class="text-secondary small">Dernière mise à jour</div>
              </div>
            </div>
          </div>
        </div>
        
        <!-- CVE Information -->
        <div v-if="aptStatus.cve_list" class="mt-3">
          <CVEList 
            :cveList="aptStatus.cve_list" 
            :showMaxSeverity="true"
            :alwaysExpanded="false"
            :limit="5"
          />
        </div>
      </div>
    </div>
    <div v-if="!aptStatus" class="card"><div class="card-body text-secondary">Données APT non disponibles pour cet hôte.</div></div>
    </div><!-- /onglet APT -->

    <!-- ═══ ONGLET COMMANDES ═══ -->
    <div v-show="activeTab === 'commandes'">
    <div class="card mt-0">
      <div class="card-header">
        <h3 class="card-title">Historique de commandes</h3>
        <div class="card-options">
          <span class="badge bg-secondary-lt text-secondary">{{ showFullHistory ? combinedHistoryTotal : combinedHistory.length }}</span>
        </div>
      </div>
      <div class="table-responsive">
        <table class="table table-vcenter card-table">
          <thead>
            <tr>
              <th>Date</th>
              <th>Type</th>
              <th>Commande</th>
              <th>Statut</th>
              <th>Utilisateur</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="cmd in combinedHistory" :key="cmd.id">
              <td class="text-secondary small">{{ formatDate(cmd.created_at) }}</td>
              <td>
                <span :class="cmdModuleClass(cmd.module)">{{ cmdModuleLabel(cmd.module) }}</span>
              </td>
              <td>
                <code class="small">{{ cmdLabel(cmd) }}</code>
              </td>
              <td><span :class="statusClass(cmd.status)">{{ cmd.status }}</span></td>
              <td class="text-secondary small">{{ cmd.triggered_by || '-' }}</td>
              <td>
                <button
                  @click="watchCommand(cmd)"
                  class="btn btn-sm btn-ghost-secondary"
                  title="Voir les logs"
                >
                  <svg class="icon icon-sm" width="16" height="16" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" xmlns="http://www.w3.org/2000/svg"><path stroke="none" d="M0 0h24v24H0z" fill="none"/><path d="M4 6l16 0" /><path d="M4 12l16 0" /><path d="M4 18l12 0" /></svg>
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div v-if="combinedHistoryTotal > 5 && !showFullHistory" class="card-footer text-center">
        <button @click="showFullHistory = true" class="btn btn-outline-secondary btn-sm">
          Afficher tout ({{ combinedHistoryTotal - 5 }} autres)
        </button>
      </div>
    </div>
    <div v-if="!combinedHistoryTotal" class="card"><div class="card-body text-secondary">Aucune commande exécutée sur cet hôte.</div></div>
    </div><!-- /onglet Commandes -->

    <!-- ═══ ONGLET SYSTÈME ═══ -->
    <div v-if="canRunApt" v-show="activeTab === 'systeme'">
    <!-- Logs système (journalctl) -->
    <div class="card mb-4">
      <div class="card-header">
        <h3 class="card-title">Logs système (journalctl)</h3>
      </div>
      <div class="card-body">
        <div class="d-flex gap-2">
          <input
            v-model="journalService"
            type="text"
            class="form-control"
            placeholder="Nom du service (ex: nginx, ssh, docker)"
            @keyup.enter="loadJournalLogs"
            style="max-width: 320px;"
          />
          <button
            class="btn btn-primary"
            @click="loadJournalLogs"
            :disabled="!journalService.trim() || journalLoading"
          >
            <span v-if="journalLoading" class="spinner-border spinner-border-sm me-1"></span>
            {{ journalLoading ? 'Chargement...' : 'Charger les logs' }}
          </button>
        </div>
        <div v-if="journalError" class="alert alert-danger mt-3 mb-0">{{ journalError }}</div>
        <div v-if="journalCmdId" class="text-secondary small mt-2">
          Stream → commande #{{ journalCmdId }} — les logs apparaissent dans la Console Live →
        </div>
      </div>
    </div>

    <HostSystemdPanel :hostId="hostId" :can-run="canRunApt" @open-console="openConsoleFromPanel" @history-changed="loadCmdHistory" />

    </div><!-- /onglet Système -->

    <!-- ═══ ONGLET PROCESSUS ═══ -->
    <div v-if="canRunApt" v-show="activeTab === 'processus'">
    <HostProcessesPanel :hostId="hostId" :can-run="canRunApt" @history-changed="loadCmdHistory" />
    </div><!-- /onglet Processus -->

    <!-- ═══ ONGLET TÂCHES PLANIFIÉES ═══ -->
    <div v-show="activeTab === 'planifiees'">
    <div class="d-flex justify-content-between align-items-center mb-3">
      <div v-if="tasksError" class="alert alert-danger mb-0 flex-fill me-3">{{ tasksError }}</div>
      <div v-else class="flex-fill"></div>
      <button v-if="canRunApt" class="btn btn-primary" @click="openCreateTask">
        Nouvelle tâche
      </button>
    </div>
    <div class="card">
      <div v-if="tasksLoading" class="card-body text-center py-5">
        <span class="spinner-border text-primary"></span>
      </div>
      <div v-else-if="!tasks.length" class="card-body text-center py-5">
        <svg xmlns="http://www.w3.org/2000/svg" class="icon mb-3 text-muted" width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/>
        </svg>
        <h3 class="mb-1">Aucune tâche planifiée</h3>
        <p class="text-secondary mb-3">Automatisez vos opérations en créant une tâche planifiée.</p>
        <button v-if="canRunApt" class="btn btn-primary" @click="openCreateTask">Nouvelle tâche</button>
      </div>
      <div v-else class="table-responsive">
        <table class="table table-vcenter table-hover card-table mb-0">
          <thead>
            <tr>
              <th>Nom</th>
              <th>Module / Action</th>
              <th>Planification</th>
              <th>Prochaine exécution</th>
              <th>Dernier résultat</th>
              <th>Activée</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="task in tasks" :key="task.id">
              <td>{{ task.name }}</td>
              <td>
                <span class="badge bg-blue-lt me-1">{{ task.module }}</span>
                <span class="text-secondary small">{{ task.action }}</span>
                <span v-if="task.target" class="text-muted small ms-1">— {{ task.target }}</span>
              </td>
              <td>
                <span v-if="isManualOnly(task)" class="badge bg-secondary-lt text-secondary">Manuel</span>
                <template v-else>
                  <code class="small">{{ task.cron_expression }}</code>
                  <span v-if="describeCron(task.cron_expression)" class="text-muted small ms-1">— {{ describeCron(task.cron_expression) }}</span>
                </template>
              </td>
              <td>
                <span v-if="task.next_run_at && !isManualOnly(task)">{{ formatTaskDate(task.next_run_at) }}</span>
                <span v-else class="text-muted">—</span>
              </td>
              <td>
                <span v-if="task.last_run_status"
                  :class="task.last_run_status === 'completed' ? 'badge bg-success-lt' : 'badge bg-danger-lt'">
                  {{ task.last_run_status }}
                  <span v-if="task.last_run_at" class="ms-1 text-muted small">{{ formatTaskDate(task.last_run_at) }}</span>
                </span>
                <span v-else class="text-muted">jamais</span>
              </td>
              <td>
                <input v-if="canRunApt && !isManualOnly(task)" type="checkbox" class="form-check-input"
                  :checked="task.enabled" @change="toggleTask(task)" />
                <span v-else-if="isManualOnly(task)" class="text-muted small">—</span>
                <span v-else>{{ task.enabled ? 'Oui' : 'Non' }}</span>
              </td>
              <td class="text-end">
                <div class="d-flex gap-1 justify-content-end">
                  <button v-if="task.last_command_id" class="btn btn-sm btn-ghost-secondary" title="Voir les logs" @click="openTaskLogs(task)">
                    <svg class="icon icon-sm" width="16" height="16" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none"><path stroke="none" d="M0 0h24v24H0z" fill="none"/><path d="M4 6l16 0" /><path d="M4 12l16 0" /><path d="M4 18l12 0" /></svg>
                  </button>
                  <button v-if="canRunApt" class="btn btn-sm btn-outline-primary"
                    :disabled="taskRunningId === task.id" @click="runTaskNow(task)">
                    <span v-if="taskRunningId === task.id" class="spinner-border spinner-border-sm"></span>
                    <span v-else>Exécuter</span>
                  </button>
                  <button v-if="canRunApt" class="btn btn-sm btn-outline-secondary" @click="openEditTask(task)">Modifier</button>
                  <button v-if="canRunApt" class="btn btn-sm btn-outline-danger" @click="confirmDeleteTask(task)">Supprimer</button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Create / Edit modal -->
    <div v-if="showTaskModal" class="modal modal-blur show d-block" tabindex="-1" style="background: rgba(0,0,0,.5)">
      <div class="modal-dialog modal-dialog-centered modal-lg">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ editingTask ? 'Modifier la tâche' : 'Nouvelle tâche planifiée' }}</h5>
            <button type="button" class="btn-close" @click="closeTaskModal"></button>
          </div>
          <div class="modal-body">
            <div v-if="taskModalError" class="alert alert-danger">{{ taskModalError }}</div>
            <div class="mb-3">
              <label class="form-label">Nom</label>
              <input v-model="taskForm.name" type="text" class="form-control" placeholder="Mise à jour APT hebdomadaire" />
            </div>
            <div class="row g-3 mb-3">
              <div class="col">
                <label class="form-label">Module</label>
                <select v-model="taskForm.module" class="form-select" @change="onTaskModuleChange">
                  <option value="apt">apt</option>
                  <option value="docker">docker</option>
                  <option value="systemd">systemd</option>
                  <option value="journal">journal</option>
                  <option value="processes">processes</option>
                  <option value="custom">custom</option>
                </select>
              </div>
              <div class="col">
                <label class="form-label">Action</label>
                <select v-if="taskModuleActions[taskForm.module]" v-model="taskForm.action" class="form-select">
                  <option v-for="a in taskModuleActions[taskForm.module]" :key="a" :value="a">{{ a }}</option>
                </select>
                <input v-else v-model="taskForm.action" type="text" class="form-control" placeholder="run" />
              </div>
            </div>
            <div v-if="taskForm.module !== 'apt' && taskForm.module !== 'processes'" class="mb-3">
              <label class="form-label">{{ taskForm.module === 'custom' ? 'Tâche (tasks.yaml)' : 'Cible' }}</label>
              <template v-if="taskForm.module === 'custom'">
                <select v-if="customTaskOptions.length" v-model="taskForm.target" class="form-select">
                  <option value="" disabled>-- Sélectionner une tâche --</option>
                  <option v-for="t in customTaskOptions" :key="t.id" :value="t.id">{{ t.name }} ({{ t.id }})</option>
                </select>
                <template v-else>
                  <input v-model="taskForm.target" type="text" class="form-control" placeholder="cleanup_logs" />
                  <div class="form-hint">Aucune tâche détectée dans <code>tasks.yaml</code> — saisissez l'ID manuellement.</div>
                </template>
              </template>
              <input v-else v-model="taskForm.target" type="text" class="form-control" placeholder="nginx.service" />
            </div>
            <div class="mb-3">
              <label class="form-check form-switch">
                <input v-model="taskManualOnly" type="checkbox" class="form-check-input" />
                <span class="form-check-label">Exécution manuelle uniquement (pas de planification automatique)</span>
              </label>
            </div>
            <div v-if="!taskManualOnly" class="mb-3">
              <CronBuilder v-model="taskForm.cron_expression" />
            </div>
            <div class="form-check form-switch mb-1" v-if="!taskManualOnly">
              <input v-model="taskForm.enabled" type="checkbox" class="form-check-input" id="taskEnabled" />
              <label class="form-check-label" for="taskEnabled">Activée</label>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-outline-secondary" @click="closeTaskModal">Annuler</button>
            <button type="button" class="btn btn-primary" :disabled="taskSaving" @click="saveTask">
              <span v-if="taskSaving" class="spinner-border spinner-border-sm me-1"></span>
              {{ editingTask ? 'Enregistrer' : 'Créer' }}
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Run result toast -->
    <div v-if="taskRunResult" class="position-fixed bottom-0 end-0 p-3" style="z-index:1100">
      <div class="toast show align-items-center text-bg-success border-0">
        <div class="d-flex">
          <div class="toast-body">
            <strong>{{ taskRunResult.name }}</strong> déclenchée — commande <code>{{ taskRunResult.id }}</code>
          </div>
          <button type="button" class="btn-close btn-close-white me-2 m-auto" @click="taskRunResult = null"></button>
        </div>
      </div>
    </div>
    </div><!-- /onglet Tâches planifiées -->

      </div><!-- /host-panel-main -->

      <!-- Colonne droite: Console Live -->
      <div 
        v-show="showConsole"
        class="host-panel-right"
      >
        <div class="card" style="display: flex; flex-direction: column; height: 100%;">
          <div class="card-header d-flex align-items-center justify-content-between">
            <h3 class="card-title">
              <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler me-1" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                <path stroke="none" d="M0 0h24v24H0z" fill="none"/>
                <path d="M8 9l3 3l-3 3" />
                <path d="M13 15l3 0" />
                <path d="M3 4m0 2a2 2 0 0 1 2 -2h14a2 2 0 0 1 2 2v12a2 2 0 0 1 -2 2h-14a2 2 0 0 1 -2 -2z" />
              </svg>
              Console Live
            </h3>
            <div class="d-flex gap-1">
              <button
                @click="copyConsoleOutput"
                class="btn btn-sm btn-ghost-secondary"
                :title="consoleCopied ? 'Copié !' : 'Copier la sortie'"
                :disabled="!liveCommand"
              >
                <svg v-if="!consoleCopied" xmlns="http://www.w3.org/2000/svg" class="icon" width="18" height="18" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                  <path stroke="none" d="M0 0h24v24H0z" fill="none"/>
                  <path d="M8 8m0 2a2 2 0 0 1 2 -2h8a2 2 0 0 1 2 2v8a2 2 0 0 1 -2 2h-8a2 2 0 0 1 -2 -2z" />
                  <path d="M16 8v-2a2 2 0 0 0 -2 -2h-8a2 2 0 0 0 -2 2v8a2 2 0 0 0 2 2h2" />
                </svg>
                <svg v-else xmlns="http://www.w3.org/2000/svg" class="icon text-success" width="18" height="18" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                  <path stroke="none" d="M0 0h24v24H0z" fill="none"/>
                  <path d="M5 12l5 5l10 -10" />
                </svg>
              </button>
              <button
                @click="downloadConsoleOutput"
                class="btn btn-sm btn-ghost-secondary"
                title="Télécharger (.txt)"
                :disabled="!liveCommand"
              >
                <svg xmlns="http://www.w3.org/2000/svg" class="icon" width="18" height="18" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                  <path stroke="none" d="M0 0h24v24H0z" fill="none"/>
                  <path d="M4 17v2a2 2 0 0 0 2 2h12a2 2 0 0 0 2 -2v-2" />
                  <path d="M7 11l5 5l5 -5" />
                  <path d="M12 4l0 12" />
                </svg>
              </button>
              <button
                @click="clearConsoleOutput"
                class="btn btn-sm btn-ghost-secondary"
                title="Vider la console"
                :disabled="!liveCommand"
              >
                <svg xmlns="http://www.w3.org/2000/svg" class="icon" width="18" height="18" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                  <path stroke="none" d="M0 0h24v24H0z" fill="none"/>
                  <path d="M4 7h16" /><path d="M5 7l1 12a2 2 0 0 0 2 2h8a2 2 0 0 0 2 -2l1 -12" />
                  <path d="M9 7v-3a1 1 0 0 1 1 -1h4a1 1 0 0 1 1 1v3" />
                </svg>
              </button>
              <button
                @click="closeLiveConsole(); showConsole = false"
                class="btn btn-sm btn-ghost-secondary"
                title="Masquer la console"
              >
                <svg xmlns="http://www.w3.org/2000/svg" class="icon" width="18" height="18" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                  <path stroke="none" d="M0 0h24v24H0z" fill="none"/>
                  <path d="M17 6l-10 10" />
                  <path d="M7 6l10 10" />
                </svg>
              </button>
            </div>
          </div>
          <div class="card-body d-flex flex-column" style="flex: 1; min-height: 0; padding: 0;">
            <!-- État vide -->
            <div v-if="!liveCommand" class="d-flex align-items-center justify-content-center flex-fill text-secondary" style="background: #1e293b; border-radius: 0 0 0.5rem 0.5rem;">
              <div class="text-center p-4">
                <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler mb-2" width="48" height="48" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round" style="opacity: 0.5;">
                  <path stroke="none" d="M0 0h24v24H0z" fill="none"/>
                  <path d="M8 9l3 3l-3 3" />
                  <path d="M13 15l3 0" />
                  <path d="M3 4m0 2a2 2 0 0 1 2 -2h14a2 2 0 0 1 2 2v12a2 2 0 0 1 -2 2h-14a2 2 0 0 1 -2 -2z" />
                </svg>
                <div style="opacity: 0.7;">Aucune console active</div>
                <div class="small mt-1" style="opacity: 0.5;">Cliquez sur "Voir les logs" pour afficher la sortie d'une commande</div>
              </div>
            </div>

            <!-- Console active -->
            <div v-else style="display: flex; flex-direction: column; height: 100%;">
              <div class="px-3 pt-3 pb-2" style="background: #1e293b; border-bottom: 1px solid rgba(255,255,255,0.1);">
                <div class="d-flex align-items-start justify-content-between mb-2">
                  <div class="flex-fill" style="min-width: 0;">
                    <div class="fw-semibold text-light" style="font-size: 0.95rem;">{{ host?.hostname || 'Hôte' }}</div>
                    <div class="text-secondary small mt-1">
                      <code style="background: rgba(0,0,0,0.3); padding: 0.15rem 0.4rem; border-radius: 0.25rem; color: #94a3b8;">{{ liveCommand.prefix }}{{ liveCommand.command }}</code>
                    </div>
                  </div>
                  <span :class="statusClass(liveCommand.status)" style="margin-left: 0.5rem;">{{ liveCommand.status }}</span>
                </div>
              </div>
              <pre
                ref="consoleOutput"
                class="console-output mb-0 flex-fill"
                v-html="colorizedOutput || '<span style=\'opacity:0.5\'>En attente de sortie...</span>'"
              ></pre>
            </div>
          </div>
        </div>
      </div>

      <!-- Bouton pour afficher la console quand cachée -->
      <button
        v-show="!showConsole"
        @click="showConsole = true"
        class="btn btn-primary"
        style="position: fixed; bottom: 1.5rem; right: 1.5rem; z-index: 100;"
      >
        <svg xmlns="http://www.w3.org/2000/svg" class="icon me-1" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
          <path stroke="none" d="M0 0h24v24H0z" fill="none"/>
          <path d="M8 9l3 3l-3 3" />
          <path d="M13 15l3 0" />
          <path d="M3 4m0 2a2 2 0 0 1 2 -2h14a2 2 0 0 1 2 2v12a2 2 0 0 1 -2 2h-14a2 2 0 0 1 -2 -2z" />
        </svg>
        Console
      </button>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, computed, nextTick, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import RelativeTime from '../components/RelativeTime.vue'
import CVEList from '../components/CVEList.vue'
import DiskMetricsCard from '../components/DiskMetricsCard.vue'
import DiskHealthCard from '../components/DiskHealthCard.vue'
import apiClient from '../api'
import { useAuthStore } from '../stores/auth'
import { useConfirmDialog } from '../composables/useConfirmDialog'
import { useWebSocket } from '../composables/useWebSocket'
import WsStatusBar from '../components/WsStatusBar.vue'
import HostMetricsPanel from '../components/HostMetricsPanel.vue'
import HostSystemdPanel from '../components/HostSystemdPanel.vue'
import HostProcessesPanel from '../components/HostProcessesPanel.vue'
import CronBuilder from '../components/CronBuilder.vue'
import { formatHostStatus, hostStatusClass } from '../utils/formatHostStatus'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import utc from 'dayjs/plugin/utc'
import 'dayjs/locale/fr'

dayjs.extend(relativeTime)
dayjs.extend(utc)
dayjs.locale('fr')

const latestAgentVersion = ref('')
const activeTab = ref('metrics')

const route = useRoute()
const router = useRouter()
const hostId = route.params.id

const host = ref(null)
const metrics = ref(null)
const containers = ref([])
const versionComparisons = ref([])
const versionMap = computed(() => {
  const m = {}
  for (const vc of versionComparisons.value) {
    m[vc.docker_image] = vc
  }
  return m
})
function containerVersion(c) {
  return versionMap.value[c.image] || versionMap.value[c.image + ':' + c.image_tag] || null
}
const aptStatus = ref(null)
const showFullHistory = ref(false)
const auditLogs = ref([])
const isEditing = ref(false)
const saving = ref(false)
const editForm = ref({ name: '', hostname: '', ip_address: '', os: '' })
const rotateKeyLoading = ref(false)
const rotateKeyResult = ref(null)
const rotateCopiedKey = ref(false)
const rotateCopiedConfig = ref(false)
const liveCommand = ref(null)
const consoleOutput = ref(null)
const showConsole = ref(false)
let streamWs = null
const journalService = ref('')

const journalLoading = ref(false)
const journalError = ref('')
const journalCmdId = ref(null)
const auth = useAuthStore()
const dialog = useConfirmDialog()
const canRunApt = computed(() => auth.role === 'admin' || auth.role === 'operator')
const aptCmdLoading = ref('')

const serverHostname =
  typeof window !== 'undefined' && window.location?.hostname
    ? window.location.hostname
    : 'localhost'

const rotatedAgentConfig = computed(() => {
  if (!rotateKeyResult.value) return ''
  return `server_url: "http://${serverHostname}:8080"\napi_key: "${rotateKeyResult.value.api_key}"\nreport_interval: 30\ncollect_docker: true\ncollect_apt: true`
})

const consoleCopied = ref(false)

const colorizedOutput = computed(() => {
  if (!liveCommand.value) return ''
  const raw = liveCommand.value.output || ''
  const plain = renderConsoleOutput(raw)
  if (!plain) return ''
  return plain
    .split('\n')
    .map(line => {
      const escaped = line.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
      const lower = line.toLowerCase()
      if (/\berror\b|err:|erreur|failed|échec|exception/i.test(lower))
        return `<span style="color:var(--tblr-danger)">${escaped}</span>`
      if (/\bwarn(?:ing)?\b|attention|deprecated/i.test(lower))
        return `<span style="color:var(--tblr-warning)">${escaped}</span>`
      if (/\bsuccess\b|done|ok\b|completed|✓/i.test(lower))
        return `<span style="color:var(--tblr-success)">${escaped}</span>`
      return escaped
    })
    .join('\n')
})

function copyConsoleOutput() {
  if (!liveCommand.value) return
  const plain = renderConsoleOutput(liveCommand.value.output || '')
  navigator.clipboard.writeText(plain).then(() => {
    consoleCopied.value = true
    setTimeout(() => { consoleCopied.value = false }, 2000)
  })
}

function downloadConsoleOutput() {
  if (!liveCommand.value) return
  const plain = renderConsoleOutput(liveCommand.value.output || '')
  const blob = new Blob([plain], { type: 'text/plain' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `console-${liveCommand.value.command || 'output'}.txt`
  a.click()
  URL.revokeObjectURL(url)
}

function clearConsoleOutput() {
  if (!liveCommand.value) return
  liveCommand.value = { ...liveCommand.value, output: '' }
}

const { wsStatus, wsError, retryCount, reconnect } = useWebSocket(`/api/v1/ws/hosts/${hostId}`, (payload) => {
  if (payload.type !== 'host_detail') return
  host.value = payload.host
  metrics.value = payload.metrics
  containers.value = payload.containers || []
  versionComparisons.value = payload.version_comparisons || []
  aptStatus.value = payload.apt_status
  auditLogs.value = payload.audit_logs || []
}, { debounceMs: 200 })

const cmdHistory = ref([])

async function loadCmdHistory() {
  try {
    const res = await apiClient.getHostCommandHistory(hostId)
    cmdHistory.value = res.data?.commands || []
  } catch {
    cmdHistory.value = []
  }
}

function startEdit() {
  if (!host.value) return
  editForm.value = {
    name: host.value.name || '',
    hostname: host.value.hostname || '',
    ip_address: host.value.ip_address || '',
    os: host.value.os || '',
  }
  rotateKeyResult.value = null
  isEditing.value = true
}

function cancelEdit() {
  isEditing.value = false
  rotateKeyResult.value = null
  editError.value = ''
}

async function rotateHostKey() {
  rotateKeyLoading.value = true
  rotateKeyResult.value = null
  try {
    const res = await apiClient.rotateHostKey(hostId)
    rotateKeyResult.value = res.data
  } catch (e) {
    console.error('Failed to rotate API key:', e.response?.data || e.message)
  } finally {
    rotateKeyLoading.value = false
  }
}

async function copyRotatedKey() {
  if (!rotateKeyResult.value?.api_key) return
  await navigator.clipboard.writeText(rotateKeyResult.value.api_key)
  rotateCopiedKey.value = true
  setTimeout(() => {
    rotateCopiedKey.value = false
  }, 1500)
}

async function copyRotatedConfig() {
  if (!rotatedAgentConfig.value) return
  await navigator.clipboard.writeText(rotatedAgentConfig.value)
  rotateCopiedConfig.value = true
  setTimeout(() => {
    rotateCopiedConfig.value = false
  }, 1500)
}

const editError = ref('')

async function saveEdit() {
  editError.value = ''
  saving.value = true
  try {
    const res = await apiClient.updateHost(hostId, editForm.value)
    host.value = res.data
    isEditing.value = false
  } catch (e) {
    editError.value = e.response?.data?.error || e.message
  } finally {
    saving.value = false
  }
}

async function sendAptCmd(command) {
  const confirmed = await dialog.confirm({
    title: `apt ${command}`,
    message: `Exécuter sur : ${host.value?.hostname}`,
    variant: command === 'dist-upgrade' ? 'danger' : 'warning'
  })

  if (!confirmed) return

  aptCmdLoading.value = command
  try {
    const response = await apiClient.sendAptCommand([hostId], command)

    // Auto-open console with command
    if (response.data?.commands?.length > 0) {
      const cmd = response.data.commands[0]
      if (cmd.command_id) {
        watchCommand({
          id: cmd.command_id,
          command: command,
          status: 'pending',
          output: ''
        })
      }
    }
  } catch (e) {
    await dialog.confirm({
      title: 'Erreur',
      message: e.response?.data?.error || e.message,
      variant: 'danger'
    })
  } finally {
    aptCmdLoading.value = ''
  }
}

function formatDate(date) {
  if (!date || date === '0001-01-01T00:00:00Z') return 'Jamais'
  return dayjs.utc(date).local().fromNow()
}

const combinedHistory = computed(() =>
  showFullHistory.value ? cmdHistory.value : cmdHistory.value.slice(0, 5)
)

const combinedHistoryTotal = computed(() => cmdHistory.value.length)

function loadColor(load, cores) {
  if (!load || !cores) return 'text-secondary'
  const ratio = load / cores
  if (ratio > 1) return 'text-red'
  if (ratio > 0.7) return 'text-yellow'
  return 'text-green'
}

const MODULE_META = {
  apt:       { label: 'APT',       cls: 'badge bg-azure-lt text-azure' },
  docker:    { label: 'Docker',    cls: 'badge bg-blue-lt text-blue' },
  systemd:   { label: 'Systemd',   cls: 'badge bg-green-lt text-green' },
  journal:   { label: 'Journal',   cls: 'badge bg-purple-lt text-purple' },
  processes: { label: 'Processus', cls: 'badge bg-orange-lt text-orange' },
  custom:    { label: 'Custom',    cls: 'badge bg-teal-lt text-teal' },
}

function cmdModuleLabel(module) { return MODULE_META[module]?.label ?? module }
function cmdModuleClass(module)  { return MODULE_META[module]?.cls ?? 'badge bg-secondary' }
function cmdLabel(cmd) {
  const parts = [cmd.action]
  if (cmd.target) parts.push(cmd.target)
  return parts.join(' ')
}

function statusClass(status) {
  if (status === 'completed') return 'badge bg-green-lt text-green'
  if (status === 'failed') return 'badge bg-red-lt text-red'
  return 'badge bg-yellow-lt text-yellow'
}

function renderConsoleOutput(raw) {
  if (!raw) return ''
  const lines = ['']
  let currentLine = ''

  for (let i = 0; i < raw.length; i++) {
    const ch = raw[i]
    if (ch === '\r') {
      currentLine = ''
      lines[lines.length - 1] = ''
      continue
    }
    if (ch === '\n') {
      currentLine = ''
      lines.push('')
      continue
    }
    currentLine += ch
    lines[lines.length - 1] = currentLine
  }

  return lines.join('\n')
}

function isAgentUpToDate(version) {
  if (!version || !latestAgentVersion.value) return false
  return version === latestAgentVersion.value
}

async function fetchLatestAgentVersion() {
  try {
    const res = await apiClient.getSettings()
    latestAgentVersion.value = res.data?.settings?.latestAgentVersion || ''
  } catch { /* non-critical */ }
}

async function loadJournalLogs() {
  const svc = journalService.value.trim()
  if (!svc) return
  journalLoading.value = true
  journalError.value = ''
  journalCmdId.value = null
  try {
    const res = await apiClient.sendJournalCommand(hostId, svc)
    const cmdId = res.data.command_id
    journalCmdId.value = cmdId
    liveCommand.value = {
      id: cmdId,
      prefix: '',
      command: `journalctl -u ${svc}`,
      status: 'running',
      output: '',
    }
    showConsole.value = true
    const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws'
    const wsUrl = `${protocol}://${window.location.host}/api/v1/ws/commands/stream/${cmdId}`
    if (streamWs) streamWs.close()
    streamWs = new WebSocket(wsUrl)
    streamWs.onopen = () => {
      streamWs.send(JSON.stringify({ type: 'auth', token: auth.token }))
    }
    streamWs.onmessage = (event) => {
      try {
        const payload = JSON.parse(event.data)
        if (payload.type === 'cmd_stream_init') {
          liveCommand.value.status = payload.status
          liveCommand.value.output = payload.output || ''
          nextTick(() => scrollToBottom())
        } else if (payload.type === 'cmd_stream') {
          liveCommand.value.output += payload.chunk
          nextTick(() => scrollToBottom())
        } else if (payload.type === 'cmd_status_update') {
          liveCommand.value.status = payload.status
          if (payload.status === 'completed' || payload.status === 'failed') {
            journalLoading.value = false
            loadCmdHistory()
          }
        }
      } catch (e) { /* ignore */ }
    }
    streamWs.onclose = () => { journalLoading.value = false }
  } catch (e) {
    journalError.value = e.response?.data?.error || 'Impossible d\'envoyer la commande'
    journalLoading.value = false
  }
}

function watchCommand(cmd) {
  let displayCommand
  let prefix = ''
  const module = cmd.module || 'apt'
  if (module === 'apt') {
    prefix = 'apt '
    displayCommand = cmd.action || cmd.command
  } else if (module === 'journal') {
    displayCommand = `journalctl -u ${cmd.target || cmd.container_name}`
  } else {
    displayCommand = `${cmd.action} ${cmd.target || cmd.container_name}`
  }
  liveCommand.value = {
    id: cmd.id,
    prefix,
    command: displayCommand,
    status: cmd.status,
    output: cmd.output || '',
  }
  showConsole.value = true
  connectStreamWebSocket(cmd.id)
  nextTick(() => scrollToBottom())
}

function closeLiveConsole() {
  if (streamWs) {
    streamWs.onclose = null
    streamWs.onmessage = null
    streamWs.close()
    streamWs = null
  }
  liveCommand.value = null
  journalLoading.value = false
}

function openConsoleFromPanel({ commandId, prefix, command }) {
  liveCommand.value = {
    id: commandId,
    prefix: prefix || '',
    command,
    status: 'running',
    output: '',
  }
  showConsole.value = true
  connectStreamWebSocket(commandId)
  nextTick(() => scrollToBottom())
}

function scrollToBottom() {
  if (consoleOutput.value) {
    consoleOutput.value.scrollTop = consoleOutput.value.scrollHeight
  }
}

function connectStreamWebSocket(commandId) {
  if (streamWs) {
    streamWs.close()
  }
  const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws'
  const wsUrl = `${protocol}://${window.location.host}/api/v1/ws/commands/stream/${commandId}`
  streamWs = new WebSocket(wsUrl)

  streamWs.onopen = () => {
    streamWs.send(JSON.stringify({ type: 'auth', token: auth.token }))
  }

  streamWs.onmessage = (event) => {
    try {
      const payload = JSON.parse(event.data)
      if (payload.type === 'cmd_stream_init') {
        liveCommand.value.status = payload.status
        liveCommand.value.output = payload.output || ''
        nextTick(() => scrollToBottom())
      } else if (payload.type === 'cmd_stream') {
        liveCommand.value.output += payload.chunk
        nextTick(() => scrollToBottom())
      } else if (payload.type === 'cmd_status_update') {
        liveCommand.value.status = payload.status
        // Sync status in history table immediately (no reload needed for intermediate states)
        const histCmd = cmdHistory.value.find(c => c.id === liveCommand.value.id)
        if (histCmd) {
          histCmd.status = payload.status
          if (payload.status === 'completed' || payload.status === 'failed') {
            loadCmdHistory()
          }
        } else {
          // Command not yet in history (auto-triggered before loadCmdHistory was called) — fetch it now
          loadCmdHistory()
        }
      }
    } catch (e) {
      // Ignore malformed payloads
    }
  }

  streamWs.onclose = () => {
    // Connection closed, don't reconnect automatically
  }
}

// ── Tâches planifiées ──────────────────────────────────────────────────────
const MANUAL_SENTINEL = '0 0 29 2 *'
const taskModuleActions = {
  apt:       ['update', 'upgrade', 'dist-upgrade'],
  docker:    ['start', 'stop', 'restart', 'logs', 'pull'],
  systemd:   ['start', 'stop', 'restart', 'status', 'enable', 'disable'],
  journal:   ['read'],
  processes: ['list'],
  custom:    null,
}

const tasks = ref([])
const tasksLoading = ref(false)
const tasksError = ref('')
const taskRunningId = ref(null)
const taskRunResult = ref(null)
const showTaskModal = ref(false)
const editingTask = ref(null)
const taskSaving = ref(false)
const taskModalError = ref('')
const taskManualOnly = ref(false)
const customTaskOptions = ref([])
const taskForm = ref({ name: '', module: 'apt', action: 'update', target: '', cron_expression: '0 3 * * 0', enabled: true })

watch(taskManualOnly, (val) => {
  if (val) {
    taskForm.value.enabled = false
    taskForm.value.cron_expression = MANUAL_SENTINEL
  } else {
    taskForm.value.enabled = true
    if (taskForm.value.cron_expression === MANUAL_SENTINEL) {
      taskForm.value.cron_expression = '0 3 * * 0'
    }
  }
})

function isManualOnly(task) {
  return task.cron_expression === MANUAL_SENTINEL && !task.enabled
}

function describeCron(expr) {
  if (!expr) return ''
  const presets = {
    '@daily': 'tous les jours à minuit',
    '@hourly': 'toutes les heures',
    '@weekly': 'hebdomadaire (dim. minuit)',
    '@monthly': 'mensuel (1er à minuit)',
  }
  if (presets[expr]) return presets[expr]
  const parts = expr.split(' ')
  if (parts.length !== 5) return ''
  const [min, hour, dom, , dow] = parts
  const dayNames = ['dim', 'lun', 'mar', 'mer', 'jeu', 'ven', 'sam']
  if (dom === '*' && dow === '*' && hour !== '*' && min !== '*')
    return `tous les jours à ${hour.padStart(2, '0')}h${min.padStart(2, '0')}`
  if (dom !== '*' && dow === '*' && hour !== '*' && min !== '*')
    return `le ${dom} du mois à ${hour.padStart(2, '0')}h${min.padStart(2, '0')}`
  if (dom === '*' && dow !== '*') {
    const days = dow.split(',').map(d => { const n = parseInt(d); return !isNaN(n) && n <= 6 ? dayNames[n] : d })
    if (hour !== '*' && min !== '*')
      return `chaque ${days.join(', ')} à ${hour.padStart(2, '0')}h${min.padStart(2, '0')}`
    return `chaque ${days.join(', ')}`
  }
  return ''
}

function formatTaskDate(iso) {
  if (!iso) return ''
  return new Date(iso).toLocaleString('fr-FR', { dateStyle: 'short', timeStyle: 'short' })
}

async function loadTasks() {
  tasksLoading.value = true
  tasksError.value = ''
  try {
    const { data } = await apiClient.getScheduledTasks(hostId)
    tasks.value = data
  } catch (e) {
    tasksError.value = e.response?.data?.error || 'Erreur de chargement'
  } finally {
    tasksLoading.value = false
  }
}

function switchToTasks() {
  activeTab.value = 'planifiees'
  if (!tasks.value.length && !tasksLoading.value) loadTasks()
}

async function loadCustomTasks() {
  try {
    const { data } = await apiClient.getHostCustomTasks(hostId)
    customTaskOptions.value = Array.isArray(data) ? data : []
    if (customTaskOptions.value.length && !taskForm.value.target)
      taskForm.value.target = customTaskOptions.value[0].id
  } catch {
    customTaskOptions.value = []
  }
}

async function onTaskModuleChange() {
  const actions = taskModuleActions[taskForm.value.module]
  taskForm.value.action = actions ? actions[0] : 'run'
  if (taskForm.value.module === 'custom') await loadCustomTasks()
}

function openCreateTask() {
  editingTask.value = null
  taskManualOnly.value = false
  customTaskOptions.value = []
  taskForm.value = { name: '', module: 'apt', action: 'update', target: '', cron_expression: '0 3 * * 0', enabled: true }
  taskModalError.value = ''
  showTaskModal.value = true
}

async function openEditTask(task) {
  editingTask.value = task
  taskManualOnly.value = isManualOnly(task)
  customTaskOptions.value = []
  taskForm.value = { name: task.name, module: task.module, action: task.action, target: task.target, cron_expression: task.cron_expression, enabled: task.enabled }
  taskModalError.value = ''
  showTaskModal.value = true
  if (task.module === 'custom') await loadCustomTasks()
}

function closeTaskModal() {
  showTaskModal.value = false
}

async function saveTask() {
  if (!taskForm.value.name || !taskForm.value.action) {
    taskModalError.value = 'Nom et action sont obligatoires.'
    return
  }
  if (!taskManualOnly.value && !taskForm.value.cron_expression) {
    taskModalError.value = 'Expression cron obligatoire.'
    return
  }
  taskSaving.value = true
  taskModalError.value = ''
  try {
    if (editingTask.value) {
      await apiClient.updateScheduledTask(editingTask.value.id, taskForm.value)
    } else {
      await apiClient.createScheduledTask(hostId, taskForm.value)
    }
    closeTaskModal()
    await loadTasks()
  } catch (e) {
    taskModalError.value = e.response?.data?.error || e.response?.data?.warning || 'Erreur lors de la sauvegarde'
  } finally {
    taskSaving.value = false
  }
}

async function toggleTask(task) {
  try {
    await apiClient.updateScheduledTask(task.id, { ...task, enabled: !task.enabled })
    await loadTasks()
  } catch (e) {
    tasksError.value = e.response?.data?.error || 'Erreur'
  }
}

async function runTaskNow(task) {
  taskRunningId.value = task.id
  try {
    const { data } = await apiClient.runScheduledTask(task.id)
    taskRunResult.value = { id: data.command_id, name: task.name }
    setTimeout(() => { taskRunResult.value = null }, 5000)
    // Ouvrir automatiquement les logs dans la console live
    watchCommand({
      id: data.command_id,
      module: task.module,
      action: task.action,
      target: task.target,
      status: 'pending',
      output: '',
    })
    await loadTasks()
  } catch (e) {
    tasksError.value = e.response?.data?.error || 'Erreur'
  } finally {
    taskRunningId.value = null
  }
}

function openTaskLogs(task) {
  if (!task.last_command_id) return
  watchCommand({
    id: task.last_command_id,
    module: task.module,
    action: task.action,
    target: task.target,
    status: task.last_run_status || 'completed',
    output: '',
  })
}

async function confirmDeleteTask(task) {
  const confirmed = await dialog.confirm({
    title: 'Supprimer la tâche',
    message: `Supprimer la tâche "${task.name}" ?\nCette action est irréversible.`,
    variant: 'danger',
  })
  if (!confirmed) return
  try {
    await apiClient.deleteScheduledTask(task.id)
    await loadTasks()
  } catch (e) {
    tasksError.value = e.response?.data?.error || 'Erreur de suppression'
  }
}
// ── /Tâches planifiées ─────────────────────────────────────────────────────

async function deleteHost() {
  const confirmed = await dialog.confirm({
    title: 'Supprimer l\'hôte',
    message: `Cette action est irréversible. Toutes les données associées seront supprimées.`,
    variant: 'danger',
    requiredText: host.value?.hostname || host.value?.name
  })
  
  if (!confirmed) return
  
  try {
    await apiClient.deleteHost(hostId)
    router.push('/')
  } catch (e) {
    await dialog.confirm({
      title: 'Erreur',
      message: e.response?.data?.error || e.message,
      variant: 'danger'
    })
  }
}

onMounted(() => {
  fetchLatestAgentVersion()
  loadCmdHistory()
})

onUnmounted(() => {
  if (streamWs) streamWs.close()
})
</script>

<style scoped>
.console-output {
  background: #0f172a;
  color: #f1f5f9;
  padding: 1rem;
  margin: 0;
  overflow-y: auto;
  font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
  font-size: 0.813rem;
  line-height: 1.5;
  border-radius: 0 0 0.5rem 0.5rem;
  white-space: pre-wrap;
  word-break: break-all;
}

.host-detail-page {
  display: flex;
  flex-direction: column;
  height: calc(100vh - 100px);
}

.host-layout {
  display: flex;
  flex: 1;
  gap: 1rem;
  overflow: hidden;
  min-height: 0;
}

.host-panel-main {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
  min-width: 0;
}

.host-panel-right {
  width: 38%;
  min-width: 380px;
  display: flex;
  flex-direction: column;
  transition: all 0.3s ease-in-out;
  overflow: hidden;
}

@media (max-width: 991px) {
  .host-detail-page {
    height: auto;
  }

  .host-layout {
    flex-direction: column;
    overflow: visible;
    height: auto;
  }

  .host-panel-main {
    overflow-y: visible;
  }

  .host-panel-right {
    width: 100%;
    min-width: 0;
    max-height: 70vh;
  }
}
</style>