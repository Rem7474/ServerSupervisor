<template>
  <div class="card">
    <div class="card-header d-flex align-items-center justify-content-between">
      <div>
        <h3 class="card-title mb-1">
          Modèles de règles
        </h3>
        <div class="text-muted small">
          Définissez une règle une fois, appliquez-la à plusieurs hôtes d'un coup.
        </div>
      </div>
      <button
        v-if="isAdmin"
        type="button"
        class="btn btn-primary btn-sm"
        @click="$emit('add')"
      >
        <IconPlus
          :size="14"
          class="icon me-1"
        />
        Nouveau modèle
      </button>
    </div>

    <LoadingSkeleton
      v-if="loading && !fetched"
      variant="table"
      :lines="3"
      class="m-3"
    />
    <div
      v-else-if="templates.length === 0"
      class="card-body"
    >
      <EmptyState
        title="Aucun modèle de règle"
        subtitle="Créez un modèle pour appliquer la même règle (métrique, seuils, notifications) à plusieurs hôtes en une fois."
      />
    </div>
    <div
      v-else
      class="table-responsive"
    >
      <table class="table table-vcenter card-table">
        <thead>
          <tr>
            <th>Nom</th>
            <th>Métrique</th>
            <th>Seuils</th>
            <th class="text-end">
              Actions
            </th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="t in templates"
            :key="t.id"
          >
            <td class="fw-semibold">
              {{ t.name }}
            </td>
            <td>
              <code class="small">{{ t.metric }} {{ t.operator }}</code>
            </td>
            <td class="text-muted small">
              avert. {{ t.threshold_warn }} · crit. {{ t.threshold_crit }}
            </td>
            <td class="text-end">
              <button
                v-if="isAdmin"
                type="button"
                class="btn btn-sm btn-ghost-secondary"
                title="Appliquer à des hôtes"
                @click="$emit('apply', t)"
              >
                <IconTargetArrow :size="14" />
                Appliquer
              </button>
              <button
                v-if="isAdmin"
                type="button"
                class="btn btn-icon btn-sm btn-ghost-secondary"
                title="Modifier"
                aria-label="Modifier le modèle"
                @click="$emit('edit', t)"
              >
                <IconPencil :size="14" />
              </button>
              <button
                v-if="isAdmin"
                type="button"
                class="btn btn-icon btn-sm btn-ghost-danger"
                title="Supprimer"
                aria-label="Supprimer le modèle"
                @click="$emit('delete', t)"
              >
                <IconTrash :size="14" />
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { IconPencil, IconPlus, IconTargetArrow, IconTrash } from '@tabler/icons-vue'
import EmptyState from '../EmptyState.vue'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import type { AlertRuleTemplate } from '../../types/generated'

withDefaults(defineProps<{
  templates?: AlertRuleTemplate[]
  loading?: boolean
  fetched?: boolean
  isAdmin?: boolean
}>(), {
  templates: () => [],
  loading: false,
  fetched: false,
  isAdmin: false,
})

defineEmits<{
  (e: 'add'): void
  (e: 'edit', template: AlertRuleTemplate): void
  (e: 'delete', template: AlertRuleTemplate): void
  (e: 'apply', template: AlertRuleTemplate): void
}>()
</script>
