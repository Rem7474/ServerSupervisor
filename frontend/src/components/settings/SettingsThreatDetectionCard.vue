<template>
  <div class="card h-100">
    <div class="card-header">
      <h3 class="card-title">
        Score de menace (IPs suspectes)
      </h3>
    </div>
    <div class="card-body">
      <p class="text-secondary small">
        Pondérations utilisées pour calculer le score et le niveau (LOW/MEDIUM/HIGH/CRITICAL)
        des IP suspectes sur la page Menaces. Le score combine la catégorie de motif détecté,
        le code de réponse HTTP et le nombre de chemins distincts ciblés — un volume élevé de
        requêtes toutes en 2xx reste peu suspect, alors qu'un faible volume en 404/5xx ou
        réparti sur de nombreux chemins fait grimper le score rapidement.
      </p>

      <h4 class="mb-2">
        Poids par catégorie
      </h4>
      <div class="row g-2 mb-3">
        <div class="col-6 col-md-4">
          <label
            for="threat-weight-wordpress"
            class="form-label"
          >WordPress</label>
          <input
            id="threat-weight-wordpress"
            v-model.number="form.threatWeightWordpress"
            type="number"
            step="0.1"
            min="0"
            class="form-control"
          >
        </div>
        <div class="col-6 col-md-4">
          <label
            for="threat-weight-adminpanel"
            class="form-label"
          >Panneau admin</label>
          <input
            id="threat-weight-adminpanel"
            v-model.number="form.threatWeightAdminpanel"
            type="number"
            step="0.1"
            min="0"
            class="form-control"
          >
        </div>
        <div class="col-6 col-md-4">
          <label
            for="threat-weight-pathtraversal"
            class="form-label"
          >Traversée de chemin</label>
          <input
            id="threat-weight-pathtraversal"
            v-model.number="form.threatWeightPathtraversal"
            type="number"
            step="0.1"
            min="0"
            class="form-control"
          >
        </div>
        <div class="col-6 col-md-4">
          <label
            for="threat-weight-knownscanner"
            class="form-label"
          >Scanner connu</label>
          <input
            id="threat-weight-knownscanner"
            v-model.number="form.threatWeightKnownscanner"
            type="number"
            step="0.1"
            min="0"
            class="form-control"
          >
        </div>
        <div class="col-6 col-md-4">
          <label
            for="threat-weight-suspiciousmethod"
            class="form-label"
          >Méthode suspecte</label>
          <input
            id="threat-weight-suspiciousmethod"
            v-model.number="form.threatWeightSuspiciousmethod"
            type="number"
            step="0.1"
            min="0"
            class="form-control"
          >
        </div>
      </div>

      <h4 class="mb-2">
        Multiplicateurs par code de réponse
      </h4>
      <div class="row g-2 mb-3">
        <div class="col-6 col-md-4">
          <label
            for="threat-weight-status-2xx"
            class="form-label"
          >2xx (succès)</label>
          <input
            id="threat-weight-status-2xx"
            v-model.number="form.threatWeightStatus2xx"
            type="number"
            step="0.05"
            min="0"
            class="form-control"
          >
        </div>
        <div class="col-6 col-md-4">
          <label
            for="threat-weight-status-3xx"
            class="form-label"
          >3xx (redirection)</label>
          <input
            id="threat-weight-status-3xx"
            v-model.number="form.threatWeightStatus3xx"
            type="number"
            step="0.1"
            min="0"
            class="form-control"
          >
        </div>
        <div class="col-6 col-md-4">
          <label
            for="threat-weight-status-404"
            class="form-label"
          >404</label>
          <input
            id="threat-weight-status-404"
            v-model.number="form.threatWeightStatus404"
            type="number"
            step="0.1"
            min="0"
            class="form-control"
          >
        </div>
        <div class="col-6 col-md-4">
          <label
            for="threat-weight-status-4xx"
            class="form-label"
          >4xx (autre)</label>
          <input
            id="threat-weight-status-4xx"
            v-model.number="form.threatWeightStatus4xx"
            type="number"
            step="0.1"
            min="0"
            class="form-control"
          >
        </div>
        <div class="col-6 col-md-4">
          <label
            for="threat-weight-status-5xx"
            class="form-label"
          >5xx</label>
          <input
            id="threat-weight-status-5xx"
            v-model.number="form.threatWeightStatus5xx"
            type="number"
            step="0.1"
            min="0"
            class="form-control"
          >
        </div>
      </div>

      <h4 class="mb-2">
        Pondération structurelle
      </h4>
      <div class="row g-2 mb-3">
        <div class="col-6 col-md-4">
          <label
            for="threat-weight-breadth"
            class="form-label"
          >Largeur (chemins distincts)</label>
          <input
            id="threat-weight-breadth"
            v-model.number="form.threatWeightBreadth"
            type="number"
            step="0.1"
            min="0"
            class="form-control"
            aria-describedby="threat-breadth-hint"
          >
          <div
            id="threat-breadth-hint"
            class="form-hint"
          >
            Poids par chemin distinct scanné par l'IP
          </div>
        </div>
        <div class="col-6 col-md-4">
          <label
            for="threat-weight-hits"
            class="form-label"
          >Volume de requêtes</label>
          <input
            id="threat-weight-hits"
            v-model.number="form.threatWeightHits"
            type="number"
            step="0.1"
            min="0"
            class="form-control"
            aria-describedby="threat-hits-hint"
          >
          <div
            id="threat-hits-hint"
            class="form-hint"
          >
            Poids du volume (atténué en ln(hits+1), pas linéaire)
          </div>
        </div>
      </div>

      <h4 class="mb-2">
        Seuils de niveau
      </h4>
      <div class="row g-2">
        <div class="col-6 col-md-4">
          <label
            for="threat-threshold-medium"
            class="form-label"
          >MEDIUM à partir de</label>
          <input
            id="threat-threshold-medium"
            v-model.number="form.threatThresholdMedium"
            type="number"
            step="1"
            min="0"
            class="form-control"
          >
        </div>
        <div class="col-6 col-md-4">
          <label
            for="threat-threshold-high"
            class="form-label"
          >HIGH à partir de</label>
          <input
            id="threat-threshold-high"
            v-model.number="form.threatThresholdHigh"
            type="number"
            step="1"
            min="0"
            class="form-control"
          >
        </div>
        <div class="col-6 col-md-4">
          <label
            for="threat-threshold-critical"
            class="form-label"
          >CRITICAL à partir de</label>
          <input
            id="threat-threshold-critical"
            v-model.number="form.threatThresholdCritical"
            type="number"
            step="1"
            min="0"
            class="form-control"
          >
        </div>
      </div>
      <div
        v-if="thresholdOrderWarning"
        class="alert alert-warning mt-3 mb-0 py-2"
      >
        {{ thresholdOrderWarning }}
      </div>
    </div>
    <div class="card-footer d-flex align-items-center gap-2">
      <button
        v-if="authIsAdmin"
        type="button"
        class="btn btn-primary"
        :disabled="savingThreatDetection"
        @click="$emit('save')"
      >
        {{ savingThreatDetection ? 'Enregistrement...' : 'Enregistrer' }}
      </button>
      <span
        v-if="threatDetectionSaveMsg"
        :class="['ms-auto small', threatDetectionSaveOk ? 'text-success' : 'text-danger']"
      >
        {{ threatDetectionSaveMsg }}
      </span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

interface ThreatDetectionForm {
  threatWeightWordpress: number
  threatWeightAdminpanel: number
  threatWeightPathtraversal: number
  threatWeightKnownscanner: number
  threatWeightSuspiciousmethod: number
  threatWeightStatus2xx: number
  threatWeightStatus3xx: number
  threatWeightStatus404: number
  threatWeightStatus4xx: number
  threatWeightStatus5xx: number
  threatWeightBreadth: number
  threatWeightHits: number
  threatThresholdMedium: number
  threatThresholdHigh: number
  threatThresholdCritical: number
}

const props = withDefaults(defineProps<{
  form: ThreatDetectionForm
  authIsAdmin?: boolean
  savingThreatDetection?: boolean
  threatDetectionSaveMsg?: string
  threatDetectionSaveOk?: boolean
}>(), {
  authIsAdmin: false,
  savingThreatDetection: false,
  threatDetectionSaveMsg: '',
  threatDetectionSaveOk: false,
})

defineEmits<{
  (e: 'save'): void
}>()

const thresholdOrderWarning = computed(() => {
  const { threatThresholdMedium: m, threatThresholdHigh: h, threatThresholdCritical: c } = props.form
  if (m < h && h < c) return ''
  return 'Les seuils devraient être croissants (MEDIUM < HIGH < CRITICAL), sinon le niveau affiché peut être incohérent.'
})
</script>
