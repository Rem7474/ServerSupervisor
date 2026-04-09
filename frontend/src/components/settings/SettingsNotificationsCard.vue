<template>
  <div class="card h-100">
    <div class="card-header">
      <h3 class="card-title">
        Notifications
      </h3>
    </div>
    <div class="card-body">
      <div class="mb-3">
        <label class="form-label">URL ntfy.sh</label>
        <input
          v-model="form.ntfyUrl"
          type="text"
          class="form-control"
          placeholder="https://ntfy.sh/mon-topic"
        >
      </div>
      <div class="mb-0">
        <label class="form-label">GitHub Token</label>
        <div class="input-group">
          <input
            v-model="form.githubToken"
            :type="showGitHubToken ? 'text' : 'password'"
            class="form-control"
            placeholder="ghp_..."
            autocomplete="new-password"
          >
          <button
            class="btn btn-outline-secondary"
            type="button"
            @click="$emit('update:show-github-token', !showGitHubToken)"
          >
            {{ showGitHubToken ? 'Masquer' : 'Afficher' }}
          </button>
        </div>
        <div class="form-hint">
          Pour le suivi des releases GitHub
        </div>
      </div>
    </div>
    <div class="card-footer d-flex align-items-center gap-2">
      <button
        v-if="authIsAdmin"
        class="btn btn-primary"
        :disabled="savingNotif"
        @click="$emit('save')"
      >
        {{ savingNotif ? 'Enregistrement...' : 'Enregistrer' }}
      </button>
      <button
        class="btn btn-outline-secondary"
        :disabled="testingNtfy || !form.ntfyUrl"
        @click="$emit('test')"
      >
        {{ testingNtfy ? 'Test...' : 'Tester ntfy' }}
      </button>
      <span
        v-if="notifSaveMsg"
        :class="['ms-auto small', notifSaveOk ? 'text-success' : 'text-danger']"
      >
        {{ notifSaveMsg }}
      </span>
      <span
        v-if="ntfyTestMessage"
        :class="['ms-auto small', ntfyTestSuccess ? 'text-success' : 'text-danger']"
      >
        {{ ntfyTestMessage }}
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
  showGitHubToken: {
    type: Boolean,
    default: false,
  },
  savingNotif: {
    type: Boolean,
    default: false,
  },
  notifSaveMsg: {
    type: String,
    default: '',
  },
  notifSaveOk: {
    type: Boolean,
    default: false,
  },
  testingNtfy: {
    type: Boolean,
    default: false,
  },
  ntfyTestMessage: {
    type: String,
    default: '',
  },
  ntfyTestSuccess: {
    type: Boolean,
    default: false,
  },
})

defineEmits(['save', 'test', 'update:show-github-token'])
</script>
