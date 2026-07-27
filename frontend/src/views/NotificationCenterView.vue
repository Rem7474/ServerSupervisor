<template>
  <div>
    <div class="page-header mb-4">
      <div class="page-pretitle">
        <router-link
          to="/"
          class="text-decoration-none"
        >
          Dashboard
        </router-link>
        <span class="text-muted mx-1">/</span>
        <span>Notifications</span>
      </div>
      <div class="d-flex align-items-center justify-content-between">
        <h2 class="page-title mb-0">
          Centre de notifications
          <span
            v-if="unreadCount > 0"
            class="badge bg-red text-white ms-2"
          >{{ unreadCount }}</span>
        </h2>
        <button
          v-if="unreadCount > 0"
          type="button"
          class="btn btn-sm btn-outline-secondary"
          :disabled="markingRead"
          @click="handleMarkRead"
        >
          <span
            v-if="markingRead"
            class="spinner-border spinner-border-sm me-1"
          />
          Tout marquer comme lu
        </button>
      </div>
    </div>

    <!-- Filters -->
    <div class="d-flex flex-wrap gap-2 mb-3">
      <div class="d-flex gap-1">
        <button
          v-for="f in SEVERITY_FILTERS"
          :key="f.value"
          type="button"
          class="btn btn-sm"
          :class="severityFilter === f.value ? 'btn-primary' : 'btn-outline-secondary'"
          @click="severityFilter = f.value"
        >
          {{ f.label }}
        </button>
      </div>
      <div class="d-flex gap-1">
        <button
          v-for="f in TYPE_FILTERS"
          :key="f.value"
          type="button"
          class="btn btn-sm"
          :class="typeFilter === f.value ? 'btn-secondary' : 'btn-outline-secondary'"
          @click="typeFilter = f.value"
        >
          {{ f.label }}
        </button>
      </div>
      <div class="d-flex gap-1 ms-auto">
        <button
          v-for="f in STATUS_FILTERS"
          :key="f.value"
          type="button"
          class="btn btn-sm"
          :class="statusFilter === f.value ? 'btn-secondary' : 'btn-outline-secondary'"
          @click="statusFilter = f.value"
        >
          {{ f.label }}
        </button>
      </div>
      <button
        type="button"
        class="btn btn-sm btn-outline-secondary"
        :title="groupByHost ? 'Afficher en liste chronologique' : 'Regrouper par hôte'"
        @click="toggleGroupByHost"
      >
        <IconStack2
          v-if="!groupByHost"
          :size="14"
          class="icon me-1"
        />
        <IconList
          v-else
          :size="14"
          class="icon me-1"
        />
        {{ groupByHost ? 'Vue chronologique' : 'Regrouper par hôte' }}
      </button>
    </div>

    <div
      v-if="error"
      class="alert alert-danger mb-3"
    >
      {{ error }}
    </div>

    <div class="card">
      <div
        v-if="loading && items.length === 0"
        class="card-body text-center text-muted py-5"
      >
        <div class="spinner-border mb-2" />
        <div>Chargement…</div>
      </div>

      <div
        v-else-if="items.length === 0"
        class="card-body text-center text-muted py-5"
      >
        Aucune notification.
      </div>

      <!-- Grouped by host -->
      <div v-else-if="groupByHost">
        <div
          v-for="group in groupedItems"
          :key="group.key"
          class="border-bottom"
        >
          <button
            type="button"
            class="btn btn-link text-decoration-none w-100 d-flex align-items-center gap-2 px-3 py-2 text-start"
            @click="toggleHostGroup(group.key)"
          >
            <IconChevronRight
              :size="16"
              class="icon transition-transform"
              :class="{ 'rotate-90': !isHostCollapsed(group.key) }"
            />
            <span class="fw-medium text-body">{{ group.hostName }}</span>
            <span class="badge bg-secondary-lt text-secondary">{{ group.items.length }}</span>
            <span
              v-if="group.unreadCount > 0"
              class="badge bg-red-lt text-red"
            >{{ group.unreadCount }} non lue{{ group.unreadCount > 1 ? 's' : '' }}</span>
          </button>
          <div
            v-if="!isHostCollapsed(group.key)"
            class="list-group list-group-flush"
          >
            <NotificationListItem
              v-for="item in group.items"
              :key="item.id"
              :item="item"
              :unread="isUnread(item)"
              :is-admin="auth.isAdmin"
              :resolving="resolvingId === item.id"
              @resolve="resolveIncident"
            />
          </div>
        </div>
      </div>

      <!-- Flat chronological list -->
      <div
        v-else
        class="list-group list-group-flush"
      >
        <NotificationListItem
          v-for="item in items"
          :key="item.id"
          :item="item"
          :unread="isUnread(item)"
          :is-admin="auth.isAdmin"
          :resolving="resolvingId === item.id"
          @resolve="resolveIncident"
        />
      </div>

      <div
        v-if="total > items.length"
        class="card-footer text-center text-muted small"
      >
        Affichage de {{ items.length }} / {{ total }} notifications.
        <button
          type="button"
          class="btn btn-sm btn-link p-0 ms-1"
          @click="loadMore"
        >
          Charger plus
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { IconChevronRight, IconList, IconStack2 } from '@tabler/icons-vue'
import NotificationListItem from '../components/NotificationListItem.vue'
import { useAuthStore } from '../stores/auth'
import { useNotificationCenter } from '../composables/useNotificationCenter'

const auth = useAuthStore()

const {
  SEVERITY_FILTERS,
  TYPE_FILTERS,
  STATUS_FILTERS,
  items,
  total,
  loading,
  error,
  markingRead,
  resolvingId,
  severityFilter,
  typeFilter,
  statusFilter,
  unreadCount,
  groupByHost,
  toggleGroupByHost,
  groupedItems,
  isHostCollapsed,
  toggleHostGroup,
  isUnread,
  resolveIncident,
  loadMore,
  handleMarkRead,
} = useNotificationCenter()
</script>

<style scoped>
.transition-transform {
  transition: transform 0.15s ease;
}
.rotate-90 {
  transform: rotate(90deg);
}
</style>
