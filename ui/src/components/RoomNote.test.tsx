import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { RoomCheck } from '../api/types'
import { defaultSettings, SettingsProvider } from '../state/settings'
import { RoomNote } from './RoomNote'

function check(over: Partial<RoomCheck>): RoomCheck {
  return {
    verdict: 'enough',
    message: 'There is room for this: it needs about 4.1 GB and the drive Ollama saves models to (C:) has 120.0 GB free.',
    need: { value: 4_100_000_000, source: 'estimated' },
    free_bytes: 120_000_000_000,
    free_known: true,
    left: { value: 115_900_000_000, source: 'estimated' },
    volume: 'C:',
    where: 'models',
    actions: [],
    ...over,
  }
}

function stub(body: unknown, status = 200) {
  const fetchMock = vi.fn(async () => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } }))
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}

function show(ui: React.ReactElement, advanced = false) {
  return render(
    <SettingsProvider initial={{ ...defaultSettings, advanced }}>
      <MemoryRouter>{ui}</MemoryRouter>
    </SettingsProvider>,
  )
}

afterEach(() => vi.unstubAllGlobals())

describe('RoomNote (the free-space check beside a download button)', () => {
  it('asks the daemon about the tag and shows its sentence when there is not enough room', async () => {
    const onResult = vi.fn()
    const fetchMock = stub(
      check({
        verdict: 'not_enough',
        message: 'This needs about 9.1 GB and the drive Ollama saves models to (C:) has 6.4 GB free. Remove a model you no longer use, or free up space.',
        free_bytes: 6_400_000_000,
        actions: ['remove_models'],
      }),
    )
    show(<RoomNote target={{ kind: 'pull', tag: 'qwen3:14b' }} onResult={onResult} />)

    const note = await screen.findByRole('alert')
    expect(note).toHaveTextContent('This needs about 9.1 GB and the drive Ollama saves models to (C:) has 6.4 GB free.')
    expect(note).toHaveAttribute('data-verdict', 'not_enough')
    expect(screen.getByRole('link', { name: /remove/i })).toHaveAttribute('href', '/models')
    const url = String((fetchMock.mock.calls[0] as unknown[])[0])
    expect(url).toContain('/models/pull/check?ollama_tag=qwen3%3A14b')
    await waitFor(() => expect(onResult).toHaveBeenCalledWith(expect.objectContaining({ verdict: 'not_enough' })))
  })

  it('warns, without an alert, when there is room but little left', async () => {
    stub(check({ verdict: 'low', message: 'After this download, about 7 GB will be left on C:.' }))
    show(<RoomNote target={{ kind: 'pull', tag: 'llama3.2:3b' }} />)
    const note = await screen.findByTestId('room-note')
    expect(note).toHaveTextContent('After this download, about 7 GB will be left on C:.')
    expect(note).toHaveClass('notice--warning')
    expect(screen.queryByRole('alert')).toBeNull()
  })

  it('says there is room when there is, and quiet hides that', async () => {
    stub(check({}))
    const { unmount } = show(<RoomNote target={{ kind: 'pull', tag: 'llama3.2:3b' }} />)
    expect(await screen.findByTestId('room-note')).toHaveAttribute('data-verdict', 'enough')
    unmount()

    const onResult = vi.fn()
    show(<RoomNote target={{ kind: 'pull', tag: 'llama3.2:3b' }} quiet onResult={onResult} />)
    await waitFor(() => expect(onResult).toHaveBeenCalled())
    expect(screen.queryByTestId('room-note')).toBeNull()
  })

  it('quiet still shows a warning that matters', async () => {
    stub(check({ verdict: 'low', message: 'After this download, about 7 GB will be left on C:.' }))
    show(<RoomNote target={{ kind: 'pull', tag: 'x' }} quiet />)
    expect(await screen.findByTestId('room-note')).toBeInTheDocument()
  })

  it('checks the installer download for an install target', async () => {
    const fetchMock = stub(check({ where: 'temp' }))
    show(<RoomNote target={{ kind: 'install', backend: 'ollama' }} />)
    await screen.findByTestId('room-note')
    expect(String((fetchMock.mock.calls[0] as unknown[])[0])).toContain('/backends/ollama/install/check')
  })

  it('says the check could not be made, and never blocks, when the daemon fails', async () => {
    const onResult = vi.fn()
    stub({ error: 'boom' }, 500)
    show(<RoomNote target={{ kind: 'pull', tag: 'x' }} onResult={onResult} />)
    expect(await screen.findByText(/could not check|couldn.t check/i)).toBeInTheDocument()
    await waitFor(() => expect(onResult).toHaveBeenCalledWith(null))
    expect(screen.queryByRole('alert')).toBeNull()
  })

  it('treats an answer that is not a check as no answer', async () => {
    const onResult = vi.fn()
    stub({})
    show(<RoomNote target={{ kind: 'pull', tag: 'x' }} onResult={onResult} />)
    await waitFor(() => expect(onResult).toHaveBeenCalledWith(null))
    expect(screen.queryByTestId('room-note')).toBeNull()
  })

  it('shows the numbers with their sources behind Advanced only', async () => {
    stub(check({}))
    const { unmount } = show(<RoomNote target={{ kind: 'pull', tag: 'x' }} />, false)
    await screen.findByTestId('room-note')
    expect(document.querySelector('dl.facts')).toBeNull()
    unmount()

    show(<RoomNote target={{ kind: 'pull', tag: 'x' }} />, true)
    await screen.findByTestId('room-note')
    const need = document.querySelector('[data-source="estimated"]')
    expect(need).not.toBeNull()
    expect(need).toHaveTextContent('4.1 GB')
    expect(document.querySelector('dl.facts')).toHaveTextContent('120 GB')
  })
})
