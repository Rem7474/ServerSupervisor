import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useAuthStore } from '../stores/auth'
import { useConfirmDialog } from './useConfirmDialog'
import apiClient from '../api'
import dayjs from '../utils/dayjs'
import { getApiErrorMessage, isApiAbort } from '../api/client'
import { useAbortSignal } from './useAbortSignal'

interface User {
  id: string
  username: string
  role: string
  created_at?: string
}

const USERS_REFRESH_SEC = 60

export function useUsers() {
  const auth = useAuthStore()
  const signal = useAbortSignal()
  const dialog = useConfirmDialog()
  const users = ref<User[]>([])
  const loading = ref(false)
  const saving = ref(false)
  const creatingUser = ref(false)
  const autoRefresh = ref(true)
  const lastUpdatedAt = ref<Date | null>(null)
  let refreshTimer: ReturnType<typeof setInterval> | null = null

  const newUserForm = ref({
    username: '',
    password: '',
    role: 'viewer',
  })

  const createMessage = ref('')
  const createSuccess = ref(false)

  const actionMessage = ref('')
  const actionSuccess = ref(false)

  function formatDate(date: string | undefined): string {
    if (!date) return '-'
    return dayjs.utc(date).local().format('YYYY-MM-DD HH:mm')
  }

  const adminCount = computed(() => users.value.filter((u) => u.role === 'admin').length)

  function isLastAdmin(userId: string): boolean {
    const user = users.value.find((u) => u.id === userId)
    return !!user && user.role === 'admin' && adminCount.value === 1
  }

  function getDeleteButtonTitle(user: User): string {
    if (user.username === auth.username) return 'Impossible de supprimer votre propre compte'
    if (isLastAdmin(user.id)) return 'Impossible de supprimer le dernier admin'
    return 'Supprimer cet utilisateur'
  }

  async function fetchUsers(): Promise<void> {
    loading.value = true
    try {
      const res = await apiClient.getUsers(signal)
      users.value = res.data || []
      lastUpdatedAt.value = new Date()
    } catch (e) {
      if (isApiAbort(e)) return
      console.error('Erreur lors du chargement des utilisateurs:', getApiErrorMessage(e))
      users.value = []
    } finally {
      loading.value = false
    }
  }

  function startRefreshTimer(): void {
    stopRefreshTimer()
    refreshTimer = setInterval(() => {
      // Skip a tick while a role change/delete is in flight to avoid
      // refetching mid-mutation (saveRole/deleteUser already refetch
      // themselves once the confirm dialog + request settle).
      if (autoRefresh.value && !saving.value) fetchUsers()
    }, USERS_REFRESH_SEC * 1000)
  }

  function stopRefreshTimer(): void {
    if (refreshTimer) { clearInterval(refreshTimer); refreshTimer = null }
  }

  async function createUser(): Promise<void> {
    if (!newUserForm.value.username || !newUserForm.value.password) {
      createMessage.value = 'Veuillez remplir tous les champs'
      createSuccess.value = false
      return
    }

    if (users.value.find((u) => u.username === newUserForm.value.username)) {
      createMessage.value = 'Ce nom d\'utilisateur existe déjà'
      createSuccess.value = false
      return
    }

    creatingUser.value = true
    createMessage.value = ''
    try {
      await apiClient.createUser(
        newUserForm.value.username,
        newUserForm.value.password,
        newUserForm.value.role
      )
      createSuccess.value = true
      createMessage.value = 'Utilisateur créé avec succès'
      newUserForm.value = { username: '', password: '', role: 'viewer' }
      await fetchUsers()
    } catch (e: unknown) {
      createSuccess.value = false
      createMessage.value = getApiErrorMessage(e, 'Erreur lors de la création')
    } finally {
      creatingUser.value = false
    }
  }

  async function saveRole(user: User): Promise<void> {
    const confirmed = await dialog.confirm({
      title: 'Modifier le rôle',
      message: `Attribuer le rôle « ${user.role} » à ${user.username} ?`,
      variant: 'warning',
    })
    if (!confirmed) {
      await fetchUsers()
      return
    }
    saving.value = true
    try {
      await apiClient.updateUserRole(user.id, user.role)
      actionSuccess.value = true
      actionMessage.value = `Rôle de ${user.username} mis à jour.`
    } catch (e) {
      actionSuccess.value = false
      actionMessage.value = getApiErrorMessage(e, `Échec de la mise à jour du rôle de ${user.username}.`)
      await fetchUsers()
    } finally {
      saving.value = false
    }
  }

  async function deleteUser(user: User): Promise<void> {
    const confirmed = await dialog.confirm({
      title: `Supprimer l'utilisateur`,
      message: `Cette action est irréversible.`,
      variant: 'danger',
      requiredText: user.username,
    })
    if (!confirmed) return

    saving.value = true
    try {
      await apiClient.deleteUser(user.id)
      actionSuccess.value = true
      actionMessage.value = `Utilisateur ${user.username} supprimé.`
      await fetchUsers()
    } catch (e) {
      actionSuccess.value = false
      actionMessage.value = getApiErrorMessage(e, `Échec de la suppression de ${user.username}.`)
    } finally {
      saving.value = false
    }
  }

  onMounted(() => {
    fetchUsers()
    startRefreshTimer()
  })
  onUnmounted(stopRefreshTimer)

  return {
    auth,
    users,
    loading,
    saving,
    creatingUser,
    newUserForm,
    createMessage,
    createSuccess,
    actionMessage,
    actionSuccess,
    autoRefresh,
    lastUpdatedAt,
    USERS_REFRESH_SEC,
    formatDate,
    isLastAdmin,
    getDeleteButtonTitle,
    fetchUsers,
    createUser,
    saveRole,
    deleteUser,
  }
}
