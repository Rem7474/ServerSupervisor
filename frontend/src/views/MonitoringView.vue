<template>
  <div>
    <div class="page-header mb-3">
      <div class="d-flex flex-column flex-lg-row align-items-lg-center justify-content-between gap-3">
        <div>
          <h2 class="page-title">
            Monitoring
          </h2>
          <div class="text-muted">
            Sondes HTTP/TCP synthétiques et suivi des certificats SSL/TLS.
          </div>
        </div>
        <button
          v-if="auth.role === 'admin'"
          type="button"
          class="btn btn-primary"
          @click="tab === 'ssl' ? sslPanelRef?.openCreateCert() : uptimePanelRef?.openCreateProbe()"
        >
          {{ tab === 'ssl' ? '+ Ajouter un certificat' : '+ Nouvelle sonde' }}
        </button>
      </div>
    </div>

    <!-- Tabs -->
    <div class="mb-3">
      <ul class="nav nav-tabs">
        <li class="nav-item">
          <button
            type="button"
            :class="['nav-link', tab === 'uptime' ? 'active' : '']"
            @click="setTab('uptime')"
          >
            <IconActivity
              :size="16"
              class="icon icon-sm me-1"
            />
            Sondes uptime
            <span
              v-if="downCount > 0"
              class="badge bg-red text-white ms-1"
            >{{ downCount }}</span>
          </button>
        </li>
        <li class="nav-item">
          <button
            type="button"
            :class="['nav-link', tab === 'ssl' ? 'active' : '']"
            @click="setTab('ssl')"
          >
            <IconLock
              :size="16"
              class="icon icon-sm me-1"
            />
            Certificats SSL
            <span
              v-if="expiringCount > 0"
              class="badge bg-yellow text-white ms-1"
            >{{ expiringCount }}</span>
          </button>
        </li>
      </ul>
    </div>

    <!-- Both panels stay mounted (v-show, not v-if) so their badge counts in
         the tab bar above are always current, not just while their tab is
         active — matching this page's pre-split behavior of loading both
         domains together on mount. Each panel owns its full lifecycle
         (fetch/auto-refresh/CRUD/modals) independently — see useUptimeProbes
         / useSslCertificates. -->
    <UptimeProbesPanel
      v-show="tab === 'uptime'"
      ref="uptimePanelRef"
      @update:down-count="downCount = $event"
    />
    <SslCertificatesPanel
      v-show="tab === 'ssl'"
      ref="sslPanelRef"
      @update:expiring-count="expiringCount = $event"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { IconActivity, IconLock } from '@tabler/icons-vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import UptimeProbesPanel from '../components/monitoring/UptimeProbesPanel.vue'
import SslCertificatesPanel from '../components/monitoring/SslCertificatesPanel.vue'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()

type Tab = 'uptime' | 'ssl'
const tab = ref<Tab>((route.query.tab as Tab) === 'ssl' ? 'ssl' : 'uptime')

function setTab(t: Tab) {
  tab.value = t
  router.replace({ query: t !== 'uptime' ? { tab: t } : {} })
}

watch(() => route.query.tab, (v) => {
  tab.value = (v as Tab) === 'ssl' ? 'ssl' : 'uptime'
})

// Populated by each panel's emit (see the comment above the templates) —
// kept here only so the tab bar's badges don't require either panel to be
// active to show a count.
const downCount = ref(0)
const expiringCount = ref(0)

const uptimePanelRef = ref<InstanceType<typeof UptimeProbesPanel> | null>(null)
const sslPanelRef = ref<InstanceType<typeof SslCertificatesPanel> | null>(null)
</script>
