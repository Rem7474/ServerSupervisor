<template>
  <svg
    :class="cssClass"
    :width="size"
    :height="size"
    fill="none"
    stroke="currentColor"
    viewBox="0 0 24 24"
  >
    <template
      v-for="(el, idx) in resolved.elements"
      :key="`${name}-${idx}`"
    >
      <component
        :is="el.tag"
        v-bind="el.attrs"
      />
    </template>
  </svg>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  name: { type: String, required: true },
  size: { type: [String, Number], default: 24 },
  cssClass: { type: String, default: 'icon' },
})

const ICONS = {
  brand: {
    elements: [
      { tag: 'path', attrs: { 'stroke-linecap': 'round', 'stroke-linejoin': 'round', 'stroke-width': '2', d: 'M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01' } },
    ],
  },
  dashboard: {
    elements: [
      { tag: 'path', attrs: { 'stroke-linecap': 'round', 'stroke-linejoin': 'round', 'stroke-width': '2', d: 'M4 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2V6zM14 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2V6zM4 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2v-2zM14 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2v-2z' } },
    ],
  },
  docker: {
    elements: [
      { tag: 'path', attrs: { 'stroke-linecap': 'round', 'stroke-linejoin': 'round', 'stroke-width': '2', d: 'M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4' } },
    ],
  },
  updates: {
    elements: [
      { tag: 'path', attrs: { 'stroke-linecap': 'round', 'stroke-linejoin': 'round', 'stroke-width': '2', d: 'M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15' } },
    ],
  },
  proxmox: {
    elements: [
      { tag: 'rect', attrs: { x: '2', y: '3', width: '20', height: '14', rx: '2', 'stroke-width': '2' } },
      { tag: 'path', attrs: { 'stroke-linecap': 'round', 'stroke-linejoin': 'round', 'stroke-width': '2', d: 'M8 21h8M12 17v4' } },
      { tag: 'path', attrs: { 'stroke-linecap': 'round', 'stroke-linejoin': 'round', 'stroke-width': '2', d: 'M7 8h2v3H7zM11 8h2v3h-2zM15 8h2v3h-2z' } },
    ],
  },
  alerts: {
    elements: [
      { tag: 'path', attrs: { 'stroke-linecap': 'round', 'stroke-linejoin': 'round', 'stroke-width': '2', d: 'M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9' } },
    ],
  },
  more: {
    elements: [
      { tag: 'circle', attrs: { cx: '5', cy: '12', r: '1.5', fill: 'currentColor' } },
      { tag: 'circle', attrs: { cx: '12', cy: '12', r: '1.5', fill: 'currentColor' } },
      { tag: 'circle', attrs: { cx: '19', cy: '12', r: '1.5', fill: 'currentColor' } },
    ],
  },
  admin: {
    elements: [
      { tag: 'path', attrs: { 'stroke-linecap': 'round', 'stroke-linejoin': 'round', 'stroke-width': '2', d: 'M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z' } },
    ],
  },
  warning: {
    elements: [
      { tag: 'path', attrs: { 'stroke-linecap': 'round', 'stroke-linejoin': 'round', 'stroke-width': '2', d: 'M12 9v2m0 4h.01M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z' } },
    ],
  },
}

const resolved = computed(() => ICONS[props.name] || ICONS.warning)
</script>
