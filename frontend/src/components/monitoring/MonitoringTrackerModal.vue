<template>
  <!-- ===== MODAL SUIVI (sonde uptime / certificat SSL) =====
       One shared modal for both "add a monitoring target" flows — a probe
       and a cert are different resources with different fields, so their
       bodies stay distinct, but they share one entry point, one chrome
       (header/footer/backdrop) and a type switcher instead of two
       independent modals a user could otherwise open side by side. -->
  <div
    v-if="createModalOpen"
    ref="createModalRef"
    class="modal modal-blur fade show d-block"
    tabindex="-1"
    role="dialog"
    aria-modal="true"
  >
    <div class="modal-dialog modal-lg modal-dialog-centered">
      <div class="modal-content">
        <div class="modal-header">
          <h5 class="modal-title">
            {{ isEditingCreateModal ? (createType === 'probe' ? t('monitoring.editProbeModalTitle') : t('monitoring.editCertModalTitle')) : t('monitoring.newTrackerButton') }}
          </h5>
          <button
            type="button"
            :aria-label="t('common.close')"
            class="btn-close"
            :disabled="savingProbe || savingCert"
            @click="closeCreateModal"
          />
        </div>
        <!-- Editing an existing probe or cert is unambiguous (the row
             already told us which), so it keeps the original two
             fully-independent forms. Creating is where a shared "Host /
             Target" actually helps — a probe and a cert for the same
             target would otherwise mean typing the same domain twice. -->
        <template v-if="isEditingCreateModal">
          <MonitoringProbeEditForm
            v-if="createType === 'probe'"
            v-model:probe-form="probeForm"
            :saving="savingProbe"
            :error="probeFormError"
            @save="saveProbe"
            @close="closeCreateModal"
          />
          <MonitoringCertEditForm
            v-else
            v-model:cert-form="certForm"
            :saving="savingCert"
            :error="certFormError"
            @save="saveCert"
            @close="closeCreateModal"
          />
        </template>
        <MonitoringCreateForm
          v-else
          v-model:probe-form="probeForm"
          v-model:cert-form="certForm"
          :saving="savingProbe || savingCert"
          :error="probeFormError || certFormError"
          :is-probe-modal-open="isProbeModalOpen"
          :is-cert-modal-open="isCertModalOpen"
          :save-probe="saveProbe"
          :save-cert="saveCert"
          @close="closeCreateModal"
        />
      </div>
    </div>
  </div>
  <div
    v-if="createModalOpen"
    class="modal-backdrop fade show"
  />
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useModalChrome } from '../../composables/useModalChrome'
import type { ProbeForm } from '../../composables/useUptimeProbes'
import type { CertForm } from '../../composables/useSslCertificates'
import MonitoringProbeEditForm from './MonitoringProbeEditForm.vue'
import MonitoringCertEditForm from './MonitoringCertEditForm.vue'
import MonitoringCreateForm from './MonitoringCreateForm.vue'

// Probe creation/edit and cert creation/edit render through one shared
// modal instead of two independent ones — createType tracks which body is
// showing, driven by whichever of probeModalOpen/certModalOpen the underlying
// composables set, so no new open-state needs to be introduced.
const props = defineProps<{
  probeModalOpen: boolean
  certModalOpen: boolean
  savingProbe: boolean
  savingCert: boolean
  probeFormError: string
  certFormError: string
  // Live getters (see useMonitoringCreateForm): a prop snapshot lags a render.
  isProbeModalOpen: () => boolean
  isCertModalOpen: () => boolean
  saveProbe: () => Promise<void>
  saveCert: () => Promise<void>
  closeProbeModal: () => void
  closeCertModal: () => void
}>()
const probeForm = defineModel<ProbeForm>('probeForm', { required: true })
const certForm = defineModel<CertForm>('certForm', { required: true })

const { t } = useI18n()

const createType = computed<'probe' | 'cert'>(() => (props.probeModalOpen ? 'probe' : 'cert'))
const createModalOpen = computed(() => props.probeModalOpen || props.certModalOpen)
const isEditingCreateModal = computed(() =>
  createType.value === 'probe' ? !!probeForm.value.id : !!certForm.value.id
)

function closeCreateModal(): void {
  props.closeProbeModal()
  props.closeCertModal()
}

const createModalRef = ref<HTMLElement | null>(null)
useModalChrome(createModalRef, () => createModalOpen.value, { onClose: closeCreateModal })
</script>
