<template>
  <ul v-if="totalPages > 1" class="pagination pagination-sm mb-0">
    <li class="page-item" :class="{ disabled: currentPage <= 1 }">
      <a class="page-link" href="#" @click.prevent="emitSelect(currentPage - 1)">‹</a>
    </li>
    <li
      v-for="item in visibleItems"
      :key="item.key"
      class="page-item"
      :class="{ active: item.type === 'page' && item.value === currentPage, disabled: item.type === 'ellipsis' }"
    >
      <span v-if="item.type === 'ellipsis'" class="page-link">…</span>
      <a v-else class="page-link" href="#" @click.prevent="emitSelect(item.value)">{{ item.value }}</a>
    </li>
    <li class="page-item" :class="{ disabled: currentPage >= totalPages }">
      <a class="page-link" href="#" @click.prevent="emitSelect(currentPage + 1)">›</a>
    </li>
  </ul>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  currentPage: {
    type: Number,
    required: true,
  },
  totalPages: {
    type: Number,
    required: true,
  },
  siblingCount: {
    type: Number,
    default: 1,
  },
})

const emit = defineEmits(['select'])

const visibleItems = computed(() => {
  const total = Math.max(1, props.totalPages)
  const current = Math.min(Math.max(1, props.currentPage), total)
  const siblingCount = Math.max(1, props.siblingCount)

  if (total <= 7) {
    return Array.from({ length: total }, (_, index) => ({
      type: 'page',
      value: index + 1,
      key: `page-${index + 1}`,
    }))
  }

  const items = []
  const startPage = Math.max(2, current - siblingCount)
  const endPage = Math.min(total - 1, current + siblingCount)

  items.push({ type: 'page', value: 1, key: 'page-1' })

  if (startPage > 2) {
    items.push({ type: 'ellipsis', key: `ellipsis-left-${startPage}` })
  }

  for (let page = startPage; page <= endPage; page += 1) {
    items.push({ type: 'page', value: page, key: `page-${page}` })
  }

  if (endPage < total - 1) {
    items.push({ type: 'ellipsis', key: `ellipsis-right-${endPage}` })
  }

  items.push({ type: 'page', value: total, key: `page-${total}` })

  return items
})

function emitSelect(page) {
  if (page < 1 || page > props.totalPages || page === props.currentPage) return
  emit('select', page)
}
</script>