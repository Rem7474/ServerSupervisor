<template>
  <DataToolbar
    searchable
    :search="search"
    search-placeholder="Rechercher un hôte..."
    @update:search="search = $event"
  >
    <template #right>
      <div class="btn-group">
        <button
          v-for="f in filterOptions"
          :key="f.value"
          class="btn btn-sm"
          :class="quickFilter === f.value ? 'btn-primary' : 'btn-outline-secondary'"
          @click="quickFilter = f.value"
        >
          {{ f.label }}
        </button>
      </div>
      <select
        v-model="sortKey"
        class="form-select form-select-sm sort-select"
      >
        <option value="name">
          Trier par nom
        </option>
        <option value="pending">
          Trier par paquets en attente
        </option>
        <option value="security">
          Trier par mises à jour sécurité
        </option>
        <option value="cve">
          Trier par CVE
        </option>
      </select>
      <button
        class="btn btn-sm btn-outline-secondary"
        :title="sortDir === 'asc' ? 'Croissant' : 'Décroissant'"
        @click="sortDir = sortDir === 'asc' ? 'desc' : 'asc'"
      >
        <svg
          v-if="sortDir === 'asc'"
          xmlns="http://www.w3.org/2000/svg"
          width="16"
          height="16"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
        ><path d="M3 8l4-4 4 4M7 4v16M13 16l4 4 4-4M17 20V4" /></svg>
        <svg
          v-else
          xmlns="http://www.w3.org/2000/svg"
          width="16"
          height="16"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
        ><path d="M3 16l4 4 4-4M7 20V4M13 8l4-4 4 4M17 4v16" /></svg>
      </button>
    </template>
    <template #bottom>
      <div class="d-flex flex-wrap align-items-center gap-3">
        <label class="form-check">
          <input
            v-model="allSelected"
            type="checkbox"
            class="form-check-input"
          >
          <span class="form-check-label">Sélectionner tous les hôtes</span>
        </label>
        <div class="ms-auto d-flex flex-wrap gap-2">
          <template v-if="canRunApt && selectedCount > 0">
            <button
              class="btn btn-outline-secondary btn-sm"
              :disabled="!!bulkLoading"
              @click="$emit('bulk-cmd', 'update')"
            >
              <span
                v-if="bulkLoading === 'update'"
                class="spinner-border spinner-border-sm me-1"
                role="status"
              />
              apt update ({{ selectedCount }})
            </button>
            <button
              class="btn btn-primary btn-sm"
              :disabled="!!bulkLoading"
              @click="$emit('bulk-cmd', 'upgrade')"
            >
              <span
                v-if="bulkLoading === 'upgrade'"
                class="spinner-border spinner-border-sm me-1"
                role="status"
              />
              apt upgrade ({{ selectedCount }})
            </button>
            <button
              class="btn btn-outline-danger btn-sm"
              :disabled="!!bulkLoading"
              @click="$emit('bulk-cmd', 'dist-upgrade')"
            >
              <span
                v-if="bulkLoading === 'dist-upgrade'"
                class="spinner-border spinner-border-sm me-1"
                role="status"
              />
              apt dist-upgrade ({{ selectedCount }})
            </button>
          </template>
          <span
            v-else-if="selectedCount === 0"
            class="text-secondary small align-self-center"
          >Sélectionner des hôtes pour les actions groupées</span>
        </div>
      </div>
    </template>
  </DataToolbar>
</template>

<script setup lang="ts">
import DataToolbar from '../common/DataToolbar.vue'

defineProps<{
  filterOptions: { value: string, label: string }[]
  canRunApt: boolean
  selectedCount: number
  bulkLoading: string | null
}>()

defineEmits<{
  (e: 'bulk-cmd', command: string): void
}>()

const search = defineModel<string>('search', { required: true })
const quickFilter = defineModel<string>('quickFilter', { required: true })
const sortKey = defineModel<'name' | 'pending' | 'security' | 'cve'>('sortKey', { required: true })
const sortDir = defineModel<'asc' | 'desc'>('sortDir', { required: true })
const allSelected = defineModel<boolean>('allSelected', { required: true })
</script>

<style scoped>
.sort-select { width: auto; }
</style>
