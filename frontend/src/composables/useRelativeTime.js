import { ref, onMounted, onUnmounted } from 'vue'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import utc from 'dayjs/plugin/utc'
import 'dayjs/locale/fr'

dayjs.extend(relativeTime)
dayjs.extend(utc)
dayjs.locale('fr')

/**
 * Composable pour afficher un timestamp en temps relatif avec mise à jour automatique
 * @param {string|Date|Ref} dateInput - La date à afficher (peut être une ref Vue)
 * @param {number} updateInterval - Intervalle de mise à jour en ms (défaut: 1000ms = 1 seconde)
 * @returns {Ref<string>} - Le texte formaté du temps relatif
 */
export function useRelativeTime(dateInput, updateInterval = 1000) {
  const relativeText = ref('')
  let intervalId = null

  function updateRelativeTime() {
    const date = typeof dateInput === 'function' ? dateInput() : (dateInput?.value || dateInput)
    
    if (!date || date === '0001-01-01T00:00:00Z') {
      relativeText.value = 'Jamais'
      return
    }

    const dateObj = dayjs.utc(date).local()
    const now = dayjs()
    const diffSeconds = now.diff(dateObj, 'second')

    // Moins de 10 secondes : "il y a quelques secondes"
    if (diffSeconds < 10) {
      relativeText.value = 'il y a quelques secondes'
    }
    // Entre 10s et 60s : "il y a Xs"
    else if (diffSeconds < 60) {
      relativeText.value = `il y a ${diffSeconds}s`
    }
    // Plus de 60s : utiliser dayjs fromNow()
    else {
      relativeText.value = dateObj.fromNow()
    }
  }

  onMounted(() => {
    updateRelativeTime()
    intervalId = setInterval(updateRelativeTime, updateInterval)
  })

  onUnmounted(() => {
    if (intervalId) {
      clearInterval(intervalId)
    }
  })

  return relativeText
}

/**
 * Fonction utilitaire pour formater une date sans réactivité
 * @param {string|Date} date - La date à formater
 * @returns {string} - Le texte formaté
 */
export function formatRelativeTime(date) {
  if (!date || date === '0001-01-01T00:00:00Z') {
    return 'Jamais'
  }

  const dateObj = dayjs.utc(date).local()
  const now = dayjs()
  const diffSeconds = now.diff(dateObj, 'second')

  if (diffSeconds < 10) {
    return 'il y a quelques secondes'
  } else if (diffSeconds < 60) {
    return `il y a ${diffSeconds}s`
  } else {
    return dateObj.fromNow()
  }
}
