<template>
  <div
    ref="dialogRef"
    class="modal modal-blur fade show"
    style="display: block;"
    tabindex="-1"
    role="dialog"
    aria-modal="true"
    :aria-label="t('common.commandPaletteAriaLabel')"
    @click.self="close"
    @keydown.tab="trapFocus"
  >
    <div class="modal-dialog modal-lg modal-dialog-centered">
      <div class="modal-content command-palette">
        <div class="command-palette-input-wrap">
          <IconSearch
            :size="16"
            class="icon text-secondary me-2"
          />
          <input
            ref="inputRef"
            v-model="query"
            type="text"
            class="form-control command-palette-input"
            :placeholder="t('common.commandPaletteSearchPlaceholder')"
            autocomplete="off"
            spellcheck="false"
          >
          <kbd class="command-palette-esc">{{ t('common.commandPaletteEscLabel') }}</kbd>
        </div>
        <div class="command-palette-results">
          <EmptyState
            v-if="!results.length"
            :title="query.trim() ? t('common.commandPaletteNoResultsTitle') : t('common.commandPaletteStartTypingTitle')"
          />
          <div
            v-for="group in groupedResults"
            v-else
            :key="group.key"
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
                :size="16"
                class="icon me-2 text-secondary flex-shrink-0"
              />
              <span class="command-palette-item-label">
                <template
                  v-for="(part, i) in highlightParts(result.label, query)"
                  :key="i"
                ><mark
                  v-if="part.matched"
                  class="command-palette-match"
                >{{ part.text }}</mark><template v-else>{{ part.text }}</template></template>
              </span>
              <span
                v-if="result.sublabel"
                class="command-palette-item-sublabel"
              >
                <template
                  v-for="(part, i) in highlightParts(result.sublabel, query)"
                  :key="i"
                ><mark
                  v-if="part.matched"
                  class="command-palette-match"
                >{{ part.text }}</mark><template v-else>{{ part.text }}</template></template>
              </span>
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
import { useI18n } from 'vue-i18n'
import { IconSearch } from '@tabler/icons-vue'
import { useCommandPalette, PALETTE_GROUP_LABEL_KEYS, type PaletteResult } from '../composables/useCommandPalette'
import { useModalChrome } from '../composables/useModalChrome'
import { highlightParts } from '../utils/highlightMatch'
import EmptyState from './EmptyState.vue'

const { t } = useI18n()
const { query, activeIndex, results, isOpen, close, selectResult } = useCommandPalette()

const inputRef = ref<HTMLInputElement | null>(null)
const dialogRef = ref<HTMLElement | null>(null)

// Only the scroll lock is missing here — CommandPalette already has its own
// working ESC (a global window listener in useCommandPalette.ts, shared with
// the Ctrl/Cmd+K open shortcut) and its own Tab trap (trapFocus below), both
// correct because this component is mounted fresh per open rather than
// staying mounted and toggling an inner v-if like every other modal in the
// app. Reusing useModalChrome's ESC/focus-trap here would double-fire
// alongside those, so both are switched off.
useModalChrome(dialogRef, () => isOpen.value, { closeOnEsc: false, trapFocus: false })

// Mounted fresh on every open (App.vue renders this v-if="isOpen"), so a
// plain onMounted focus is reliable — unlike a permanently-mounted modal
// toggled by an internal v-if, there's no stale-ref timing to work around.
onMounted(() => inputRef.value?.focus())

// Keeps Tab/Shift+Tab cycling within the palette instead of escaping to the
// page underneath — this modal isn't teleported out of the app tree and has
// no backdrop-level focus trap, so without this a keyboard-only user could
// tab straight past it into the dimmed navbar/host table behind it.
function trapFocus(e: KeyboardEvent): void {
  const root = dialogRef.value
  if (!root) return
  const focusable = Array.from(
    root.querySelectorAll<HTMLElement>('input, button:not([disabled])')
  )
  if (!focusable.length) return
  const first = focusable[0]
  const last = focusable[focusable.length - 1]
  if (e.shiftKey && document.activeElement === first) {
    e.preventDefault()
    last.focus()
  } else if (!e.shiftKey && document.activeElement === last) {
    e.preventDefault()
    first.focus()
  }
}

interface DisplayResult extends PaletteResult {
  globalIndex: number
}

const groupedResults = computed(() => {
  const groups: { key: string; label: string; items: DisplayResult[] }[] = []
  results.value.forEach((result, globalIndex) => {
    let group = groups.find((g) => g.key === result.group)
    if (!group) {
      group = { key: result.group, label: t(PALETTE_GROUP_LABEL_KEYS[result.group]), items: [] }
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
  /* Without flex-basis + min-width:0, a `width:100%` form-control inside this
     flex row keeps its full intrinsic width instead of sharing space with
     the icon/kbd siblings, pushing the Esc shortcut hint past the card's rounded corner
     (clipped by .command-palette's overflow:hidden) instead of just before it. */
  flex: 1 1 auto;
  min-width: 0;
  border: 0;
  box-shadow: none;
  padding: 0;
  font-size: 1.05rem;
  background: transparent;
}

.command-palette-input:focus {
  box-shadow: none;
}

/* Matches the navbar trigger button's kbd styling (App.vue) — Tabler's
   default <kbd> (dark code-block background + h5-scale font) reads as a
   mismatched "code snippet" chip rather than a subtle shortcut hint here. */
.command-palette-esc {
  flex-shrink: 0;
  margin-left: 0.5rem;
  padding: 0.15rem 0.4rem;
  font-size: 0.7rem;
  line-height: 1.4;
  color: var(--tblr-secondary);
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid var(--tblr-border-color);
  border-radius: 4px;
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

/* Uses the theme's own accent + a translucent bg (not a plain <mark> yellow
   highlight) so it reads correctly on the dark surface and doesn't clash
   with the active/hover row background above. */
.command-palette-match {
  background: rgba(var(--tblr-primary-rgb, 32, 107, 196), 0.22);
  color: var(--tblr-primary, inherit);
  border-radius: 3px;
  padding: 0 1px;
}
</style>
