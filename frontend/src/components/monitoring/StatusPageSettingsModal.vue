<template>
  <div
    v-if="open"
    ref="modalRef"
    class="modal modal-blur fade show d-block"
    tabindex="-1"
    role="dialog"
    aria-modal="true"
    :aria-labelledby="titleId"
  >
    <div class="modal-dialog modal-lg modal-dialog-centered modal-dialog-scrollable">
      <form
        class="modal-content"
        @submit.prevent="save"
      >
        <div class="modal-header">
          <h5
            :id="titleId"
            class="modal-title"
          >
            {{ t('monitoring.statusPage.modalTitle') }}
          </h5>
          <button
            type="button"
            class="btn-close"
            :aria-label="t('common.close')"
            :disabled="saving"
            @click="emit('close')"
          />
        </div>
        <div class="modal-body">
          <p class="text-secondary">
            {{ t('monitoring.statusPage.intro') }}
          </p>
          <div
            v-if="error"
            class="alert alert-danger"
          >
            {{ error }}
          </div>
          <LoadingSkeleton
            v-if="loading"
            variant="table"
            :lines="3"
          />
          <template v-else>
            <label class="form-check form-switch mb-1">
              <input
                v-model="form.enabled"
                type="checkbox"
                class="form-check-input"
              >
              <span class="form-check-label">{{ t('monitoring.statusPage.enabledLabel') }}</span>
            </label>
            <div class="small text-secondary mb-3">
              <template v-if="form.enabled">
                {{ t('monitoring.statusPage.publicUrl') }} :
                <a
                  :href="publicUrl"
                  target="_blank"
                  rel="noopener"
                >{{ publicUrl }}</a>
              </template>
              <template v-else>
                {{ t('monitoring.statusPage.disabledHint', { url: publicUrl }) }}
              </template>
            </div>

            <div class="row g-3 mb-3">
              <div class="col-md-6">
                <label
                  :for="titleInputId"
                  class="form-label"
                >{{ t('monitoring.statusPage.titleLabel') }}</label>
                <input
                  :id="titleInputId"
                  v-model="form.title"
                  type="text"
                  class="form-control"
                  maxlength="120"
                  :placeholder="t('monitoring.statusPage.titlePlaceholder')"
                >
              </div>
              <div class="col-md-6">
                <label
                  :for="descInputId"
                  class="form-label"
                >{{ t('monitoring.statusPage.descriptionLabel') }}</label>
                <input
                  :id="descInputId"
                  v-model="form.description"
                  type="text"
                  class="form-control"
                  maxlength="500"
                >
              </div>
            </div>

            <h4 class="mb-2">
              {{ t('monitoring.statusPage.servicesTitle') }}
              <span class="text-secondary fw-normal small ms-1">{{ t('monitoring.statusPage.publishedCount', { count: publishedCount }, publishedCount) }}</span>
            </h4>
            <div
              v-if="!rows.length"
              class="text-secondary"
            >
              {{ t('monitoring.statusPage.servicesEmpty') }}
            </div>
            <div
              v-else
              class="list-group list-group-flush border rounded"
            >
              <div
                v-for="row in rows"
                :key="row.probeId"
                class="list-group-item d-flex flex-column flex-sm-row align-items-sm-center gap-2"
                :data-probe="row.probeId"
              >
                <label class="form-check mb-0 flex-fill text-truncate">
                  <input
                    v-model="row.published"
                    type="checkbox"
                    class="form-check-input"
                    :aria-label="t('monitoring.statusPage.publishLabel', { name: row.probeName })"
                  >
                  <span class="form-check-label">{{ row.probeName }}</span>
                </label>
                <input
                  v-model="row.publicName"
                  type="text"
                  class="form-control form-control-sm public-name-input"
                  maxlength="80"
                  :disabled="!row.published"
                  :required="row.published"
                  :aria-label="t('monitoring.statusPage.publicNameLabel', { name: row.probeName })"
                  :placeholder="t('monitoring.statusPage.publicNamePlaceholder')"
                >
              </div>
            </div>
          </template>
        </div>
        <div class="modal-footer">
          <button
            type="button"
            class="btn link-secondary"
            :disabled="saving"
            @click="emit('close')"
          >
            {{ t('monitoring.cancelButton') }}
          </button>
          <button
            type="submit"
            class="btn btn-primary"
            :disabled="saving || loading"
          >
            {{ saving ? t('monitoring.savingLabel') : t('monitoring.saveButton') }}
          </button>
        </div>
      </form>
    </div>
  </div>
  <div
    v-if="open"
    class="modal-backdrop fade show"
  />
</template>

<script setup lang="ts">
import { computed, onMounted, ref, useId } from 'vue'
import { useI18n } from 'vue-i18n'
import api, { getApiErrorMessage } from '../../api'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import { useModalChrome } from '../../composables/useModalChrome'
import { addToast } from '../../composables/useGlobalToast'

interface Row {
  probeId: string
  probeName: string
  published: boolean
  publicName: string
}

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'saved'): void
}>()

const { t } = useI18n()
const titleId = useId()
const titleInputId = useId()
const descInputId = useId()

const open = ref(false)
const loading = ref(true)
const saving = ref(false)
const error = ref('')
const form = ref({ enabled: false, title: '', description: '' })
const rows = ref<Row[]>([])

const publicUrl = `${window.location.origin}/status`
const publishedCount = computed(() => rows.value.filter((r) => r.published).length)

const modalRef = ref<HTMLElement | null>(null)
useModalChrome(modalRef, () => open.value, { onClose: () => { if (!saving.value) emit('close') } })

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    const [cfgRes, probesRes] = await Promise.all([api.getStatusPageConfig(), api.getUptimeProbes()])
    const cfg = cfgRes.data
    form.value = { enabled: cfg.enabled, title: cfg.title, description: cfg.description }
    const published = new Map(cfg.services.map((s, i) => [s.probe_id, { name: s.public_name, pos: i }]))
    const probes = probesRes.data?.probes ?? []
    // Published services first, in their page order; the rest after, by name.
    rows.value = probes
      .map((p) => ({
        probeId: p.id,
        probeName: p.name,
        published: published.has(p.id),
        // Never prefilled with the internal probe name: what the public
        // reads is always typed on purpose.
        publicName: published.get(p.id)?.name ?? '',
      }))
      .sort((a, b) => {
        const pa = published.get(a.probeId)?.pos ?? Number.MAX_SAFE_INTEGER
        const pb = published.get(b.probeId)?.pos ?? Number.MAX_SAFE_INTEGER
        return pa - pb || a.probeName.localeCompare(b.probeName)
      })
  } catch (e: unknown) {
    error.value = getApiErrorMessage(e, t('monitoring.statusPage.loadError'))
  } finally {
    loading.value = false
  }
}

async function save(): Promise<void> {
  const services = rows.value
    .filter((r) => r.published)
    .map((r) => ({ probe_id: r.probeId, public_name: r.publicName.trim() }))
  if (services.some((s) => !s.public_name)) {
    error.value = t('monitoring.statusPage.publicNameRequired')
    return
  }
  saving.value = true
  error.value = ''
  try {
    await api.updateStatusPageConfig({ ...form.value, services })
    addToast(t('monitoring.statusPage.saved'), 'success')
    emit('saved')
    emit('close')
  } catch (e: unknown) {
    error.value = getApiErrorMessage(e, t('monitoring.statusPage.saveError'))
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  open.value = true
  void load()
})
</script>

<style scoped>
.public-name-input {
  max-width: 18rem;
}

@media (max-width: 575.98px) {
  .public-name-input {
    max-width: none;
  }
}
</style>
