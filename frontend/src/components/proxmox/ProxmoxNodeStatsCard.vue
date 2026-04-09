<template>
  <div class="card mb-4">
    <div class="card-body position-relative">
      <div class="row g-4 align-items-start">
        <!-- CPU -->
        <div class="col-6 col-sm-4 col-lg">
          <div class="subheader mb-1">
            CPU
          </div>
          <div class="h3 mb-1">
            {{ (node.cpu_usage * 100).toFixed(1) }}%
          </div>
          <div class="progress progress-xs mb-1">
            <div
              class="progress-bar"
              :class="cpuColor(node.cpu_usage)"
              :style="`width:${(node.cpu_usage*100).toFixed(1)}%`"
            />
          </div>
          <div class="text-muted small">
            {{ node.cpu_count }} cœurs
          </div>
        </div>

        <!-- RAM -->
        <div class="col-6 col-sm-4 col-lg">
          <div class="subheader mb-1">
            RAM
          </div>
          <div class="h3 mb-1">
            {{ formatBytes(node.mem_used) }}
          </div>
          <div class="progress progress-xs mb-1">
            <div
              class="progress-bar"
              :class="ramColor(node.mem_used, node.mem_total)"
              :style="`width:${memPct(node)}%`"
            />
          </div>
          <div class="text-muted small">
            / {{ formatBytes(node.mem_total) }}
          </div>
        </div>

        <!-- Uptime -->
        <div class="col-6 col-sm-4 col-lg">
          <div class="subheader mb-1">
            Uptime
          </div>
          <div class="h3 mb-0">
            {{ formatUptime(node.uptime) }}
          </div>
        </div>

        <!-- Guests -->
        <div class="col-6 col-sm-4 col-lg">
          <div class="subheader mb-1">
            Guests
          </div>
          <div class="h3 mb-0">
            <span class="text-primary">{{ node.vm_count }}</span><span class="text-muted fs-5 ms-1">VM</span>
            <span class="ms-2 text-info">{{ node.lxc_count }}</span><span class="text-muted fs-5 ms-1">LXC</span>
          </div>
        </div>

        <!-- Live data separator -->
        <template v-if="liveStatus">
          <div class="col-auto d-none d-lg-flex align-items-stretch py-1">
            <div class="vr" />
          </div>

          <!-- IO Wait -->
          <div class="col-6 col-sm-4 col-lg">
            <div class="subheader mb-1">
              IO Wait
            </div>
            <div
              class="h3 mb-0"
              :class="liveStatus.wait > 0.2 ? 'text-danger' : liveStatus.wait > 0.05 ? 'text-warning' : 'text-success'"
            >
              {{ (liveStatus.wait * 100).toFixed(2) }}%
            </div>
            <div class="text-muted small">
              disque
            </div>
          </div>

          <!-- Swap -->
          <div class="col-6 col-sm-4 col-lg">
            <div class="subheader mb-1">
              Swap
            </div>
            <div class="h3 mb-1">
              {{ formatBytes(liveStatus.swap.used) }}
            </div>
            <div
              v-if="liveStatus.swap.total"
              class="progress progress-xs mb-1"
            >
              <div
                class="progress-bar"
                :class="ramColor(liveStatus.swap.used, liveStatus.swap.total)"
                :style="`width:${(liveStatus.swap.used/liveStatus.swap.total*100).toFixed(1)}%`"
              />
            </div>
            <div class="text-muted small">
              / {{ formatBytes(liveStatus.swap.total) }}
            </div>
          </div>

          <!-- Rootfs -->
          <div class="col-6 col-sm-4 col-lg">
            <div class="subheader mb-1">
              Rootfs
            </div>
            <div class="h3 mb-1">
              {{ formatBytes(liveStatus.rootfs.used) }}
            </div>
            <div class="progress progress-xs mb-1">
              <div
                class="progress-bar"
                :class="storageColor(liveStatus.rootfs.used, liveStatus.rootfs.total)"
                :style="`width:${(liveStatus.rootfs.used/liveStatus.rootfs.total*100).toFixed(1)}%`"
              />
            </div>
            <div class="text-muted small">
              / {{ formatBytes(liveStatus.rootfs.total) }}
            </div>
          </div>
        </template>

        <!-- Live loading placeholder -->
        <div
          v-else-if="liveStatusLoading"
          class="col align-self-center text-muted small"
        >
          <span class="spinner-border spinner-border-sm me-1" />Chargement…
        </div>
      </div>

      <!-- Live refresh timestamp + error (absolute, no added height) -->
      <div class="position-absolute bottom-0 end-0 pb-2 pe-3 d-flex align-items-center gap-2">
        <span
          v-if="liveStatusError"
          class="text-danger"
          style="font-size:0.7rem"
        >{{ liveStatusError }}</span>
        <span
          v-if="liveStatus"
          class="text-muted"
          style="font-size:0.7rem"
        >
          <span
            v-if="liveStatusLoading"
            class="spinner-border me-1"
            style="width:.65rem;height:.65rem;border-width:.1em"
          />
          Actualisé à {{ liveStatusTime }}
        </span>
      </div>
    </div>
  </div>
</template>

<script setup>
import { defineProps } from 'vue'
import { formatBytes, formatUptime } from '../utils/formatters'

const props = defineProps({
  node: { type: Object, required: true },
  liveStatus: { type: Object, default: null },
  liveStatusLoading: { type: Boolean, default: false },
  liveStatusError: { type: String, default: '' },
  liveStatusTime: { type: String, default: '' },
})

function cpuColor(val) {
  if (val >= 0.8) return 'bg-danger'
  if (val >= 0.5) return 'bg-warning'
  return 'bg-success'
}

function ramColor(used, total) {
  const pct = used / total
  if (pct >= 0.8) return 'bg-danger'
  if (pct >= 0.5) return 'bg-warning'
  return 'bg-success'
}

function storageColor(used, total) {
  const pct = used / total
  if (pct >= 0.9) return 'bg-danger'
  if (pct >= 0.7) return 'bg-warning'
  return 'bg-success'
}

function memPct(node) {
  return node.mem_total ? ((node.mem_used / node.mem_total) * 100).toFixed(1) : 0
}
</script>
