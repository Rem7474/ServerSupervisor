<template>
  <div class="card">
    <div
      class="card-header"
      style="cursor:pointer"
      @click="isOpen = !isOpen"
    >
      <h3 class="card-title d-flex align-items-center gap-2">
        Versions &amp; Mises à jour Docker
        <span v-if="outdatedCount > 0" class="badge bg-yellow-lt text-yellow">{{ outdatedCount }} en retard</span>
        <svg class="ms-auto" width="16" height="16" fill="none" stroke="currentColor" viewBox="0 0 24 24"
             :style="isOpen ? 'transform:rotate(180deg)' : ''" style="transition:transform .2s;flex-shrink:0">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"/>
        </svg>
      </h3>
      <div class="card-options text-secondary small">Suivi via <router-link to="/git-webhooks" @click.stop>Git / Automatisation</router-link></div>
    </div>
    <div v-show="isOpen" class="table-responsive">
      <table class="table table-vcenter card-table">
        <thead>
          <tr>
            <th>Image</th>
            <th>Hôte</th>
            <th>Conteneurs</th>
            <th>En cours</th>
            <th>Dernière version</th>
            <th>Statut</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="v in versions" :key="v.docker_image + v.host_id">
            <td class="fw-semibold">{{ v.docker_image }}</td>
            <td class="text-secondary">{{ v.hostname }}</td>
            <td>
              <span v-if="v.container_count > 0" class="badge bg-azure-lt text-azure" :title="`${v.container_count} conteneur${v.container_count > 1 ? 's' : ''} utilisent cette image`">{{ v.container_count }}</span>
              <span v-else class="text-secondary small">—</span>
            </td>
            <td><code v-if="v.running_version">{{ v.running_version }}</code><span v-else class="text-secondary small">inconnue</span></td>
            <td>
              <a v-if="v.release_url" :href="v.release_url" target="_blank" class="link-primary">{{ v.latest_version }}</a>
              <span v-else>{{ v.latest_version }}</span>
            </td>
            <td>
              <span v-if="v.is_up_to_date" class="badge bg-green-lt text-green">À jour</span>
              <span v-else-if="v.running_version || v.update_confirmed" class="badge bg-yellow-lt text-yellow">Mise à jour disponible</span>
              <span v-else class="badge bg-secondary-lt text-secondary">Version inconnue</span>
            </td>
          </tr>
          <tr v-if="versions.length === 0">
            <td colspan="6" class="text-center text-secondary py-4">
              Aucun suivi de version configuré. Ajoutez des release trackers dans
              <router-link to="/git-webhooks">Git / Automatisation</router-link>.
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup>
import { defineProps, ref, computed } from 'vue'

const props = defineProps({
  versions: { type: Array, default: () => [] },
})

const isOpen = ref(false)

const outdatedCount = computed(() =>
  props.versions.filter(v => !v.is_up_to_date && (v.running_version || v.update_confirmed)).length
)
</script>
