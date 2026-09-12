<template>
  <div class="card">
    <div class="card-header d-flex align-items-center justify-content-between">
      <div>
        <h3 class="card-title mb-1">
          {{ t('alerts.templatesTitle') }}
        </h3>
        <div class="text-muted small">
          {{ t('alerts.templatesSubtitle') }}
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
        {{ t('alerts.newTemplateButton') }}
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
        :title="t('alerts.noTemplatesTitle')"
        :subtitle="t('alerts.noTemplatesSubtitle')"
      />
    </div>
    <div
      v-else
      class="table-responsive"
    >
      <table class="table table-vcenter card-table">
        <thead>
          <tr>
            <th>{{ t('alerts.nameColumn') }}</th>
            <th>{{ t('alerts.metricColumn') }}</th>
            <th>{{ t('alerts.thresholdsColumn') }}</th>
            <th class="text-end">
              {{ t('alerts.actionsColumn') }}
            </th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="tpl in templates"
            :key="tpl.id"
          >
            <td class="fw-semibold">
              {{ tpl.name }}
            </td>
            <td>
              <code class="small">{{ tpl.metric }} {{ tpl.operator }}</code>
            </td>
            <td class="text-muted small">
              {{ t('alerts.thresholdsSummary', { warn: tpl.threshold_warn, crit: tpl.threshold_crit }) }}
            </td>
            <td class="text-end">
              <button
                v-if="isAdmin"
                type="button"
                class="btn btn-sm btn-ghost-secondary"
                :title="t('alerts.applyToHostsTooltip')"
                @click="$emit('apply', tpl)"
              >
                <IconTargetArrow :size="14" />
                {{ t('alerts.applyButton') }}
              </button>
              <button
                v-if="isAdmin"
                type="button"
                class="btn btn-icon btn-sm btn-ghost-secondary"
                :title="t('alerts.editTooltip')"
                :aria-label="t('alerts.editTemplateAriaLabel')"
                @click="$emit('edit', tpl)"
              >
                <IconPencil :size="14" />
              </button>
              <button
                v-if="isAdmin"
                type="button"
                class="btn btn-icon btn-sm btn-ghost-danger"
                :title="t('alerts.deleteTooltip')"
                :aria-label="t('alerts.deleteTemplateAriaLabel')"
                @click="$emit('delete', tpl)"
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
import { useI18n } from 'vue-i18n'
import { IconPencil, IconPlus, IconTargetArrow, IconTrash } from '@tabler/icons-vue'
import EmptyState from '../EmptyState.vue'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import type { AlertRuleTemplate } from '../../types/generated'

const { t } = useI18n()

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
