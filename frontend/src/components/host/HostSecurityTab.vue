<template>
  <div
    class="card"
  >
    <div class="card-header d-flex align-items-center justify-content-between">
      <h3 class="card-title mb-0 d-flex align-items-center gap-2">
        <IconLock
          :size="16"
          class="icon icon-sm text-warning"
        />
        {{ t('host.perHostPermissionsTitle') }}
      </h3>
      <span class="badge badge-sm bg-danger text-white">{{ t('common.adminOnly') }}</span>
    </div>
    <div class="card-body p-0">
      <div v-if="permLoading">
        <LoadingSkeleton variant="table" />
      </div>
      <EmptyState
        v-else-if="!hostPerms.length"
        :title="t('host.noRestrictionsTitle')"
        :subtitle="t('host.noRestrictionsSubtitle')"
      />
      <div
        v-else
        class="table-responsive"
      >
        <table
          class="table table-vcenter mb-0"
        >
          <thead>
            <tr>
              <th>{{ t('host.userColumn') }}</th>
              <th>{{ t('host.levelColumn') }}</th>
              <th />
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="p in hostPerms"
              :key="p.username"
            >
              <td>{{ p.username }}</td>
              <td>
                <span :class="p.level === 'operator' ? 'badge bg-blue-lt' : 'badge bg-secondary-lt'">
                  {{ p.level }}
                </span>
              </td>
              <td class="text-end">
                <button
                  type="button"
                  class="btn btn-icon btn-sm btn-ghost-danger"
                  :title="t('host.revokeTooltip')"
                  :aria-label="t('host.revokeTooltip')"
                  @click="$emit('revoke', p.username)"
                >
                  <IconX
                    :size="16"
                    class="icon icon-sm"
                  />
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
    <div class="card-footer d-flex justify-content-end">
      <button
        type="button"
        class="btn btn-sm btn-outline-primary"
        @click="$emit('add')"
      >
        {{ t('host.addPermissionLabel') }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { IconLock, IconX } from '@tabler/icons-vue'
import { useI18n } from 'vue-i18n'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import EmptyState from '../EmptyState.vue'
import type { HostRecord } from '../../types/hostDetail'

defineProps<{
  hostPerms: HostRecord[]
  permLoading: boolean
}>()

defineEmits<{
  revoke: [username: string]
  add: []
}>()

const { t } = useI18n()
</script>
