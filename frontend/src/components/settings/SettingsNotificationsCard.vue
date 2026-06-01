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
          aria-describedby="ntfy-hint"
        >
        <div
          id="ntfy-hint"
          class="form-hint"
          style="display:none;"
        >
          URL ntfy.sh pour les notifications
        </div>
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
            aria-describedby="github-token-hint"
          >
          <button
            class="btn btn-outline-secondary"
            type="button"
            @click="$emit('update:show-github-token', !showGitHubToken)"
          >
            {{ showGitHubToken ? 'Masquer' : 'Afficher' }}
          </button>
        </div>
        <div
          id="github-token-hint"
          class="form-hint"
        >
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

<script setup lang="ts">
interface NotifForm {
  ntfyUrl: string
  githubToken: string
}

withDefaults(defineProps<{
  form: NotifForm
  authIsAdmin?: boolean
  showGitHubToken?: boolean
  savingNotif?: boolean
  notifSaveMsg?: string
  notifSaveOk?: boolean
  testingNtfy?: boolean
  ntfyTestMessage?: string
  ntfyTestSuccess?: boolean
}>(), {
  authIsAdmin: false,
  showGitHubToken: false,
  savingNotif: false,
  notifSaveMsg: '',
  notifSaveOk: false,
  testingNtfy: false,
  ntfyTestMessage: '',
  ntfyTestSuccess: false,
})

defineEmits<{
  (e: 'save'): void
  (e: 'test'): void
  (e: 'update:show-github-token', value: boolean): void
}>()
</script>
