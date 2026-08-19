<template>
  <div class="row g-2">
    <div
      v-if="showHost"
      class="col-md-3"
    >
      <label class="form-label form-label-sm">Hôte</label>
      <select
        v-model="hostId"
        class="form-select form-select-sm"
      >
        <option value="">
          Sélectionner un hôte...
        </option>
        <option
          v-for="host in hostsStore.hosts"
          :key="host.id"
          :value="host.id"
        >
          {{ host.name || host.hostname || host.ip_address }}
        </option>
      </select>
    </div>
    <div class="col-md-3">
      <label class="form-label form-label-sm">Module</label>
      <select
        v-model="module"
        class="form-select form-select-sm"
        @change="onModuleChange"
      >
        <option
          v-for="m in modules"
          :key="m.value"
          :value="m.value"
        >
          {{ m.label }}
        </option>
      </select>
    </div>
    <div
      v-if="module !== 'custom'"
      class="col-md-3"
    >
      <label class="form-label form-label-sm">Action</label>
      <select
        v-if="actionOptions.length"
        v-model="action"
        class="form-select form-select-sm"
      >
        <option
          v-for="a in actionOptions"
          :key="a.value"
          :value="a.value"
        >
          {{ a.label }}
        </option>
      </select>
      <input
        v-else
        v-model="action"
        type="text"
        class="form-control form-control-sm"
      >
    </div>
    <div
      v-if="targetInfo"
      class="col-md-3"
    >
      <label class="form-label form-label-sm">{{ targetInfo.label }}</label>
      <RestrictedSelect
        v-model="target"
        :options="targetOptions"
        :empty-label="targetEmptyLabel"
        :placeholder="targetInfo.placeholder"
      />
      <div
        v-if="module === 'restic' && resticGroups.length"
        class="form-hint"
      >
        Un groupe lance plusieurs profils en une seule exécution.
      </div>
    </div>
    <div
      v-if="showCron"
      class="col-12"
    >
      <label class="form-label form-label-sm">Planification</label>
      <CronBuilder v-model="cronExpression" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import apiClient from '../api'
import { useHostsStore } from '../stores/hosts'
import CronBuilder from './CronBuilder.vue'
import RestrictedSelect from './RestrictedSelect.vue'
import type { SelectOption, OptionGroup } from './RestrictedSelect.vue'
import type { CustomTaskSummary } from '../types/task'
import { DISPATCH_MODULES } from '../utils/dispatchStep'
import type { DispatchOption } from '../utils/dispatchStep'

const props = withDefaults(defineProps<{
  // Returns the selectable actions for a module, or [] to fall back to a
  // free-text input.
  actionsForModule: (module: string) => DispatchOption[]
  // Returns null to hide the target field for this module, or a label
  // (+ optional placeholder) to show it.
  targetConfig: (module: string) => { label: string; placeholder?: string } | null
  showCron?: boolean
  // Off when the host is implied by the caller's own context rather than
  // chosen here (e.g. an alert rule's command trigger dispatches on
  // whichever host the alert fired for — there is nothing to pick).
  showHost?: boolean
  // Restrict/relabel the module list for callers whose backend or product
  // scope doesn't cover the full DISPATCH_MODULES set.
  modules?: DispatchOption[]
}>(), {
  showHost: true,
  modules: () => DISPATCH_MODULES,
})

const hostId = defineModel<string>('hostId', { default: '' })
const module = defineModel<string>('module', { required: true })
const action = defineModel<string>('action', { required: true })
const target = defineModel<string>('target', { required: true })
const cronExpression = defineModel<string>('cronExpression', { default: '' })

const hostsStore = useHostsStore()

const actionOptions = computed(() => props.actionsForModule(module.value))
const targetInfo = computed(() => props.targetConfig(module.value))

async function onModuleChange(): Promise<void> {
  target.value = ''
  // The action <select>'s <option>s are re-rendered off the same module
  // change (via actionOptions), in the same tick. Setting the select's new
  // value before the DOM patch lands means the browser can't find a
  // matching <option> yet, so it silently shows no selection at all instead
  // of the new first action — wait for the patch, then set it.
  await nextTick()
  action.value = actionOptions.value[0]?.value || ''
}

// Custom-task target: the agent already reports each host's tasks.yaml-defined
// tasks (GET /hosts/:id/custom-tasks) — restrict the field to a <select> of
// exactly what the agent discovered rather than a blind free-text field, only
// falling back to text if nothing has been reported yet (host unreachable, or
// tasks.yaml empty).
const customTasks = ref<CustomTaskSummary[]>([])
const loadedForHost = ref('')

watch([hostId, module], ([h, m]) => {
  if (m !== 'custom' || !h || h === loadedForHost.value) return
  loadedForHost.value = h
  apiClient.getHostCustomTasks(h)
    .then((res) => { customTasks.value = res.data || [] })
    .catch(() => { customTasks.value = [] })
}, { immediate: true })

// Restic profile/group target: mirrors the custom-task <select> above — the
// agent reports the host's resticprofile.yaml profile names (GET
// /hosts/:id/backup/profiles) and "groups" section names (GET
// /hosts/:id/backup/groups) — a group runs several profiles together and
// resticprofile resolves it identically to a profile when passed to
// run_backup.sh's --name argument, so both are offered as separate option
// groups in the same <select> instead of a blind free-text field.
const resticProfiles = ref<string[]>([])
const resticGroups = ref<string[]>([])
const loadedResticForHost = ref('')

watch([hostId, module], ([h, m]) => {
  if (m !== 'restic' || !h || h === loadedResticForHost.value) return
  loadedResticForHost.value = h
  apiClient.getBackupProfiles(h)
    .then((res) => { resticProfiles.value = res.data?.profiles || [] })
    .catch(() => { resticProfiles.value = [] })
  apiClient.getBackupGroups(h)
    .then((res) => { resticGroups.value = res.data?.groups || [] })
    .catch(() => { resticGroups.value = [] })
}, { immediate: true })

// Feeds RestrictedSelect for every module: custom/restic get real discovered
// options, everything else (docker/apt/systemd/…) gets an empty list, which
// makes RestrictedSelect fall back to its own free-text input — the same
// behavior those modules always had.
const targetOptions = computed<SelectOption[] | OptionGroup[]>(() => {
  if (module.value === 'custom') {
    return customTasks.value.map((t) => ({ value: t.id, label: `${t.name} (${t.id})` }))
  }
  if (module.value === 'restic') {
    const groups: OptionGroup[] = []
    if (resticProfiles.value.length) groups.push({ label: 'Profils', options: resticProfiles.value })
    if (resticGroups.value.length) groups.push({ label: 'Groupes', options: resticGroups.value })
    return groups
  }
  return []
})

const targetEmptyLabel = computed(() => {
  if (module.value === 'custom') return 'Sélectionner une tâche...'
  if (module.value === 'restic') return 'Profil (défaut)'
  return ''
})
</script>
