<template>
  <div class="card mb-3">
    <div class="card-header d-flex align-items-center justify-content-between">
      <h3 class="card-title mb-0">
        {{ t('webhooks.versionHistoryTitle') }}
      </h3>
      <small class="text-muted">{{ t('webhooks.publicationDetectionSubtitle') }}</small>
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
      <EmptyState
        v-else-if="!history.length"
        :title="t('webhooks.noVersionAvailableTitle')"
      />
      <div
        v-else
        class="table-responsive scroll-table"
      >
        <table class="table table-sm table-vcenter mb-0">
          <thead>
            <tr>
              <th>{{ t('webhooks.versionColumn') }}</th>
              <th>{{ t('webhooks.detailsColumn') }}</th>
              <th class="text-end">
                {{ t('webhooks.publicationDateColumn') }}
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
          type="button"
          class="btn btn-outline-secondary btn-sm"
          @click="showAll = !showAll"
        >
          {{ showAll
            ? t('webhooks.showLessButton')
            : t('webhooks.showMoreButton', { n: history.length - PREVIEW_LIMIT }) }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import EmptyState from '../EmptyState.vue'
import { formatDateTime } from '../../utils/formatters'
import type { ReleaseVersionHistoryItem } from '../../types/tracker'

const { t } = useI18n()

const props = defineProps<{
  history: ReleaseVersionHistoryItem[]
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
