<template>
  <div class="card mb-4">
    <div class="card-header d-flex align-items-center justify-content-between">
      <h3 class="card-title mb-0">
        {{ t('settings.hostGroupsTitle') }}
      </h3>
      <button
        v-if="authIsAdmin && !showForm"
        type="button"
        class="btn btn-sm btn-primary"
        @click="openAddForm"
      >
        {{ t('settings.addHostGroup') }}
      </button>
    </div>

    <div class="card-body border-bottom py-2">
      <p class="text-muted small mb-0">
        {{ t('settings.hostGroupsDesc') }}
      </p>
    </div>

    <!-- Add / Edit form -->
    <div
      v-if="showForm && authIsAdmin"
      class="card-body border-bottom"
    >
      <div class="row g-3">
        <div class="col-md-6">
          <label
            :for="`${uid}-name`"
            class="form-label"
          >{{ t('settings.name') }} *</label>
          <input
            :id="`${uid}-name`"
            v-model="form.name"
            type="text"
            class="form-control"
            maxlength="100"
            :placeholder="t('settings.hostGroupNamePlaceholder')"
          >
        </div>
        <div class="col-md-6">
          <label
            :for="`${uid}-mode`"
            class="form-label"
          >{{ t('settings.hostGroupMatchModeLabel') }}</label>
          <select
            :id="`${uid}-mode`"
            v-model="form.match_mode"
            class="form-select"
          >
            <option value="all">
              {{ t('settings.hostGroupMatchAll') }}
            </option>
            <option value="any">
              {{ t('settings.hostGroupMatchAny') }}
            </option>
          </select>
        </div>
        <div class="col-md-6">
          <label
            :for="`${uid}-tags`"
            class="form-label"
          >{{ t('settings.hostGroupTagsLabel') }}</label>
          <input
            :id="`${uid}-tags`"
            v-model="form.tags"
            type="text"
            class="form-control"
            :aria-describedby="`${uid}-tags-hint`"
            placeholder="prod, web"
          >
          <div
            :id="`${uid}-tags-hint`"
            class="form-hint"
          >
            {{ t('settings.hostGroupTagsHint') }}
          </div>
          <div
            v-if="knownTags.length"
            class="d-flex flex-wrap gap-1 mt-2"
          >
            <button
              v-for="tag in knownTags"
              :key="tag"
              type="button"
              class="btn btn-sm py-0 px-2"
              :class="formTags.some(t2 => t2.toLowerCase() === tag.toLowerCase()) ? 'btn-primary' : 'btn-outline-secondary'"
              :aria-pressed="formTags.some(t2 => t2.toLowerCase() === tag.toLowerCase())"
              @click="toggleTag(tag)"
            >
              {{ tag }}
            </button>
          </div>
        </div>
        <div class="col-md-6">
          <label
            :for="`${uid}-os`"
            class="form-label"
          >{{ t('settings.hostGroupOsLabel') }}</label>
          <input
            :id="`${uid}-os`"
            v-model="form.os_contains"
            type="text"
            class="form-control"
            maxlength="100"
            :aria-describedby="`${uid}-os-hint`"
            placeholder="debian"
          >
          <div
            :id="`${uid}-os-hint`"
            class="form-hint"
          >
            {{ t('settings.hostGroupOsHint') }}
          </div>
        </div>
        <div class="col-12">
          <label
            :for="`${uid}-desc`"
            class="form-label"
          >{{ t('settings.hostGroupDescriptionLabel') }}</label>
          <input
            :id="`${uid}-desc`"
            v-model="form.description"
            type="text"
            class="form-control"
          >
        </div>
        <div
          class="col-12 small"
          aria-live="polite"
        >
          <span class="text-muted">{{ t('settings.hostGroupPreview', { count: previewMembers.length }, previewMembers.length) }}</span>
          <span v-if="previewMembers.length"> — {{ previewMembers.map(hostLabel).join(', ') }}</span>
        </div>
      </div>
      <div class="mt-3 d-flex align-items-center gap-2">
        <button
          type="button"
          class="btn btn-primary"
          :disabled="saving"
          @click="save"
        >
          {{ saving ? t('common.saving') : (editingId ? t('settings.update') : t('settings.create')) }}
        </button>
        <button
          type="button"
          class="btn btn-outline-secondary"
          @click="cancelForm"
        >
          {{ t('settings.cancel') }}
        </button>
        <span
          v-if="formMsg"
          :class="['ms-auto small', formOk ? 'text-success' : 'text-danger']"
        >{{ formMsg }}</span>
      </div>
    </div>

    <!-- List -->
    <div class="table-responsive">
      <table class="table table-vcenter card-table">
        <thead>
          <tr>
            <th>{{ t('settings.name') }}</th>
            <th>{{ t('settings.hostGroupRuleColumn') }}</th>
            <th>{{ t('settings.hostGroupMembersColumn') }}</th>
            <th v-if="authIsAdmin" />
          </tr>
        </thead>
        <tbody>
          <tr v-if="loading && !groups.length">
            <td colspan="4">
              <LoadingSkeleton variant="table" />
            </td>
          </tr>
          <tr v-else-if="groups.length === 0">
            <td colspan="4">
              <EmptyState :title="t('settings.noHostGroups')" />
            </td>
          </tr>
          <tr
            v-for="group in groups"
            :key="group.id"
          >
            <td>
              <div class="fw-medium">
                {{ group.name }}
              </div>
              <div
                v-if="group.description"
                class="text-muted small"
              >
                {{ group.description }}
              </div>
            </td>
            <td class="small">
              <span
                v-if="group.tags.length"
                class="text-muted me-1"
              >{{ group.match_mode === 'any' ? t('settings.hostGroupAnyOf') : t('settings.hostGroupAllOf') }}</span>
              <span
                v-for="tag in group.tags"
                :key="tag"
                class="badge bg-secondary-lt text-secondary me-1"
              >{{ tag }}</span>
              <span
                v-if="group.os_contains"
                class="text-muted"
              >{{ t('settings.hostGroupOsSummary', { os: group.os_contains }) }}</span>
            </td>
            <td class="small">
              <span
                class="badge bg-blue-lt text-blue"
                :title="memberNames(group)"
              >{{ t('settings.hostGroupMemberCount', { count: group.member_ids.length }, group.member_ids.length) }}</span>
            </td>
            <td
              v-if="authIsAdmin"
              class="text-end"
            >
              <div class="d-flex gap-1 justify-content-end">
                <button
                  type="button"
                  class="btn btn-icon btn-sm btn-ghost-secondary"
                  :title="t('settings.edit')"
                  :aria-label="t('settings.editHostGroupAriaLabel', { name: group.name })"
                  @click="openEditForm(group)"
                >
                  <IconPencil
                    :size="16"
                    class="icon icon-sm"
                  />
                </button>
                <button
                  type="button"
                  class="btn btn-icon btn-sm btn-ghost-danger"
                  :title="t('settings.delete')"
                  :aria-label="t('settings.deleteHostGroupAriaLabel', { name: group.name })"
                  @click="remove(group)"
                >
                  <IconTrash
                    :size="16"
                    class="icon icon-sm"
                  />
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div
      v-if="listMsg"
      class="card-footer"
    >
      <span :class="['small', listOk ? 'text-success' : 'text-danger']">{{ listMsg }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, useId } from 'vue'
import { useI18n } from 'vue-i18n'
import { storeToRefs } from 'pinia'
import { IconPencil, IconTrash } from '@tabler/icons-vue'
import api from '../../api/index'
import type { HostGroup, HostGroupRequest } from '../../types/generated'
import { useConnectionCrud } from '../../composables/useConnectionCrud'
import { useHostsStore } from '../../stores/hosts'
import { collectHostTags, formatTagsInput, hostMatchesGroup, parseTagsInput } from '../../utils/tags'
import EmptyState from '../EmptyState.vue'
import LoadingSkeleton from '../LoadingSkeleton.vue'

const uid = useId()
const { t } = useI18n()

interface GroupForm {
  name: string
  description: string
  match_mode: string
  tags: string
  os_contains: string
}

withDefaults(defineProps<{
  authIsAdmin?: boolean
}>(), {
  authIsAdmin: false,
})

const hostsStore = useHostsStore()
const { hosts } = storeToRefs(hostsStore)
onMounted(() => {
  hostsStore.fetchHosts().catch(() => {
    // The preview and member names degrade to ids; the list still works.
  })
})

const toRequest = (f: GroupForm): HostGroupRequest => ({
  name: f.name,
  description: f.description,
  match_mode: f.match_mode,
  tags: parseTagsInput(f.tags),
  os_contains: f.os_contains,
})

const {
  items: groups, loading, showForm, editingId, form, saving, formMsg, formOk, listMsg, listOk,
  openAddForm, openEditForm, cancelForm, save, remove,
} = useConnectionCrud<HostGroup, GroupForm>({
  list: async () => {
    const data = (await api.getHostGroups()).data
    return Array.isArray(data) ? data : []
  },
  create: (f) => api.createHostGroup(toRequest(f)),
  update: (id, f) => api.updateHostGroup(id, toRequest(f)),
  remove: (id) => api.deleteHostGroup(id),
  emptyForm: () => ({ name: '', description: '', match_mode: 'all', tags: '', os_contains: '' }),
  toForm: (g) => ({
    name: g.name,
    description: g.description,
    match_mode: g.match_mode || 'all',
    tags: formatTagsInput(g.tags),
    os_contains: g.os_contains,
  }),
  validate: (f) => {
    if (!f.name.trim()) return t('settings.hostGroupNameRequired')
    if (parseTagsInput(f.tags).length === 0 && !f.os_contains.trim()) return t('settings.hostGroupCriteriaRequired')
    return ''
  },
  keys: {
    created: 'settings.hostGroupCreated',
    updated: 'settings.hostGroupUpdated',
    deleted: 'settings.hostGroupDeleted',
    deleteTitle: 'settings.deleteHostGroupTitle',
    deleteMessage: 'settings.deleteHostGroupMsg',
  },
})

const knownTags = computed(() => collectHostTags(hosts.value))
const formTags = computed(() => parseTagsInput(form.value.tags))

function toggleTag(tag: string): void {
  const current = formTags.value
  const next = current.some(t2 => t2.toLowerCase() === tag.toLowerCase())
    ? current.filter(t2 => t2.toLowerCase() !== tag.toLowerCase())
    : [...current, tag]
  form.value.tags = formatTagsInput(next)
}

const previewMembers = computed(() => {
  const rule = { match_mode: form.value.match_mode, tags: formTags.value, os_contains: form.value.os_contains }
  return hosts.value.filter(h => hostMatchesGroup(h, rule))
})

function hostLabel(h: { id: string; name?: string; hostname?: string }): string {
  return h.name || h.hostname || h.id
}

function memberNames(group: HostGroup): string {
  return group.member_ids
    .map(id => {
      const h = hosts.value.find(x => x.id === id)
      return h ? hostLabel(h) : id
    })
    .join(', ')
}
</script>
