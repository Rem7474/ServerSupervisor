<template>
  <div class="table-responsive scroll-table">
    <table class="table table-vcenter card-table">
      <thead>
        <tr>
          <th>{{ t('common.dateTime') }}</th>
          <th v-if="showUsername">
            {{ t('common.user') }}
          </th>
          <th>IP</th>
          <th>{{ t('common.browser') }}</th>
          <th>OS</th>
          <th>{{ t('common.status') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-if="loading">
          <td
            :colspan="columnCount"
            class="py-2"
          >
            <LoadingSkeleton
              variant="table"
              :lines="4"
            />
          </td>
        </tr>
        <tr v-else-if="!events.length">
          <td :colspan="columnCount">
            <EmptyState :title="t('common.noConnectionsRecorded')" />
          </td>
        </tr>
        <tr
          v-for="ev in events"
          :key="ev.id"
        >
          <td class="text-secondary small">
            {{ formatDateTime(ev.created_at) }}
          </td>
          <td
            v-if="showUsername"
            class="fw-semibold"
          >
            {{ ev.username }}
          </td>
          <td class="text-secondary small font-monospace">
            {{ ev.ip_address }}
          </td>
          <td class="text-secondary small">
            {{ parseUserAgent(ev.user_agent).browser }}
          </td>
          <td class="text-secondary small">
            {{ parseUserAgent(ev.user_agent).os }}
          </td>
          <td>
            <span
              class="badge"
              :class="ev.success ? 'bg-success-lt text-success' : 'bg-danger-lt text-danger'"
            >
              {{ ev.success ? t('common.success') : t('common.failure') }}
            </span>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { LoginEvent } from '../../types/generated'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import EmptyState from '../EmptyState.vue'
import { parseUserAgent } from '../../utils/parseUserAgent'
import { formatDateTime } from '../../utils/formatters'

const { t } = useI18n()

const props = withDefaults(defineProps<{
  events?: LoginEvent[]
  loading?: boolean
  // Audit's admin-wide view lists events across every user and needs the
  // extra column; the account-scoped views (AccountView, AccountSecurityView)
  // only ever show the current user's own events.
  showUsername?: boolean
}>(), {
  events: () => [],
  loading: false,
  showUsername: false,
})

const columnCount = computed(() => (props.showUsername ? 6 : 5))
</script>
