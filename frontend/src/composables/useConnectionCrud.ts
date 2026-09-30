import { ref, onMounted, type Ref, type UnwrapRef } from 'vue'
import { useI18n } from 'vue-i18n'
import { getApiErrorMessage } from '../api/client'
import { useConfirmDialog } from './useConfirmDialog'

interface ConnectionCrudOptions<TItem extends { id: string; name: string }, TForm extends object> {
  list: () => Promise<TItem[]>
  create: (form: TForm) => Promise<unknown>
  update: (id: string, form: TForm) => Promise<unknown>
  remove: (id: string) => Promise<unknown>
  emptyForm: () => TForm
  /** Edit form for an item; secrets stay blank ("unchanged if empty"). */
  toForm: (item: TItem) => TForm
  /** Returns a message when the form can't be saved (required fields, secret on create). */
  validate: (form: TForm, editing: boolean) => string
  /** i18n keys; deleteTitle/deleteMessage receive `{ name }`. */
  keys: { created: string; updated: string; deleted: string; deleteTitle: string; deleteMessage: string }
}

/**
 * List + inline add/edit form + confirmed delete for a settings card that
 * manages named server-side records holding a secret (NPM and Proxmox
 * connections, registry credentials). Each card keeps its own markup and
 * its own extras (connection tests, poll/refresh buttons).
 */
export function useConnectionCrud<TItem extends { id: string; name: string }, TForm extends object>(
  options: ConnectionCrudOptions<TItem, TForm>,
) {
  const { t } = useI18n()
  const { confirm } = useConfirmDialog()

  const items = ref([]) as Ref<TItem[]>
  const loading = ref(false)
  const showForm = ref(false)
  const editingId = ref<string | null>(null)
  const form = ref(options.emptyForm()) as Ref<UnwrapRef<TForm>>
  const saving = ref(false)
  const formMsg = ref('')
  const formOk = ref(false)
  const listMsg = ref('')
  const listOk = ref(false)

  async function load(): Promise<void> {
    loading.value = true
    try {
      items.value = await options.list()
    } catch {
      // Keep the current list; the card shows its empty state on first load.
    } finally {
      loading.value = false
    }
  }

  function openForm(id: string | null, value: TForm): void {
    editingId.value = id
    form.value = value as UnwrapRef<TForm>
    formMsg.value = ''
    showForm.value = true
  }

  const openAddForm = (): void => openForm(null, options.emptyForm())
  const openEditForm = (item: TItem): void => openForm(item.id, options.toForm(item))

  function cancelForm(): void {
    showForm.value = false
    formMsg.value = ''
    editingId.value = null
  }

  async function save(): Promise<void> {
    const current = form.value as TForm
    const invalid = options.validate(current, editingId.value !== null)
    if (invalid) {
      formMsg.value = invalid
      formOk.value = false
      return
    }
    saving.value = true
    formMsg.value = ''
    const wasEditing = editingId.value !== null
    try {
      if (editingId.value) await options.update(editingId.value, current)
      else await options.create(current)
      formMsg.value = t(wasEditing ? options.keys.updated : options.keys.created)
      formOk.value = true
      await load()
      showForm.value = false
      editingId.value = null
    } catch (e: unknown) {
      formMsg.value = getApiErrorMessage(e, t('settings.saveError'))
      formOk.value = false
    } finally {
      saving.value = false
    }
  }

  async function remove(item: TItem): Promise<void> {
    const confirmed = await confirm({
      title: t(options.keys.deleteTitle, { name: item.name }),
      message: t(options.keys.deleteMessage, { name: item.name }),
      variant: 'danger',
    })
    if (!confirmed) return
    try {
      await options.remove(item.id)
      await load()
      listMsg.value = t(options.keys.deleted)
      listOk.value = true
    } catch (e: unknown) {
      listMsg.value = getApiErrorMessage(e, t('settings.deleteError'))
      listOk.value = false
    }
  }

  /**
   * Runs a row action (refresh now, poll now) and reports it in the list
   * message; the server works asynchronously, so the list reloads a few
   * seconds later to pick up the result.
   */
  async function runListAction(action: () => Promise<unknown>, successMsg: string): Promise<void> {
    try {
      await action()
      listMsg.value = successMsg
      listOk.value = true
      setTimeout(() => { void load() }, 3000)
    } catch (e: unknown) {
      listMsg.value = getApiErrorMessage(e, t('settings.genericErrorPeriod'))
      listOk.value = false
    }
  }

  onMounted(load)

  return {
    items, loading, showForm, editingId, form, saving, formMsg, formOk, listMsg, listOk,
    load, openAddForm, openEditForm, cancelForm, save, remove, runListAction,
  }
}
