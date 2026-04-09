import { ref, computed } from 'vue'

export interface Tab {
  key: string
  label: string
}

export function useTabNavigation(tabs: Tab[]) {
  const activeTab = ref(tabs[0]?.key || '')

  const currentIndex = computed(() => tabs.findIndex(t => t.key === activeTab.value))

  const handleKeyDown = (e: KeyboardEvent) => {
    if (e.key === 'ArrowRight' || e.key === 'ArrowDown') {
      e.preventDefault()
      const nextIndex = (currentIndex.value + 1) % tabs.length
      activeTab.value = tabs[nextIndex].key
    } else if (e.key === 'ArrowLeft' || e.key === 'ArrowUp') {
      e.preventDefault()
      const prevIndex = (currentIndex.value - 1 + tabs.length) % tabs.length
      activeTab.value = tabs[prevIndex].key
    } else if (e.key === 'Home') {
      e.preventDefault()
      activeTab.value = tabs[0].key
    } else if (e.key === 'End') {
      e.preventDefault()
      activeTab.value = tabs[tabs.length - 1].key
    }
  }

  return {
    activeTab,
    currentIndex,
    handleKeyDown
  }
}
