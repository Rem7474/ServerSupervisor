<template>
  <div
    class="modal modal-blur fade show"
    style="display: block;"
    tabindex="-1"
    role="dialog"
    aria-modal="true"
    aria-label="Palette de commande"
    @click.self="close"
  >
    <div class="modal-dialog modal-lg modal-dialog-centered">
      <div class="modal-content command-palette">
        <div class="command-palette-input-wrap">
          <IconSearch
            :size="18"
            class="icon text-secondary me-2"
          />
          <input
            ref="inputRef"
            v-model="query"
            type="text"
            class="form-control command-palette-input"
            placeholder="Rechercher une page, un hôte, un conteneur…"
            autocomplete="off"
            spellcheck="false"
          >
          <kbd class="command-palette-esc">Échap</kbd>
        </div>
        <div class="command-palette-results">
          <div
            v-if="!results.length"
            class="text-center text-secondary py-4 small"
          >
            {{ query.trim() ? 'Aucun résultat.' : 'Commencez à taper pour rechercher.' }}
          </div>
          <div
            v-for="group in groupedResults"
            v-else
            :key="group.label"
            class="command-palette-group"
          >
            <div class="command-palette-group-label">
              {{ group.label }}
            </div>
            <button
              v-for="result in group.items"
              :key="result.key"
              type="button"
              class="command-palette-item"
              :class="{ active: result.globalIndex === activeIndex }"
              @mouseenter="activeIndex = result.globalIndex"
              @click="selectResult(result)"
            >
              <component
                :is="result.icon"
                :size="18"
                class="icon me-2 text-secondary flex-shrink-0"
              />
              <span class="command-palette-item-label">{{ result.label }}</span>
              <span
                v-if="result.sublabel"
                class="command-palette-item-sublabel"
              >{{ result.sublabel }}</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
  <div class="modal-backdrop fade show" />
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { IconSearch } from '@tabler/icons-vue'
import { useCommandPalette, type PaletteResult } from '../composables/useCommandPalette'

const { query, activeIndex, results, close, selectResult } = useCommandPalette()

const inputRef = ref<HTMLInputElement | null>(null)

// Mounted fresh on every open (App.vue renders this v-if="isOpen"), so a
// plain onMounted focus is reliable — unlike a permanently-mounted modal
// toggled by an internal v-if, there's no stale-ref timing to work around.
onMounted(() => inputRef.value?.focus())

interface DisplayResult extends PaletteResult {
  globalIndex: number
}

const groupedResults = computed(() => {
  const groups: { label: string; items: DisplayResult[] }[] = []
  results.value.forEach((result, globalIndex) => {
    let group = groups.find((g) => g.label === result.group)
    if (!group) {
      group = { label: result.group, items: [] }
      groups.push(group)
    }
    group.items.push({ ...result, globalIndex })
  })
  return groups
})
</script>

<style scoped>
.command-palette {
  overflow: hidden;
}

.command-palette-input-wrap {
  display: flex;
  align-items: center;
  padding: 0.9rem 1.1rem;
  border-bottom: 1px solid var(--tblr-border-color);
}

.command-palette-input {
  border: 0;
  box-shadow: none;
  padding: 0;
  font-size: 1.05rem;
  background: transparent;
}

.command-palette-input:focus {
  box-shadow: none;
}

.command-palette-esc {
  flex-shrink: 0;
  margin-left: 0.5rem;
}

.command-palette-results {
  max-height: 60vh;
  overflow-y: auto;
  padding: 0.5rem;
}

.command-palette-group + .command-palette-group {
  margin-top: 0.25rem;
}

.command-palette-group-label {
  padding: 0.4rem 0.6rem 0.2rem;
  font-size: 0.7rem;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--tblr-secondary);
}

.command-palette-item {
  display: flex;
  align-items: center;
  width: 100%;
  border: 0;
  background: transparent;
  padding: 0.5rem 0.6rem;
  border-radius: 6px;
  text-align: left;
  color: var(--tblr-body-color);
}

.command-palette-item.active {
  background: rgba(var(--tblr-primary-rgb, 32, 107, 196), 0.1);
}

.command-palette-item-label {
  flex: 1 1 auto;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.command-palette-item-sublabel {
  flex-shrink: 0;
  margin-left: 0.75rem;
  font-size: 0.8rem;
  color: var(--tblr-secondary);
  max-width: 40%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
