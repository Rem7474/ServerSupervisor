import { ref, Ref } from 'vue'
import apiClient, { getApiErrorMessage } from '../api'
import { addToast } from './useGlobalToast'
import { useConfirmDialog } from './useConfirmDialog'
import type { AlertRuleTemplate, AlertRuleTemplateRequest, ApplyAlertRuleTemplateResult } from '../types/generated'

interface UseAlertRuleTemplatesApi {
  templates: Ref<AlertRuleTemplate[]>
  loading: Ref<boolean>
  fetched: Ref<boolean>
  error: Ref<string>
  saving: Ref<boolean>
  saveError: Ref<string>
  applying: Ref<boolean>
  applyError: Ref<string>
  applyResult: Ref<ApplyAlertRuleTemplateResult | null>
  loadTemplates: () => Promise<void>
  createTemplate: (payload: AlertRuleTemplateRequest) => Promise<boolean>
  updateTemplate: (id: number, payload: AlertRuleTemplateRequest) => Promise<boolean>
  deleteTemplate: (template: AlertRuleTemplate) => Promise<void>
  applyTemplate: (id: number, hostIds: string[], enabled: boolean) => Promise<boolean>
  clearApplyResult: () => void
}

// Backs the "Modèles" tab (AlertsView.vue) — reusable agent-metric rule
// recipes applied to N hosts at once (ROADMAP.md item #9). Docker/Proxmox
// scopes aren't templatable (see server's models.AlertRuleTemplate doc
// comment), so this composable only ever deals with the plain
// metric/operator/thresholds/actions shape, no host_id/scope fields.
export function useAlertRuleTemplates(): UseAlertRuleTemplatesApi {
  const { confirm } = useConfirmDialog()

  const templates: Ref<AlertRuleTemplate[]> = ref([])
  const loading = ref(false)
  const fetched = ref(false)
  const error = ref('')
  const saving = ref(false)
  const saveError = ref('')
  const applying = ref(false)
  const applyError = ref('')
  const applyResult: Ref<ApplyAlertRuleTemplateResult | null> = ref(null)

  async function loadTemplates(): Promise<void> {
    loading.value = true
    error.value = ''
    try {
      const res = await apiClient.getAlertRuleTemplates()
      templates.value = res.data || []
      fetched.value = true
    } catch (e) {
      error.value = getApiErrorMessage(e, 'Impossible de charger les modèles de règles')
    } finally {
      loading.value = false
    }
  }

  async function createTemplate(payload: AlertRuleTemplateRequest): Promise<boolean> {
    saving.value = true
    saveError.value = ''
    try {
      await apiClient.createAlertRuleTemplate(payload)
      await loadTemplates()
      return true
    } catch (e) {
      saveError.value = getApiErrorMessage(e, 'Impossible de créer le modèle')
      return false
    } finally {
      saving.value = false
    }
  }

  async function updateTemplate(id: number, payload: AlertRuleTemplateRequest): Promise<boolean> {
    saving.value = true
    saveError.value = ''
    try {
      await apiClient.updateAlertRuleTemplate(id, payload)
      await loadTemplates()
      return true
    } catch (e) {
      saveError.value = getApiErrorMessage(e, 'Impossible de modifier le modèle')
      return false
    } finally {
      saving.value = false
    }
  }

  async function deleteTemplate(template: AlertRuleTemplate): Promise<void> {
    const ok = await confirm({
      title: 'Supprimer le modèle',
      message: `Supprimer le modèle "${template.name}" ? Les règles déjà créées à partir de ce modèle ne sont pas affectées.`,
      variant: 'danger',
      destructive: true,
      okLabel: 'Supprimer',
    })
    if (!ok) return
    try {
      await apiClient.deleteAlertRuleTemplate(template.id)
      templates.value = templates.value.filter((t) => t.id !== template.id)
      addToast('Modèle supprimé', 'success')
    } catch (e) {
      addToast(getApiErrorMessage(e, 'Impossible de supprimer le modèle'), 'error')
    }
  }

  async function applyTemplate(id: number, hostIds: string[], enabled: boolean): Promise<boolean> {
    applying.value = true
    applyError.value = ''
    applyResult.value = null
    try {
      const res = await apiClient.applyAlertRuleTemplate(id, { host_ids: hostIds, enabled })
      applyResult.value = res.data
      const failedCount = Object.keys(res.data.errors || {}).length
      if (failedCount > 0) {
        addToast(`${res.data.created_rule_ids?.length || 0} règle(s) créée(s), ${failedCount} échec(s)`, 'error')
      } else {
        addToast(`${res.data.created_rule_ids?.length || 0} règle(s) créée(s)`, 'success')
      }
      return true
    } catch (e) {
      applyError.value = getApiErrorMessage(e, "Impossible d'appliquer le modèle")
      return false
    } finally {
      applying.value = false
    }
  }

  function clearApplyResult(): void {
    applyResult.value = null
    applyError.value = ''
  }

  return {
    templates,
    loading,
    fetched,
    error,
    saving,
    saveError,
    applying,
    applyError,
    applyResult,
    loadTemplates,
    createTemplate,
    updateTemplate,
    deleteTemplate,
    applyTemplate,
    clearApplyResult,
  }
}
