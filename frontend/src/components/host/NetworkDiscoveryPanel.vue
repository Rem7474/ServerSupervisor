<template>
  <div>
    <template v-if="!bulkResults">
      <form
        class="mb-3"
        @submit.prevent="scan"
      >
        <label
          class="form-label"
          for="scan-cidr"
        >Sous-réseau (CIDR)</label>
        <div class="input-group">
          <input
            id="scan-cidr"
            v-model="cidr"
            type="text"
            class="form-control"
            placeholder="192.168.1.0/24"
            required
          >
          <button
            type="submit"
            class="btn btn-primary"
            :disabled="scanning"
          >
            {{ scanning ? 'Scan en cours…' : 'Scanner' }}
          </button>
        </div>
        <div class="form-hint">
          IPv4 uniquement, de /24 (254 adresses) à /30 (2 adresses) — ping ICMP de chaque
          adresse. Nécessite CAP_NET_RAW côté serveur (voir server/Dockerfile).
        </div>
      </form>

      <div
        v-if="scanError"
        class="alert alert-danger"
        role="alert"
      >
        {{ scanError }}
      </div>

      <div
        v-if="scanning"
        class="text-secondary small py-3"
      >
        <span class="spinner-border spinner-border-sm me-2" />
        Ping de chaque adresse en cours…
      </div>

      <template v-else-if="hasScanned">
        <EmptyState
          v-if="!results.length"
          title="Aucune adresse trouvée"
          subtitle="Vérifiez le sous-réseau saisi."
        />
        <template v-else>
          <div class="d-flex align-items-center justify-content-between mb-2">
            <div class="text-secondary small">
              {{ results.length }} adresse(s) scannée(s), {{ candidates.length }} nouvelle(s)
              disponible(s)
            </div>
            <button
              v-if="candidates.length"
              type="button"
              class="btn btn-sm btn-ghost-secondary"
              @click="toggleSelectAll"
            >
              {{ allSelected ? 'Tout désélectionner' : 'Tout sélectionner' }}
            </button>
          </div>

          <div class="table-responsive mb-3">
            <table class="table table-vcenter card-table table-sm">
              <thead>
                <tr>
                  <th style="width: 32px" />
                  <th>Adresse IP</th>
                  <th>Statut</th>
                  <th>Nom de l'hôte</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="r in results"
                  :key="r.ip_address"
                >
                  <td>
                    <input
                      v-if="r.responded && !r.already_registered"
                      type="checkbox"
                      class="form-check-input"
                      :checked="selected.has(r.ip_address)"
                      :aria-label="`Sélectionner ${r.ip_address}`"
                      @change="toggleSelected(r.ip_address)"
                    >
                  </td>
                  <td class="font-monospace">
                    {{ r.ip_address }}
                  </td>
                  <td>
                    <span
                      v-if="r.already_registered"
                      class="badge bg-azure-lt"
                    >
                      Déjà enregistré{{ r.existing_host_name ? ` — ${r.existing_host_name}` : '' }}
                    </span>
                    <span
                      v-else-if="r.responded"
                      class="badge bg-success-lt"
                    >
                      Répond{{ r.latency_ms ? ` (${r.latency_ms} ms)` : '' }}
                    </span>
                    <span
                      v-else
                      class="badge bg-secondary-lt"
                    >
                      Ne répond pas
                    </span>
                  </td>
                  <td>
                    <input
                      v-if="r.responded && !r.already_registered"
                      v-model="names[r.ip_address]"
                      type="text"
                      class="form-control form-control-sm"
                      :aria-label="`Nom de l'hôte pour ${r.ip_address}`"
                    >
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <div
            v-if="addError"
            class="alert alert-danger"
            role="alert"
          >
            {{ addError }}
          </div>

          <button
            v-if="candidates.length"
            type="button"
            class="btn btn-primary"
            :disabled="!selected.size || adding"
            @click="addSelected"
          >
            {{ adding ? 'Ajout…' : `Ajouter ${selected.size} hôte(s) sélectionné(s)` }}
          </button>
        </template>
      </template>
    </template>

    <div
      v-else
      class="host-success"
      role="alert"
    >
      <div class="host-success-header">
        <div class="fw-semibold">
          {{ createdCount }} hôte(s) ajouté(s)
        </div>
        <button
          type="button"
          class="btn btn-success"
          @click="finish"
        >
          Terminé
        </button>
      </div>
      <div class="text-secondary small mb-3">
        Les clés API ne seront plus affichées. Copiez-les maintenant.
      </div>

      <div class="d-flex flex-column gap-2">
        <div
          v-for="item in bulkResults"
          :key="item.ip_address"
          class="host-success-card"
        >
          <template v-if="item.created">
            <div class="d-flex align-items-center justify-content-between mb-2">
              <div>
                <span class="fw-medium">{{ item.name }}</span>
                <span class="text-secondary small ms-2 font-monospace">{{ item.ip_address }}</span>
              </div>
            </div>
            <div class="host-success-key">
              <code>{{ item.api_key }}</code>
              <button
                type="button"
                class="btn btn-outline-secondary btn-sm"
                @click="copyKey(item)"
              >
                {{ copiedKeys[item.ip_address] ? 'Copié' : 'Copier' }}
              </button>
            </div>
          </template>
          <div
            v-else
            class="text-danger small"
          >
            <span class="font-monospace">{{ item.ip_address }}</span> — {{ item.error }}
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive } from 'vue'
import EmptyState from '../EmptyState.vue'
import { useNetworkDiscovery } from '../../composables/useNetworkDiscovery'

const emit = defineEmits<{ done: [] }>()

const {
  cidr,
  scanning,
  scanError,
  results,
  hasScanned,
  candidates,
  names,
  selected,
  allSelected,
  scan,
  toggleSelected,
  toggleSelectAll,
  adding,
  addError,
  bulkResults,
  addSelected,
} = useNetworkDiscovery()

const createdCount = computed(() => bulkResults.value?.filter((r) => r.created).length ?? 0)

const copiedKeys = reactive<Record<string, boolean>>({})

async function copyKey(item: { ip_address: string; api_key?: string }): Promise<void> {
  if (!item.api_key) return
  await navigator.clipboard.writeText(item.api_key)
  copiedKeys[item.ip_address] = true
  setTimeout(() => {
    copiedKeys[item.ip_address] = false
  }, 1500)
}

function finish(): void {
  emit('done')
}
</script>

<style scoped>
.host-success {
  background: var(--ss-panel-medium);
  border: 1px solid rgba(56, 189, 248, 0.35);
  border-radius: 14px;
  padding: 20px;
  color: var(--ss-text-on-dark);
}

.host-success-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 8px;
}

.host-success-card {
  background: var(--ss-panel-strong);
  border: 1px solid var(--ss-border-default);
  border-radius: 12px;
  padding: 14px;
}

.host-success-key {
  display: flex;
  align-items: center;
  gap: 10px;
}

.host-success-key code {
  display: block;
  background: rgba(2, 6, 23, 0.6);
  color: var(--ss-text-strong);
  padding: 8px 10px;
  border-radius: 8px;
  flex: 1;
  word-break: break-all;
}
</style>
