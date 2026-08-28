<template>
  <div>
    <div class="row row-cards mb-3">
      <div class="col-6 col-md-4">
        <div class="card card-sm h-100">
          <div class="card-body">
            <div class="subheader">
              Sondes en panne
            </div>
            <div
              class="h2 mb-0 mt-1"
              :class="downCount > 0 ? 'text-danger' : 'text-success'"
            >
              {{ downCount }}
            </div>
          </div>
        </div>
      </div>
      <div class="col-6 col-md-4">
        <div class="card card-sm h-100">
          <div class="card-body">
            <div class="subheader">
              Certificats expirant &lt;30j
            </div>
            <div
              class="h2 mb-0 mt-1"
              :class="expiringCount > 0 ? 'text-warning' : 'text-success'"
            >
              {{ expiringCount }}
            </div>
          </div>
        </div>
      </div>
      <div class="col-6 col-md-4">
        <div class="card card-sm h-100">
          <div class="card-body">
            <div class="subheader">
              Total surveillé
            </div>
            <div class="h2 mb-0 mt-1">
              {{ totalMonitored }}
            </div>
          </div>
        </div>
      </div>
    </div>

    <PageRefreshBar
      v-model="autoRefresh"
      label="Monitoring"
      :interval-sec="REFRESH_SEC"
      :last-updated-at="lastUpdatedAt"
    />

    <div
      v-if="error"
      class="alert alert-danger mb-3"
    >
      {{ error }}
    </div>

    <DataToolbar
      searchable
      :search="search"
      search-placeholder="Rechercher un hôte…"
      @update:search="search = $event"
    />

    <div
      v-if="loading && !pagedRows.length"
      class="row row-cards"
    >
      <div class="col-12">
        <LoadingSkeleton
          variant="table"
          :lines="5"
        />
      </div>
    </div>

    <EmptyState
      v-else-if="!pagedRows.length"
      :title="search ? 'Aucun résultat pour cette recherche' : 'Aucune sonde ni certificat configuré'"
      :subtitle="search ? 'Modifiez votre recherche.' : 'Créez une sonde uptime ou un certificat SSL pour commencer à surveiller un service.'"
      :cta-label="!search && auth.role === 'admin' ? 'Nouveau suivi' : ''"
      @cta="openCreateProbe"
    />

    <div
      v-else
      class="card"
    >
      <div class="table-responsive scroll-table">
        <table class="table table-vcenter card-table">
          <thead>
            <tr>
              <th>
                <SortableHeader
                  label="Nom / Cible"
                  :active="rowSort.col === 'name'"
                  :direction="rowSort.dir"
                  @toggle="toggleRowSort('name')"
                />
              </th>
              <th>
                <SortableHeader
                  label="Statut"
                  :active="rowSort.col === 'status'"
                  :direction="rowSort.dir"
                  @toggle="toggleRowSort('status')"
                />
              </th>
              <th>Disponibilité</th>
              <th>
                <SortableHeader
                  label="Uptime 24h"
                  :active="rowSort.col === 'uptime'"
                  :direction="rowSort.dir"
                  @toggle="toggleRowSort('uptime')"
                />
              </th>
              <th>
                <SortableHeader
                  label="SSL"
                  :active="rowSort.col === 'ssl_days'"
                  :direction="rowSort.dir"
                  @toggle="toggleRowSort('ssl_days')"
                />
              </th>
              <th>
                <SortableHeader
                  label="Dernière vérification"
                  :active="rowSort.col === 'last_checked'"
                  :direction="rowSort.dir"
                  @toggle="toggleRowSort('last_checked')"
                />
              </th>
              <th />
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="row in pagedRows"
              :key="row.id"
              :class="{ 'opacity-60': row.probe && !row.probe.enabled }"
            >
              <td>
                <router-link
                  :to="rowLink(row)"
                  class="fw-semibold text-decoration-none"
                >
                  {{ row.name }}
                </router-link>
                <div
                  v-if="row.probe"
                  class="text-secondary small"
                >
                  <code>{{ row.probe.target }}</code>
                </div>
                <div
                  v-else-if="row.cert"
                  class="text-secondary small"
                >
                  <code>{{ row.cert.host }}:{{ row.cert.port }}</code>
                </div>
                <span
                  v-if="row.npmProxyHostId"
                  class="badge bg-azure-lt text-azure mt-1"
                >NPM</span>
              </td>
              <td>
                <template v-if="row.probe">
                  <span :class="['badge', probeBadge(row.probe)]">
                    {{ probeStatusLabel(row.probe) }}
                  </span>
                  <span
                    v-if="!row.probe.enabled"
                    class="badge bg-secondary-lt text-secondary ms-1"
                  >désactivée</span>
                </template>
                <span
                  v-else
                  class="text-secondary small"
                >—</span>
              </td>
              <td>
                <div
                  v-if="row.probe && probeHistory[row.probe.id]?.length"
                  class="tracking"
                  style="height: 20px; min-width: 110px;"
                >
                  <div
                    v-for="tick in probeHistory[row.probe.id]"
                    :key="tick.id"
                    class="flex-fill rounded-1"
                    :class="tick.success ? 'bg-success' : 'bg-danger'"
                    style="height: 100%; min-width: 2px;"
                    :title="`${formatDateTime(tick.checked_at)} — ${tick.success ? 'OK' : 'KO'}`"
                  />
                </div>
                <span
                  v-else
                  class="text-secondary small"
                >—</span>
              </td>
              <td>
                <template v-if="row.probe && probeStats[row.probe.id]">
                  <span :class="['badge', uptimeBadgeClass(probeStats[row.probe.id].uptime_percent)]">
                    {{ probeStats[row.probe.id].uptime_percent.toFixed(1) }}%
                  </span>
                </template>
                <span
                  v-else
                  class="text-secondary small"
                >—</span>
              </td>
              <td>
                <span
                  v-if="row.cert"
                  :class="['badge', daysBadge(row.cert.days_remaining)]"
                >
                  {{ daysLabel(row.cert.days_remaining) }}
                </span>
                <span
                  v-else
                  class="text-secondary small"
                >—</span>
              </td>
              <td class="text-secondary small">
                <RelativeTime
                  v-if="row.probe?.last_checked_at || row.cert?.last_checked_at"
                  :date="(row.probe?.last_checked_at || row.cert?.last_checked_at) as string"
                />
                <span
                  v-else
                  class="text-secondary"
                >Jamais</span>
              </td>
              <td class="text-end">
                <div class="btn-list flex-nowrap justify-content-end">
                  <button
                    v-if="auth.role === 'admin'"
                    type="button"
                    class="btn btn-sm btn-outline-secondary"
                    :disabled="checkingProbeId === row.probe?.id || checkingCertId === row.cert?.id"
                    @click="checkRowNow(row)"
                  >
                    Vérifier
                  </button>
                  <div
                    v-if="auth.role === 'admin' && row.probe"
                    class="btn-group btn-group-sm"
                  >
                    <button
                      type="button"
                      class="btn btn-icon btn-sm btn-ghost-secondary"
                      title="Modifier la sonde"
                      aria-label="Modifier la sonde"
                      @click="openEditProbe(row.probe)"
                    >
                      <IconActivity :size="14" />
                    </button>
                    <button
                      type="button"
                      class="btn btn-icon btn-sm btn-ghost-danger"
                      title="Supprimer la sonde"
                      aria-label="Supprimer la sonde"
                      @click="confirmDeleteProbe(row.probe)"
                    >
                      <IconTrash :size="14" />
                    </button>
                  </div>
                  <div
                    v-if="auth.role === 'admin' && row.cert"
                    class="btn-group btn-group-sm"
                  >
                    <button
                      type="button"
                      class="btn btn-icon btn-sm btn-ghost-secondary"
                      title="Modifier le certificat"
                      aria-label="Modifier le certificat"
                      @click="openEditCert(row.cert)"
                    >
                      <IconLock :size="14" />
                    </button>
                    <button
                      type="button"
                      class="btn btn-icon btn-sm btn-ghost-danger"
                      title="Supprimer le certificat"
                      aria-label="Supprimer le certificat"
                      @click="confirmDeleteCert(row.cert)"
                    >
                      <IconTrash :size="14" />
                    </button>
                  </div>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div
        v-if="rowTotalPages > 1"
        class="card-footer d-flex align-items-center justify-content-between"
      >
        <div class="text-secondary small">
          {{ (rowPage - 1) * PAGE_SIZE + 1 }}–{{ Math.min(rowPage * PAGE_SIZE, filteredCount) }} sur {{ filteredCount }}
        </div>
        <PaginationNav
          :current-page="rowPage"
          :total-pages="rowTotalPages"
          @select="setRowPage"
        />
      </div>
    </div>

    <!-- ===== MODAL SUIVI (sonde uptime / certificat SSL) =====
         One shared modal for both "add a monitoring target" flows — a probe
         and a cert are different resources with different fields, so their
         bodies stay distinct, but they share one entry point, one chrome
         (header/footer/backdrop) and a type switcher instead of two
         independent modals a user could otherwise open side by side. -->
    <div
      v-if="createModalOpen"
      ref="createModalRef"
      class="modal modal-blur fade show d-block"
      tabindex="-1"
      role="dialog"
      aria-modal="true"
    >
      <div class="modal-dialog modal-lg modal-dialog-centered">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">
              {{ isEditingCreateModal ? (createType === 'probe' ? 'Modifier la sonde' : 'Modifier le certificat') : 'Nouveau suivi' }}
            </h5>
            <button
              type="button"
              class="btn-close"
              :disabled="savingProbe || savingCert"
              @click="closeCreateModal"
            />
          </div>
          <!-- Editing an existing probe or cert is unambiguous (the row
               already told us which), so it keeps the original two
               fully-independent forms. Creating is where a shared "Hôte /
               Cible" actually helps — a probe and a cert for the same
               target would otherwise mean typing the same domain twice. -->
          <template v-if="isEditingCreateModal">
            <form
              v-if="createType === 'probe'"
              @submit.prevent="saveProbe"
            >
              <div class="modal-body pt-0">
                <div
                  v-if="probeFormError"
                  class="alert alert-danger"
                >
                  {{ probeFormError }}
                </div>
                <div class="row g-3">
                  <div class="col-md-7">
                    <label
                      for="monitoring-edit-probe-name"
                      class="form-label required"
                    >Nom</label>
                    <input
                      id="monitoring-edit-probe-name"
                      v-model="probeForm.name"
                      type="text"
                      class="form-control"
                      placeholder="Ex: API prod"
                      required
                    >
                  </div>
                  <div class="col-md-5">
                    <label
                      for="monitoring-edit-probe-type"
                      class="form-label required"
                    >Type</label>
                    <select
                      id="monitoring-edit-probe-type"
                      v-model="probeForm.type"
                      class="form-select"
                    >
                      <option value="http">
                        HTTP/HTTPS
                      </option>
                      <option value="tcp">
                        TCP
                      </option>
                      <option value="icmp">
                        ICMP (ping)
                      </option>
                    </select>
                  </div>
                  <div class="col-12">
                    <label
                      for="monitoring-edit-probe-target"
                      class="form-label required"
                    >{{ probeTargetLabel }}</label>
                    <input
                      id="monitoring-edit-probe-target"
                      v-model="probeForm.target"
                      type="text"
                      class="form-control"
                      :placeholder="probeTargetPlaceholder"
                      required
                    >
                    <div
                      v-if="probeForm.type === 'icmp'"
                      class="form-hint"
                    >
                      Nécessite CAP_NET_RAW côté conteneur serveur (activé par défaut — voir server/Dockerfile).
                      Sans elle, le check échoue explicitement plutôt que de rapporter un faux "hors ligne".
                    </div>
                  </div>
                  <div class="col-md-4">
                    <label
                      for="monitoring-edit-probe-interval"
                      class="form-label"
                    >Intervalle (sec)</label>
                    <input
                      id="monitoring-edit-probe-interval"
                      v-model.number="probeForm.interval_sec"
                      type="number"
                      min="10"
                      class="form-control"
                    >
                  </div>
                  <div class="col-md-4">
                    <label
                      for="monitoring-edit-probe-timeout"
                      class="form-label"
                    >Timeout (sec)</label>
                    <input
                      id="monitoring-edit-probe-timeout"
                      v-model.number="probeForm.timeout_sec"
                      type="number"
                      min="1"
                      max="60"
                      class="form-control"
                    >
                  </div>
                  <template v-if="probeForm.type === 'http'">
                    <div class="col-md-4">
                      <label
                        for="monitoring-edit-probe-expected-status"
                        class="form-label"
                      >Statut HTTP attendu</label>
                      <input
                        id="monitoring-edit-probe-expected-status"
                        v-model.number="probeForm.expected_status"
                        type="number"
                        min="100"
                        max="599"
                        class="form-control"
                      >
                    </div>
                    <div class="col-12">
                      <label
                        for="monitoring-edit-probe-expected-body-regex"
                        class="form-label"
                      >Regex corps attendu (optionnel)</label>
                      <input
                        id="monitoring-edit-probe-expected-body-regex"
                        v-model="probeForm.expected_body_regex"
                        type="text"
                        class="form-control"
                        placeholder="Ex: &quot;status&quot;:\s*&quot;ok&quot;"
                      >
                    </div>
                    <div class="col-md-6">
                      <label class="form-check">
                        <input
                          v-model="probeForm.follow_redirects"
                          type="checkbox"
                          class="form-check-input"
                        >
                        <span class="form-check-label">Suivre les redirections</span>
                      </label>
                    </div>
                    <div class="col-md-6">
                      <label class="form-check">
                        <input
                          v-model="probeForm.verify_tls"
                          type="checkbox"
                          class="form-check-input"
                        >
                        <span class="form-check-label">Vérifier le certificat TLS</span>
                      </label>
                    </div>
                  </template>
                  <div class="col-12">
                    <label class="form-check">
                      <input
                        v-model="probeForm.enabled"
                        type="checkbox"
                        class="form-check-input"
                      >
                      <span class="form-check-label">Activée</span>
                    </label>
                  </div>
                </div>
              </div>
              <div class="modal-footer">
                <button
                  type="button"
                  class="btn link-secondary"
                  :disabled="savingProbe"
                  @click="closeCreateModal"
                >
                  Annuler
                </button>
                <button
                  type="submit"
                  class="btn btn-primary"
                  :disabled="savingProbe"
                >
                  {{ savingProbe ? 'Enregistrement...' : 'Enregistrer' }}
                </button>
              </div>
            </form>
            <form
              v-else
              @submit.prevent="saveCert"
            >
              <div class="modal-body pt-0">
                <div
                  v-if="certFormError"
                  class="alert alert-danger"
                >
                  {{ certFormError }}
                </div>
                <div class="mb-3">
                  <label
                    for="monitoring-edit-cert-name"
                    class="form-label required"
                  >Nom</label>
                  <input
                    id="monitoring-edit-cert-name"
                    v-model="certForm.name"
                    type="text"
                    class="form-control"
                    placeholder="Ex: api.example.com"
                    required
                  >
                </div>
                <div class="row g-3">
                  <div class="col-md-8">
                    <label
                      for="monitoring-edit-cert-host"
                      class="form-label required"
                    >Hôte</label>
                    <input
                      id="monitoring-edit-cert-host"
                      v-model="certForm.host"
                      type="text"
                      class="form-control"
                      placeholder="api.example.com"
                      required
                    >
                  </div>
                  <div class="col-md-4">
                    <label
                      for="monitoring-edit-cert-port"
                      class="form-label required"
                    >Port</label>
                    <input
                      id="monitoring-edit-cert-port"
                      v-model.number="certForm.port"
                      type="number"
                      min="1"
                      max="65535"
                      class="form-control"
                    >
                  </div>
                  <div class="col-12">
                    <label
                      for="monitoring-edit-cert-sni"
                      class="form-label"
                    >SNI (override, optionnel)</label>
                    <input
                      id="monitoring-edit-cert-sni"
                      v-model="certForm.server_name"
                      type="text"
                      class="form-control"
                      placeholder="Laisser vide pour utiliser l'hôte"
                    >
                  </div>
                  <div class="col-12">
                    <label class="form-check">
                      <input
                        v-model="certForm.enabled"
                        type="checkbox"
                        class="form-check-input"
                      >
                      <span class="form-check-label">Activé</span>
                    </label>
                  </div>
                </div>
              </div>
              <div class="modal-footer">
                <button
                  type="button"
                  class="btn link-secondary"
                  :disabled="savingCert"
                  @click="closeCreateModal"
                >
                  Annuler
                </button>
                <button
                  type="submit"
                  class="btn btn-primary"
                  :disabled="savingCert"
                >
                  {{ savingCert ? 'Enregistrement...' : 'Enregistrer' }}
                </button>
              </div>
            </form>
          </template>

          <template v-else>
            <div class="modal-body pb-0">
              <div class="btn-group w-100 mb-3">
                <button
                  type="button"
                  class="btn"
                  :class="createIncludeProbe ? 'btn-primary' : 'btn-outline-secondary'"
                  @click="toggleIncludeProbe"
                >
                  Sonde uptime
                </button>
                <button
                  type="button"
                  class="btn"
                  :class="createIncludeCert ? 'btn-primary' : 'btn-outline-secondary'"
                  @click="toggleIncludeCert"
                >
                  Certificat SSL
                </button>
              </div>
            </div>
            <form @submit.prevent="submitCreateModal">
              <div class="modal-body pt-0">
                <div
                  v-if="probeFormError || certFormError"
                  class="alert alert-danger"
                >
                  {{ probeFormError || certFormError }}
                </div>
                <div class="mb-3">
                  <label
                    for="monitoring-create-name"
                    class="form-label required"
                  >Nom</label>
                  <input
                    id="monitoring-create-name"
                    v-model="sharedName"
                    type="text"
                    class="form-control"
                    placeholder="Ex: API prod"
                    required
                  >
                </div>

                <template v-if="createIncludeProbe">
                  <div class="row g-3">
                    <div class="col-md-5">
                      <label
                        for="monitoring-create-probe-type"
                        class="form-label required"
                      >Type</label>
                      <select
                        id="monitoring-create-probe-type"
                        v-model="probeForm.type"
                        class="form-select"
                      >
                        <option value="http">
                          HTTP/HTTPS
                        </option>
                        <option value="tcp">
                          TCP
                        </option>
                        <option value="icmp">
                          ICMP (ping)
                        </option>
                      </select>
                    </div>
                    <div class="col-md-7">
                      <label
                        for="monitoring-create-probe-target"
                        class="form-label required"
                      >{{ probeTargetLabel }}</label>
                      <input
                        id="monitoring-create-probe-target"
                        v-model="probeForm.target"
                        type="text"
                        class="form-control"
                        :placeholder="probeTargetPlaceholder"
                        required
                      >
                      <div
                        v-if="probeForm.type === 'icmp'"
                        class="form-hint"
                      >
                        Nécessite CAP_NET_RAW côté conteneur serveur (activé par défaut — voir server/Dockerfile).
                        Sans elle, le check échoue explicitement plutôt que de rapporter un faux "hors ligne".
                      </div>
                    </div>
                    <div class="col-md-4">
                      <label
                        for="monitoring-create-probe-interval"
                        class="form-label"
                      >Intervalle (sec)</label>
                      <input
                        id="monitoring-create-probe-interval"
                        v-model.number="probeForm.interval_sec"
                        type="number"
                        min="10"
                        class="form-control"
                      >
                    </div>
                    <div class="col-md-4">
                      <label
                        for="monitoring-create-probe-timeout"
                        class="form-label"
                      >Timeout (sec)</label>
                      <input
                        id="monitoring-create-probe-timeout"
                        v-model.number="probeForm.timeout_sec"
                        type="number"
                        min="1"
                        max="60"
                        class="form-control"
                      >
                    </div>
                    <template v-if="probeForm.type === 'http'">
                      <div class="col-md-4">
                        <label
                          for="monitoring-create-probe-expected-status"
                          class="form-label"
                        >Statut HTTP attendu</label>
                        <input
                          id="monitoring-create-probe-expected-status"
                          v-model.number="probeForm.expected_status"
                          type="number"
                          min="100"
                          max="599"
                          class="form-control"
                        >
                      </div>
                      <div class="col-12">
                        <label
                          for="monitoring-create-probe-expected-body-regex"
                          class="form-label"
                        >Regex corps attendu (optionnel)</label>
                        <input
                          id="monitoring-create-probe-expected-body-regex"
                          v-model="probeForm.expected_body_regex"
                          type="text"
                          class="form-control"
                          placeholder="Ex: &quot;status&quot;:\s*&quot;ok&quot;"
                        >
                      </div>
                      <div class="col-md-6">
                        <label class="form-check">
                          <input
                            v-model="probeForm.follow_redirects"
                            type="checkbox"
                            class="form-check-input"
                          >
                          <span class="form-check-label">Suivre les redirections</span>
                        </label>
                      </div>
                      <div class="col-md-6">
                        <label class="form-check">
                          <input
                            v-model="probeForm.verify_tls"
                            type="checkbox"
                            class="form-check-input"
                          >
                          <span class="form-check-label">Vérifier le certificat TLS</span>
                        </label>
                      </div>
                    </template>
                    <div class="col-12">
                      <label class="form-check">
                        <input
                          v-model="probeForm.enabled"
                          type="checkbox"
                          class="form-check-input"
                        >
                        <span class="form-check-label">Sonde activée</span>
                      </label>
                    </div>
                  </div>
                </template>

                <template v-if="createIncludeCert">
                  <hr
                    v-if="createIncludeProbe"
                    class="my-3"
                  >
                  <div class="row g-3">
                    <template v-if="!createIncludeProbe">
                      <div class="col-md-8">
                        <label
                          for="monitoring-create-cert-host"
                          class="form-label required"
                        >Hôte</label>
                        <input
                          id="monitoring-create-cert-host"
                          v-model="certForm.host"
                          type="text"
                          class="form-control"
                          placeholder="api.example.com"
                          required
                        >
                      </div>
                      <div class="col-md-4">
                        <label
                          for="monitoring-create-cert-port"
                          class="form-label required"
                        >Port</label>
                        <input
                          id="monitoring-create-cert-port"
                          v-model.number="certForm.port"
                          type="number"
                          min="1"
                          max="65535"
                          class="form-control"
                        >
                      </div>
                    </template>
                    <div
                      v-else
                      class="col-12 text-secondary small"
                    >
                      Certificat SSL vérifié sur <code>{{ certForm.host || '—' }}:{{ certForm.port }}</code> (dérivé de la cible de la sonde ci-dessus).
                    </div>
                    <div class="col-12">
                      <label
                        for="monitoring-create-cert-sni"
                        class="form-label"
                      >SNI (override, optionnel)</label>
                      <input
                        id="monitoring-create-cert-sni"
                        v-model="certForm.server_name"
                        type="text"
                        class="form-control"
                        placeholder="Laisser vide pour utiliser l'hôte"
                      >
                    </div>
                    <div class="col-12">
                      <label class="form-check">
                        <input
                          v-model="certForm.enabled"
                          type="checkbox"
                          class="form-check-input"
                        >
                        <span class="form-check-label">Certificat activé</span>
                      </label>
                    </div>
                  </div>
                </template>
              </div>
              <div class="modal-footer">
                <button
                  type="button"
                  class="btn link-secondary"
                  :disabled="savingProbe || savingCert"
                  @click="closeCreateModal"
                >
                  Annuler
                </button>
                <button
                  type="submit"
                  class="btn btn-primary"
                  :disabled="savingProbe || savingCert"
                >
                  {{ (savingProbe || savingCert) ? 'Enregistrement...' : 'Enregistrer' }}
                </button>
              </div>
            </form>
          </template>
        </div>
      </div>
    </div>
    <div
      v-if="createModalOpen"
      class="modal-backdrop fade show"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { IconActivity, IconLock, IconTrash } from '@tabler/icons-vue'
import { useAuthStore } from '../../stores/auth'
import EmptyState from '../EmptyState.vue'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import RelativeTime from '../RelativeTime.vue'
import PageRefreshBar from '../PageRefreshBar.vue'
import PaginationNav from '../PaginationNav.vue'
import DataToolbar from '../common/DataToolbar.vue'
import SortableHeader from '../common/SortableHeader.vue'
import { formatDateTime } from '../../utils/formatters'
import { useMonitoringOverview, type MonitoringRow } from '../../composables/useMonitoringOverview'
import { useModalChrome } from '../../composables/useModalChrome'

const auth = useAuthStore()

const {
  PAGE_SIZE,
  loading,
  error,
  downCount,
  expiringCount,
  totalMonitored,
  filteredCount,
  search,
  rowSort,
  toggleRowSort,
  pagedRows,
  rowPage,
  rowTotalPages,
  setRowPage,
  checkingProbeId,
  checkingCertId,
  checkRowNow,
  probeStats,
  probeHistory,
  probeBadge,
  probeStatusLabel,
  uptimeBadgeClass,
  daysLabel,
  daysBadge,
  autoRefresh,
  lastUpdatedAt,
  REFRESH_SEC,
  probeModalOpen,
  savingProbe,
  probeFormError,
  probeForm,
  openCreateProbe,
  openEditProbe,
  closeProbeModal,
  saveProbe,
  confirmDeleteProbe,
  certModalOpen,
  savingCert,
  certFormError,
  certForm,
  openEditCert,
  closeCertModal,
  saveCert,
  confirmDeleteCert,
} = useMonitoringOverview()

const probeTargetLabel = computed(() => {
  if (probeForm.value.type === 'http') return 'URL'
  if (probeForm.value.type === 'icmp') return 'Hôte ou IP'
  return 'host:port'
})
const probeTargetPlaceholder = computed(() => {
  if (probeForm.value.type === 'http') return 'https://example.com/health'
  if (probeForm.value.type === 'icmp') return '192.168.1.1 ou switch.local'
  return 'example.com:443'
})

// Probe creation/edit and cert creation/edit render through one shared
// modal (see the template) instead of two independent ones — createType
// tracks which body is showing, driven by whichever of probeModalOpen/
// certModalOpen the underlying composables set (openCreateProbe/openEditProbe
// vs openCreateCert/openEditCert), so no new open-state needs to be introduced.
const createType = computed<'probe' | 'cert'>(() => (probeModalOpen.value ? 'probe' : 'cert'))
const createModalOpen = computed(() => probeModalOpen.value || certModalOpen.value)
const isEditingCreateModal = computed(() =>
  createType.value === 'probe' ? !!probeForm.value.id : !!certForm.value.id
)

function closeCreateModal(): void {
  closeProbeModal()
  closeCertModal()
}

// ── Create-mode only: which resource(s) to create together, and the shared
// "Hôte / Cible" this exists for — a probe and a cert on the same target
// used to mean typing the same domain twice (once as the probe's URL, once
// as the cert's Host). Editing stays untouched (see isEditingCreateModal in
// the template): it's always exactly one resource, unambiguous from which
// row's edit button was clicked, so there's nothing to mutualize there.
const createIncludeProbe = ref(true)
const createIncludeCert = ref(false)

// openCreateProbe (the one entry point, MonitoringView's "Nouveau suivi"
// button and this panel's empty-state CTA) always opens with probe
// pre-selected; the toggle buttons above let the user add/switch to cert
// from there. openEditCert (an existing cert's own row) still opens
// cert-only, unaffected — isEditingCreateModal skips this watcher's effect
// entirely for edits.
watch(createModalOpen, (open) => {
  if (!open || isEditingCreateModal.value) return
  createIncludeProbe.value = probeModalOpen.value
  createIncludeCert.value = certModalOpen.value
})

function toggleIncludeProbe(): void {
  if (createIncludeProbe.value && !createIncludeCert.value) return // keep at least one selected
  createIncludeProbe.value = !createIncludeProbe.value
}

function toggleIncludeCert(): void {
  if (createIncludeCert.value && !createIncludeProbe.value) return
  createIncludeCert.value = !createIncludeCert.value
}

const sharedName = computed<string>({
  get: () => (createIncludeProbe.value ? probeForm.value.name : certForm.value.name),
  set: (v: string) => {
    if (createIncludeProbe.value) probeForm.value.name = v
    if (createIncludeCert.value) certForm.value.name = v
  },
})

// Keeps certForm.host/port in lockstep with the probe's own target field
// while both are selected, so the cert form has nothing left to fill in —
// parses whatever shape the probe type expects (a full URL for http,
// host:port for tcp, a bare host for icmp).
watch([() => probeForm.value.target, createIncludeProbe, createIncludeCert], () => {
  if (isEditingCreateModal.value || !createIncludeProbe.value || !createIncludeCert.value) return
  const raw = probeForm.value.target.trim()
  if (!raw) {
    certForm.value.host = ''
    return
  }
  let host = raw
  let port = 443
  if (/^https?:\/\//i.test(raw)) {
    try {
      const u = new URL(raw)
      host = u.hostname
      port = u.port ? Number(u.port) : (u.protocol === 'https:' ? 443 : 80)
    } catch {
      // Not a parseable URL — keep host as the raw string (already assigned above).
    }
  } else if (raw.includes(':')) {
    const [h, p] = raw.split(':')
    host = h
    if (p && !Number.isNaN(Number(p))) port = Number(p)
  } else {
    host = raw.split('/')[0]
  }
  certForm.value.host = host
  certForm.value.port = port
})

async function submitCreateModal(): Promise<void> {
  if (createIncludeProbe.value) {
    await saveProbe()
    if (probeModalOpen.value) return // save failed (error shown) — don't also attempt the cert
  }
  if (createIncludeCert.value) {
    await saveCert()
  }
}

const createModalRef = ref<HTMLElement | null>(null)
useModalChrome(createModalRef, () => createModalOpen.value, { onClose: closeCreateModal })

function rowLink(row: MonitoringRow): string {
  if (row.npmProxyHostId) return `/monitoring/host/${row.npmProxyHostId}`
  if (row.probe) return `/monitoring/probes/${row.probe.id}`
  return `/monitoring/ssl/${row.cert!.id}`
}

// Only openCreateProbe is called from outside (MonitoringView's single
// "Nouveau suivi" button) — the cert-only entry point was removed when the
// two header buttons were merged into one (the modal's own toggle covers
// it), don't re-expose openCreateCert here without a real caller again.
defineExpose({ openCreateProbe })
</script>
