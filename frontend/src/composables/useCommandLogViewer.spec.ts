import { describe, it, expect, vi, beforeEach } from 'vitest'

type Handlers = {
  onInit?: (p: { status: string; output?: string }) => void
  onChunk?: (p: { chunk: string }) => void
  onStatus?: (p: { status: string; output?: string }) => void
}

const stream = vi.hoisted(() => ({
  opened: [] as { id: string; handlers: Handlers }[],
  closeStream: vi.fn(),
}))

const getCommandStatus = vi.hoisted(() => vi.fn())
vi.mock('../api', () => ({ default: { getCommandStatus } }))

vi.mock('./useCommandStream', () => ({
  useCommandStream: () => ({
    openCommandStream: (id: string, handlers: Handlers) => {
      stream.opened.push({ id, handlers })
      return {} as WebSocket
    },
    closeStream: stream.closeStream,
  }),
}))

import { ref } from 'vue'
import { useCommandLogViewer, patchRow } from './useCommandLogViewer'

type Row = { id: string; status?: string; output?: string; host_name?: string }

describe('useCommandLogViewer', () => {
  beforeEach(() => {
    stream.opened = []
    stream.closeStream.mockReset()
    getCommandStatus.mockReset()
  })

  it('shows a finished command without opening a stream', () => {
    const viewer = useCommandLogViewer<Row>()
    const row = { id: 'c1', status: 'completed', output: 'done', host_name: 'web' }
    viewer.show(row)

    expect(viewer.visible.value).toBe(true)
    expect(viewer.selected.value).toEqual(row)
    expect(viewer.selected.value).not.toBe(row) // a copy: the list row is not mutated by streaming
    expect(stream.opened).toHaveLength(0)
  })

  it.each(['pending', 'running'])('follows the live output of a %s command', (status) => {
    const onStatus = vi.fn()
    const viewer = useCommandLogViewer<Row>({ onStatus })
    viewer.show({ id: 'c1', status, output: '' })
    expect(stream.opened.map((s) => s.id)).toEqual(['c1'])

    const h = stream.opened[0].handlers
    h.onInit?.({ status: 'running', output: 'line 1\n' })
    h.onChunk?.({ chunk: 'line 2\n' })
    h.onStatus?.({ status: 'completed' })

    expect(viewer.selected.value).toMatchObject({ status: 'completed', output: 'line 1\nline 2\n' })
    expect(onStatus.mock.calls).toEqual([
      ['c1', 'running', 'line 1\n'],
      ['c1', 'completed', undefined],
    ])
  })

  it('ignores late messages from a command that is no longer selected', () => {
    const viewer = useCommandLogViewer<Row>()
    viewer.show({ id: 'c1', status: 'running', output: '' })
    const first = stream.opened[0].handlers
    viewer.show({ id: 'c2', status: 'completed', output: 'other' })

    first.onChunk?.({ chunk: 'stale' })
    first.onStatus?.({ status: 'failed', output: 'stale' })

    expect(viewer.selected.value).toMatchObject({ id: 'c2', status: 'completed', output: 'other' })
  })

  it('closes the previous stream before showing another command, and on close', () => {
    const viewer = useCommandLogViewer<Row>()
    viewer.show({ id: 'c1', status: 'running' })
    viewer.show({ id: 'c2', status: 'running' })
    expect(stream.closeStream).toHaveBeenCalledTimes(2)

    viewer.close()
    expect(stream.closeStream).toHaveBeenCalledTimes(3)
    expect(viewer.selected.value).toBeNull()
    expect(viewer.visible.value).toBe(false)
  })
})

describe('useCommandLogViewer — partial stream messages', () => {
  beforeEach(() => {
    stream.opened = []
  })

  it('keeps the known status and output when a message omits them', () => {
    const viewer = useCommandLogViewer<Row>()
    viewer.show({ id: 'c1', status: 'running', output: 'so far\n' })
    const h = stream.opened[0].handlers

    h.onInit?.({ status: '' })
    expect(viewer.selected.value).toMatchObject({ status: 'running', output: 'so far\n' })

    h.onStatus?.({ status: '' })
    expect(viewer.selected.value).toMatchObject({ status: 'running', output: 'so far\n' })
  })
})

describe('useCommandLogViewer — showById', () => {
  beforeEach(() => {
    stream.opened = []
    getCommandStatus.mockReset()
  })

  it('fetches the command, shows it and follows it when still running', async () => {
    getCommandStatus.mockResolvedValue({ data: { id: 'c9', status: 'running', output: '' } })
    const viewer = useCommandLogViewer<Row>()
    await expect(viewer.showById('c9')).resolves.toBe(true)
    expect(getCommandStatus).toHaveBeenCalledWith('c9')
    expect(viewer.visible.value).toBe(true)
    expect(stream.opened.map((s) => s.id)).toEqual(['c9'])
  })

  it('reports a failed fetch and leaves the console closed', async () => {
    getCommandStatus.mockRejectedValue(new Error('gone'))
    const viewer = useCommandLogViewer<Row>()
    await expect(viewer.showById('c9')).resolves.toBe(false)
    expect(viewer.visible.value).toBe(false)
  })
})

describe('patchRow', () => {
  it('replaces the matching row with a patched copy and leaves the rest', () => {
    const rows = ref([{ id: 'a', status: 'running' }, { id: 'b', status: 'running' }])
    const before = rows.value
    patchRow(rows, (r) => r.id === 'b', { status: 'failed' })
    expect(rows.value).toEqual([{ id: 'a', status: 'running' }, { id: 'b', status: 'failed' }])
    expect(rows.value).not.toBe(before)
  })

  it('does nothing when no row matches', () => {
    const rows = ref([{ id: 'a', status: 'running' }])
    const before = rows.value
    patchRow(rows, (r) => r.id === 'zz', { status: 'failed' })
    expect(rows.value).toBe(before)
  })
})
