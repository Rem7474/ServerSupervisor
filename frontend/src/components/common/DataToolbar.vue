<template>
  <div class="card mb-3">
    <div class="card-body">
      <div class="d-flex flex-column flex-lg-row gap-3 align-items-stretch align-items-lg-center justify-content-between">
        <div class="ui-toolbar-left d-flex flex-wrap gap-2 align-items-center">
          <slot name="left">
            <input
              v-if="searchable"
              :value="search"
              type="text"
              class="form-control ui-toolbar-search"
              :placeholder="searchPlaceholder"
              @input="emit('update:search', ($event.target as HTMLInputElement).value)"
            >
          </slot>
        </div>
        <div class="ui-toolbar-right d-flex flex-wrap gap-2 align-items-center justify-content-lg-end">
          <slot name="right" />
        </div>
      </div>
      <div
        v-if="$slots.bottom"
        class="mt-3"
      >
        <slot name="bottom" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
withDefaults(defineProps<{
  searchable?: boolean
  search?: string
  searchPlaceholder?: string
}>(), {
  searchable: false,
  search: '',
  searchPlaceholder: 'Rechercher...',
})

const emit = defineEmits<{
  (e: 'update:search', value: string): void
}>()
</script>

<style scoped>
.ui-toolbar-search {
  min-width: 260px;
}

@media (max-width: 991px) {
  .ui-toolbar-search {
    min-width: 0;
    width: 100%;
  }

  .ui-toolbar-left,
  .ui-toolbar-right {
    width: 100%;
  }
}
</style>
