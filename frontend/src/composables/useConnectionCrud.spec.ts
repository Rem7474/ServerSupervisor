import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { defineComponent, h } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { setLocale } from '../i18n'

const confirm = vi.hoisted(() => vi.fn())
vi.mock('./useConfirmDialog', () => ({ useConfirmDialog: () => ({ confirm }) }))

import { useConnectionCrud } from './useConnectionCrud'

type Item = { id: string; name: string; url: string }
type Form = { name: string; url: string; secret: string }

const serverError = (msg: string) => ({ isAxiosError: true, response: { data: { error: msg } } })

function setup(overrides: Partial<Parameters<typeof useConnectionCrud<Item, Form>>[0]> = {}) {
  const api = {
    list: vi.fn(async () => [{ id: 'a', name: 'alpha', url: 'https://a' }] as Item[]),
    create: vi.fn(async () => ({})),
    update: vi.fn(async () => ({})),
    remove: vi.fn(async () => ({})),
  }
  let crud!: ReturnType<typeof useConnectionCrud<Item, Form>>
  mount(defineComponent({
    setup() {
      crud = useConnectionCrud<Item, Form>({
        ...api,
        emptyForm: () => ({ name: '', url: '', secret: '' }),
        toForm: (i) => ({ name: i.name, url: i.url, secret: '' }),
        validate: (f, editing) => (!f.name ? 'name required' : !editing && !f.secret ? 'secret required' : ''),
        keys: {
          created: 'settings.connectionCreated',
          updated: 'settings.connectionUpdated',
          deleted: 'settings.connectionDeleted',
          deleteTitle: 'settings.deleteNpmConnectionTitle',
          deleteMessage: 'settings.deleteNpmConnectionMsg',
        },
        ...overrides,
      })
      return () => h('div')
    },
  }))
  return { crud, api }
}

describe('useConnectionCrud', () => {
  beforeEach(() => {
    setLocale('fr')
    confirm.mockReset()
  })
  afterEach(() => vi.useRealTimers())

  it('loads the list on mount', async () => {
    const { crud } = setup()
    await flushPromises()
    expect(crud.items.value.map((i) => i.id)).toEqual(['a'])
    expect(crud.loading.value).toBe(false)
  })

  it('requires the secret when creating but not when editing', async () => {
    const { crud, api } = setup()
    await flushPromises()

    crud.openAddForm()
    crud.form.value = { name: 'beta', url: 'https://b', secret: '' }
    await crud.save()
    expect(crud.formMsg.value).toBe('secret required')
    expect(api.create).not.toHaveBeenCalled()

    crud.openEditForm(crud.items.value[0])
    expect(crud.form.value).toEqual({ name: 'alpha', url: 'https://a', secret: '' })
    await crud.save()
    expect(api.update).toHaveBeenCalledWith('a', { name: 'alpha', url: 'https://a', secret: '' })
    expect(crud.formOk.value).toBe(true)
    expect(crud.showForm.value).toBe(false)
  })

  it('creates, reloads and closes the form on success', async () => {
    const { crud, api } = setup()
    await flushPromises()
    crud.openAddForm()
    crud.form.value = { name: 'beta', url: 'https://b', secret: 's3cret' }
    await crud.save()
    expect(api.create).toHaveBeenCalledWith({ name: 'beta', url: 'https://b', secret: 's3cret' })
    expect(api.list).toHaveBeenCalledTimes(2)
    expect(crud.formOk.value).toBe(true)
    expect(crud.showForm.value).toBe(false)
  })

  it('keeps the form open with the server error when saving fails', async () => {
    const { crud } = setup({ create: vi.fn(async () => { throw serverError('nom déjà utilisé') }) })
    await flushPromises()
    crud.openAddForm()
    crud.form.value = { name: 'beta', url: 'https://b', secret: 'x' }
    await crud.save()
    expect(crud.formMsg.value).toBe('nom déjà utilisé')
    expect(crud.formOk.value).toBe(false)
    expect(crud.showForm.value).toBe(true)
    expect(crud.saving.value).toBe(false)
  })

  it('deletes only after confirmation', async () => {
    const { crud, api } = setup()
    await flushPromises()
    confirm.mockResolvedValueOnce(false)
    await crud.remove(crud.items.value[0])
    expect(api.remove).not.toHaveBeenCalled()

    confirm.mockResolvedValueOnce(true)
    await crud.remove(crud.items.value[0])
    expect(api.remove).toHaveBeenCalledWith('a')
    expect(crud.listOk.value).toBe(true)
  })

  it('reports a failed row action, and reloads a few seconds after a successful one', async () => {
    const { crud, api } = setup()
    await flushPromises()
    await crud.runListAction(async () => { throw serverError('poll refusé') }, 'ok')
    expect(crud.listMsg.value).toBe('poll refusé')

    vi.useFakeTimers()
    await crud.runListAction(async () => ({}), 'Collecte lancée')
    expect(crud.listMsg.value).toBe('Collecte lancée')
    const before = api.list.mock.calls.length
    await vi.advanceTimersByTimeAsync(3000)
    expect(api.list.mock.calls.length).toBe(before + 1)
  })
})
