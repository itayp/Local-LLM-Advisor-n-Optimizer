import { act, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { en } from '../copy/en'
import { CopyButton } from './CopyButton'

const c = en.copyButton
const name = 'qwen3:8b'

// user-event installs its own clipboard, so these tests use fireEvent and a
// clipboard of their own.
function stubClipboard(writeText: ((v: string) => Promise<void>) | undefined) {
  Object.defineProperty(navigator, 'clipboard', { value: writeText ? { writeText } : undefined, configurable: true })
}

async function click() {
  await act(async () => {
    fireEvent.click(screen.getByRole('button'))
  })
}

beforeEach(() => {
  vi.useFakeTimers()
})

afterEach(() => {
  vi.useRealTimers()
  vi.restoreAllMocks()
  Reflect.deleteProperty(document, 'execCommand')
})

describe('CopyButton', () => {
  it('has a spoken name that says which name it copies', () => {
    stubClipboard(async () => undefined)
    render(<CopyButton value={name} />)
    expect(screen.getByRole('button', { name: c.labelFor(name) })).toHaveTextContent(c.label)
  })

  it('copies the value, says Copied for two seconds, then goes back to Copy', async () => {
    const writeText = vi.fn(async () => undefined)
    stubClipboard(writeText)
    render(<CopyButton value={name} />)
    await click()
    expect(writeText).toHaveBeenCalledWith(name)
    expect(screen.getByRole('button')).toHaveTextContent(c.copied)

    act(() => {
      vi.advanceTimersByTime(1999)
    })
    expect(screen.getByRole('button')).toHaveTextContent(c.copied)
    act(() => {
      vi.advanceTimersByTime(1)
    })
    expect(screen.getByRole('button')).toHaveTextContent(c.label)
  })

  it('falls back to selecting and copying when the clipboard is refused', async () => {
    stubClipboard(async () => {
      throw new Error('denied')
    })
    const exec = vi.fn(() => true)
    Object.defineProperty(document, 'execCommand', { value: exec, configurable: true })
    render(<CopyButton value={name} />)
    await click()
    expect(exec).toHaveBeenCalledWith('copy')
    expect(screen.getByRole('button')).toHaveTextContent(c.copied)
    expect(screen.queryByRole('status')).toBeNull()
  })

  it('says so quietly, with no alert, when nothing can copy', async () => {
    stubClipboard(undefined)
    render(<CopyButton value={name} />)
    await click()
    expect(screen.getByRole('status')).toHaveTextContent(c.failed)
    expect(screen.queryByRole('alert')).toBeNull()
    expect(screen.getByRole('button')).toHaveTextContent(c.label)

    act(() => {
      vi.advanceTimersByTime(2000)
    })
    expect(screen.queryByRole('status')).toBeNull()
  })
})
