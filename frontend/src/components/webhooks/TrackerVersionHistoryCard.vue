<template>
  <div class="card mb-3">
    <div class="card-header d-flex align-items-center justify-content-between">
      <h3 class="card-title mb-0">
        Historique des versions
      </h3>
      <small class="text-muted">Publication / détection</small>
    </div>
    <div class="card-body p-0">
      <div
        v-if="loading"
        class="p-3"
      >
        <LoadingSkeleton
          variant="table"
          :lines="4"
        />
      </div>
      <div
        v-else-if="!history.length"
        class="p-3 text-muted"
      >
        Aucune version disponible.
      </div>
      <div
        v-else
        class="table-responsive"
      >
        <table class="table table-sm table-vcenter mb-0">
          <thead>
            <tr>
              <th>Version</th>
              <th>Détails</th>
              <th class="text-end">
                Date de publication
              </th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="entry in visibleHistory"
              :key="`${entry.version}-${entry.published_at || 'n/a'}`"
            >
              <td>
                <span class="badge bg-green-lt text-green">{{ entry.version }}</span>
              </td>
              <td>
                <a
                  v-if="entry.release_url"
                  :href="entry.release_url"
                  target="_blank"
                  class="link-primary"
                >
                  {{ entry.name || entry.release_url }}
                </a>
                <span v-else>{{ entry.name || '-' }}</span>
              </td>
              <td class="text-end text-muted">
                {{ entry.published_at ? formatDateTime(entry.published_at) : 'N/A' }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div
        v-if="history.length > PREVIEW_LIMIT"
        class="p-2 border-top text-center"
      >
        <button
          class="btn btn-outline-secondary btn-sm"
          @click="showAll = !showAll"
        >
          {{ showAll
            ? 'Afficher moins'
            : `Afficher plus (${history.length - PREVIEW_LIMIT})` }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import { formatDateTime } from '../../utils/formatters'

const props = defineProps<{
  history: any[]
  loading: boolean
}>()

const PREVIEW_LIMIT = 5
const showAll = ref(false)

// Collapse back to the preview whenever a fresh history is loaded.
watch(() => props.history, () => { showAll.value = false })

const visibleHistory = computed(() =>
  showAll.value ? props.history : props.history.slice(0, PREVIEW_LIMIT),
)
</script>
