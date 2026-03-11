<template>
  <div class="card mb-4">
    <div class="card-header">
      <h3 class="card-title">Email (SMTP)</h3>
    </div>
    <div class="card-body">
      <div class="row g-3">
        <div class="col-md-8">
          <label class="form-label">Hote SMTP</label>
          <input type="text" class="form-control" v-model="form.smtpHost" placeholder="mail.example.com">
        </div>
        <div class="col-md-4">
          <label class="form-label">Port</label>
          <input type="number" class="form-control" v-model.number="form.smtpPort" placeholder="587">
        </div>
        <div class="col-md-6">
          <label class="form-label">Utilisateur</label>
          <input type="text" class="form-control" v-model="form.smtpUser" placeholder="user@example.com" autocomplete="off">
        </div>
        <div class="col-md-6">
          <label class="form-label">Mot de passe</label>
          <div class="input-group">
            <input :type="showSmtpPass ? 'text' : 'password'" class="form-control" v-model="form.smtpPass" autocomplete="new-password">
            <button class="btn btn-outline-secondary" type="button" @click="$emit('update:show-smtp-pass', !showSmtpPass)">
              {{ showSmtpPass ? 'Masquer' : 'Afficher' }}
            </button>
          </div>
        </div>
        <div class="col-md-6">
          <label class="form-label">Expediteur (From)</label>
          <input type="email" class="form-control" v-model="form.smtpFrom" placeholder="no-reply@example.com">
        </div>
        <div class="col-md-6">
          <label class="form-label">Destinataire (To)</label>
          <input type="email" class="form-control" v-model="form.smtpTo" placeholder="admin@example.com">
        </div>
        <div class="col-12">
          <label class="form-check">
            <input type="checkbox" class="form-check-input" v-model="form.smtpTls">
            <span class="form-check-label">TLS / STARTTLS active</span>
          </label>
        </div>
      </div>
    </div>
    <div class="card-footer d-flex align-items-center gap-2">
      <button
        v-if="authIsAdmin"
        class="btn btn-primary"
        @click="$emit('save')"
        :disabled="savingSmtp"
      >
        {{ savingSmtp ? 'Enregistrement...' : 'Enregistrer SMTP' }}
      </button>
      <button
        class="btn btn-outline-secondary"
        @click="$emit('test')"
        :disabled="testingSmtp || !form.smtpHost"
      >
        {{ testingSmtp ? 'Test en cours...' : 'Tester la connexion' }}
      </button>
      <span v-if="smtpSaveMsg" :class="['ms-auto small', smtpSaveOk ? 'text-success' : 'text-danger']">
        {{ smtpSaveMsg }}
      </span>
      <span v-if="smtpTestMessage" :class="['ms-auto small', smtpTestSuccess ? 'text-success' : 'text-danger']">
        {{ smtpTestMessage }}
      </span>
    </div>
  </div>
</template>

<script setup>
defineProps({
  form: {
    type: Object,
    required: true,
  },
  authIsAdmin: {
    type: Boolean,
    default: false,
  },
  showSmtpPass: {
    type: Boolean,
    default: false,
  },
  savingSmtp: {
    type: Boolean,
    default: false,
  },
  smtpSaveMsg: {
    type: String,
    default: '',
  },
  smtpSaveOk: {
    type: Boolean,
    default: false,
  },
  testingSmtp: {
    type: Boolean,
    default: false,
  },
  smtpTestMessage: {
    type: String,
    default: '',
  },
  smtpTestSuccess: {
    type: Boolean,
    default: false,
  },
})

defineEmits(['save', 'test', 'update:show-smtp-pass'])
</script>
