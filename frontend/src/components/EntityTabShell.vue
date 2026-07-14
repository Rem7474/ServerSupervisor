<template>
  <div>
    <div :class="{ 'card-header': cardHeader }">
      <ul
        class="nav nav-tabs"
        :class="[cardHeader ? 'card-header-tabs' : 'mb-3', navClass]"
      >
        <li
          v-for="t in tabs"
          :key="t.key"
          class="nav-item"
        >
          <button
            type="button"
            class="nav-link"
            :class="{ active: modelValue === t.key }"
            @click="emit('update:modelValue', t.key)"
          >
            {{ t.label }}
            <span
              v-for="(b, i) in t.badges"
              :key="i"
              :class="b.badgeClass || 'badge bg-secondary-lt text-secondary ms-1'"
            >{{ b.value }}</span>
          </button>
        </li>
      </ul>
    </div>
    <div
      v-for="t in tabs"
      v-show="modelValue === t.key"
      :key="t.key"
    >
      <slot
        v-if="!t.lazy || visited.has(t.key)"
        :name="t.key"
        :active="modelValue === t.key"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'

export interface EntityTabBadge {
  value: number | string
  badgeClass?: string
}

export interface EntityTab {
  key: string
  label: string
  // A tab can carry more than one badge (e.g. a total count plus a
  // conditional warning count) — pass an empty/omitted array for none.
  badges?: EntityTabBadge[]
  // Mount this tab's slot content on first activation and keep it mounted
  // afterward (v-show toggling only), instead of mounting eagerly with the
  // rest of the shell. Off by default so a plain tab list behaves like a
  // regular Bootstrap nav; opt in per-tab when content is expensive to
  // fetch/render (per-guest panels, live status polling, ...).
  lazy?: boolean
}

const props = defineProps<{
  modelValue: string
  tabs: EntityTab[]
  navClass?: string
  // Wrap the nav strip in Tabler's .card-header (for a shell embedded inside
  // a .card, tab content following as the card's body) instead of a bare
  // nav-tabs strip with its own bottom margin (for a shell placed directly
  // in the page, each tab bringing its own card(s) below).
  cardHeader?: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

// Tracks which lazy tabs have been activated at least once. Watches the prop
// (rather than only the click handler above) so a tab switch driven from
// outside the shell — e.g. a view restoring the active tab from a ?tab=
// query-string param on load — still marks that tab visited.
const visited = ref(new Set<string>())
watch(() => props.modelValue, (key) => visited.value.add(key), { immediate: true })
</script>
