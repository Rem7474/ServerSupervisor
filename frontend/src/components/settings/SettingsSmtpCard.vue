<template>
  <div class="card mb-4">
    <div class="card-header">
      <h3 class="card-title">
        Email (SMTP)
      </h3>
    </div>
    <div class="card-body">
      <div class="row g-3">
        <div class="col-md-8">
          <label class="form-label">Hote SMTP</label>
          <input
            v-model="form.smtpHost"
            type="text"
            class="form-control"
            placeholder="mail.example.com"
          >
        </div>
        <div class="col-md-4">
          <label class="form-label">Port</label>
          <input
            v-model.number="form.smtpPort"
            type="number"
            class="form-control"
            placeholder="587"
          >
        </div>
        <div class="col-md-6">
          <label class="form-label">Utilisateur</label>
          <input
            v-model="form.smtpUser"
            type="text"
            class="form-control"
            placeholder="user@example.com"
            autocomplete="off"
          >
        </div>
        <div class="col-md-6">
          <label class="form-label">Mot de passe</label>
          <div class="input-group">
            <input
              v-model="form.smtpPass"
              :type="showSmtpPass ? 'text' : 'password'"
              class="form-control"
              autocomplete="new-password"
            >
            <button
              class="btn btn-outline-secondary"
              type="button"
              @click="$emit('update:show-smtp-pass', !showSmtpPass)"
            >
              {{ showSmtpPass ? 'Masquer' : 'Afficher' }}
            </button>
          </div>
        </div>
        <div class="col-md-6">
          <label class="form-label">Expediteur (From)</label>
          <input
            v-model="form.smtpFrom"
            type="email"
            class="form-control"
            placeholder="no-reply@example.com"
          >
        </div>
        <div class="col-md-6">
          <label class="form-label">Destinataire (To)</label>
          <input
            v-model="form.smtpTo"
            type="email"
            class="form-control"
            placeholder="admin@example.com"
          >
        </div>
        <div class="col-12">
          <label class="form-check">
            <input
              v-model="form.smtpTls"
              type="checkbox"
              class="form-check-input"
            >
            <span class="form-check-label">TLS / STARTTLS active</span>
          </label>
        </div>
      </div>
    </div>
    <div class="card-footer d-flex align-items-center gap-2">
      <button
        v-if="authIsAdmin"
        type="button"
        class="btn btn-primary"
        :disabled="savingSmtp"
        @click="$emit('save')"
      >
        {{ savingSmtp ? 'Enregistrement...' : 'Enregistrer SMTP' }}
      </button>
      <button
        type="button"
        class="btn btn-outline-secondary"
        :disabled="testingSmtp || !form.smtpHost"
        @click="$emit('test')"
      >
        {{ testingSmtp ? 'Test en cours...' : 'Tester la connexion' }}
      </button>
      <span
        v-if="smtpSaveMsg"
        :class="['ms-auto small', smtpSaveOk ? 'text-success' : 'text-danger']"
      >
        {{ smtpSaveMsg }}
      </span>
      <span
        v-if="smtpTestMessage"
        :class="['ms-auto small', smtpTestSuccess ? 'text-success' : 'text-danger']"
      >
        {{ smtpTestMessage }}
      </span>
    </div>
  </div>
</template>

<script setup lang="ts">
interface SmtpForm {
  smtpHost: string
  smtpPort: number
  smtpUser: string
  smtpPass: string
  smtpFrom: string
  smtpTo: string
  smtpTls: boolean
}

withDefaults(defineProps<{
  form: SmtpForm
  authIsAdmin?: boolean
  showSmtpPass?: boolean
  savingSmtp?: boolean
  smtpSaveMsg?: string
  smtpSaveOk?: boolean
  testingSmtp?: boolean
  smtpTestMessage?: string
  smtpTestSuccess?: boolean
}>(), {
  authIsAdmin: false,
  showSmtpPass: false,
  savingSmtp: false,
  smtpSaveMsg: '',
  smtpSaveOk: false,
  testingSmtp: false,
  smtpTestMessage: '',
  smtpTestSuccess: false,
})

defineEmits<{
  (e: 'save'): void
  (e: 'test'): void
  (e: 'update:show-smtp-pass', value: boolean): void
}>()
</script>
