import { ref, Ref } from 'vue'
import apiClient, { getApiErrorMessage } from '../api'
import { addToast } from './useGlobalToast'
import { useConfirmDialog } from './useConfirmDialog'
import type { AlertRuleTemplate, AlertRuleTemplateRequest, ApplyAlertRuleTemplateResult } from '../types/generated'
import { i18n } from '../i18n'

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
      error.value = getApiErrorMessage(e, i18n.global.t('alerts.templatesLoadError'))
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
      saveError.value = getApiErrorMessage(e, i18n.global.t('alerts.templateCreateError'))
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
      saveError.value = getApiErrorMessage(e, i18n.global.t('alerts.templateUpdateError'))
      return false
    } finally {
      saving.value = false
    }
  }

  async function deleteTemplate(template: AlertRuleTemplate): Promise<void> {
    const ok = await confirm({
      title: i18n.global.t('alerts.templateDeleteConfirmTitle'),
      message: i18n.global.t('alerts.templateDeleteConfirmMessage', { name: template.name }),
      variant: 'danger',
      destructive: true,
      okLabel: i18n.global.t('common.delete'),
    })
    if (!ok) return
    try {
      await apiClient.deleteAlertRuleTemplate(template.id)
      templates.value = templates.value.filter((t) => t.id !== template.id)
      addToast(i18n.global.t('alerts.templateDeletedToast'), 'success')
    } catch (e) {
      addToast(getApiErrorMessage(e, i18n.global.t('alerts.templateDeleteError')), 'error')
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
        const created = res.data.created_rule_ids?.length || 0
        // Two independent counts can't share one plural index, so each half is
        // pluralized on its own and the wrapper only joins them.
        addToast(i18n.global.t('alerts.templateApplyPartialToast', {
          createdPart: i18n.global.t('alerts.templateApplySuccessToast', { created }, created),
          failedPart: i18n.global.t('alerts.templateApplyFailedCount', { failed: failedCount }, failedCount),
        }), 'error')
      } else {
        const created = res.data.created_rule_ids?.length || 0
        addToast(i18n.global.t('alerts.templateApplySuccessToast', { created }, created), 'success')
      }
      return true
    } catch (e) {
      applyError.value = getApiErrorMessage(e, i18n.global.t('alerts.templateApplyError'))
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
