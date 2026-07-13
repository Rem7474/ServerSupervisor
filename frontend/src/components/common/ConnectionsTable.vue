<template>
  <div class="table-responsive">
    <table class="table table-vcenter card-table">
      <thead>
        <tr>
          <th>Date / Heure</th>
          <th v-if="showUsername">
            Utilisateur
          </th>
          <th>IP</th>
          <th>Navigateur</th>
          <th>OS</th>
          <th>Statut</th>
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
          <td
            :colspan="columnCount"
            class="text-center text-secondary py-4"
          >
            Aucune connexion enregistrée
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
              :class="ev.success ? 'bg-green-lt text-green' : 'bg-red-lt text-red'"
            >
              {{ ev.success ? 'Succès' : 'Échec' }}
            </span>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { LoginEvent } from '../../types/generated'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import { parseUserAgent } from '../../utils/parseUserAgent'
import { formatDateTime } from '../../utils/formatters'

const props = withDefaults(defineProps<{
  events?: LoginEvent[]
  loading?: boolean
  // Audit's admin-wide view lists events across every user and needs the
  // extra column; the account-scoped views (Mon compte, Sécurité du compte)
  // only ever show the current user's own events.
  showUsername?: boolean
}>(), {
  events: () => [],
  loading: false,
  showUsername: false,
})

const columnCount = computed(() => (props.showUsername ? 6 : 5))
</script>
