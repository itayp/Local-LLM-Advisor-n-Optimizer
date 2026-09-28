import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { SpeedNeedsResponse } from '../api/types'
import { en } from '../copy/en'
import { glossary } from '../copy/glossary'

// Term's tokens_per_sec extra content caches GET /api/speed-needs at module
// scope (one fetch no matter how many <Term> instances a page has), so each
// test resets the module registry and imports Term fresh — otherwise a
// later test would see the first test's cached response.
async function freshTerm() {
  vi.resetModules()
  const mod = await import('./Term')
  return mod.Term
}

function json(v: unknown, status = 200) {
  return new Response(JSON.stringify(v), { status })
}

const fixture: SpeedNeedsResponse = {
  words_per_token: 0.75,
  purposes: [
    { purpose: 'chat', mode: 'read_along', stream: { excellent: 14, good: 7, usable: 4 }, wait_s: { excellent: 1, good: 4, usable: 10 } },
    { purpose: 'agentic', mode: 'per_step', wait_s: { excellent: 10, good: 30, usable: 120 } },
  ],
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('Term', () => {
  it('shows the one-line explainer for an ordinary term without fetching anything', async () => {
    const calls: string[] = []
    vi.stubGlobal(
      'fetch',
      vi.fn(async (url: string) => {
        calls.push(url)
        return json({})
      }),
    )
    const Term = await freshTerm()
    const user = userEvent.setup()
    render(<Term id="vram" />)
    await user.click(screen.getByRole('button', { name: /what is vram/i }))
    expect(await screen.findByRole('note')).toHaveTextContent(glossary.vram.explain)
    expect(calls).not.toContain('/api/speed-needs')
  })

  it('does not read the speed table until the tokens_per_sec explainer is opened', async () => {
    const calls: string[] = []
    vi.stubGlobal(
      'fetch',
      vi.fn(async (url: string) => {
        calls.push(url)
        return json(fixture)
      }),
    )
    const Term = await freshTerm()
    render(<Term id="tokens_per_sec" />)
    expect(calls).not.toContain('/api/speed-needs')
  })

  it('shows what a speed is good for, per purpose, once opened — numbers from the API, not the copy', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => json(fixture)))
    const Term = await freshTerm()
    const user = userEvent.setup()
    render(<Term id="tokens_per_sec" />)
    await user.click(screen.getByRole('button', { name: /what is tok\/s/i }))

    const panel = await screen.findByTestId('speed-needs')
    expect(panel).toHaveTextContent(en.speedNeeds.heading)
    expect(panel).toHaveTextContent(en.speedNeeds.intro)

    const chatLabel = en.screens.recommend.purposes.chat
    expect(panel).toHaveTextContent(en.speedNeeds.streamLine(chatLabel, 14 * 0.75, 7 * 0.75, 4 * 0.75))
    expect(panel).toHaveTextContent(en.speedNeeds.waitLine(chatLabel, 1, 4, 10))

    const agenticLabel = en.screens.recommend.purposes.agentic
    expect(panel).toHaveTextContent(en.speedNeeds.stepLine(agenticLabel, 10, 30, 120))
    // agentic has no stream bar (per_step): its wait line reads as a whole
    // step, never as an answer streaming in.
    expect(panel).not.toHaveTextContent(en.speedNeeds.waitLine(agenticLabel, 10, 30, 120))
  })

  it('says the table could not be read, rather than showing nothing', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => json({ error: { code: 'speed_needs', message: 'boom' } }, 500)))
    const Term = await freshTerm()
    const user = userEvent.setup()
    render(<Term id="tokens_per_sec" />)
    await user.click(screen.getByRole('button', { name: /what is tok\/s/i }))
    expect(await screen.findByText(en.speedNeeds.failed)).toBeInTheDocument()
  })
})

/**
 * A sentence's own words: its text with every explainer popover taken out.
 * Term keeps its popover inside the sentence's element (Tab reaches it there),
 * so what the reader reads *as the sentence* is what is left without it.
 */
function ownText(sentence: HTMLElement): string {
  const copy = sentence.cloneNode(true) as HTMLElement
  copy.querySelectorAll('[role="note"]').forEach((n) => n.remove())
  return copy.textContent ?? ''
}

describe('Term opened inside a sentence (backlog p)', () => {
  const closedText = 'Keeps about 8,000 words of context window? in mind at once.'

  it('leaves the sentence around it whole, closed and open', async () => {
    const Term = await freshTerm()
    const user = userEvent.setup()
    render(
      <p data-testid="sentence">
        Keeps about 8,000 words of <Term id="context_window" /> in mind at once.
      </p>,
    )
    const p = screen.getByTestId('sentence')
    expect(ownText(p)).toBe(closedText)

    await user.click(screen.getByRole('button', { name: /what is context window/i }))
    const note = screen.getByRole('note')
    expect(note).toHaveTextContent(glossary.context_window.explain)

    // The bug: the explainer's own "…in mind at once." ran into the
    // sentence's. Now the sentence, minus the popover, is exactly what it was,
    // and its ending appears once outside the popover.
    expect(ownText(p)).toBe(closedText)
    expect(ownText(p).match(/in mind at once/g)).toHaveLength(1)
    // …and the popover is one element, out of the line's flow rather than
    // words spliced between the sentence's own.
    expect(p.querySelectorAll('[role="note"]')).toHaveLength(1)
    expect(note.style.position).toBe('fixed')
  })

  it('is phrasing content only, so it is valid inside a <p>, for every kind of explainer', async () => {
    const errors = vi.spyOn(console, 'error').mockImplementation(() => undefined)
    vi.stubGlobal('fetch', vi.fn(async () => json(fixture)))
    const Term = await freshTerm()
    const user = userEvent.setup()
    render(
      <p data-testid="sentence">
        one <Term id="vram" /> two <Term id="tokens_per_sec" />
      </p>,
    )
    for (const b of screen.getAllByRole('button')) await user.click(b)
    await screen.findByTestId('speed-needs')
    const p = screen.getByTestId('sentence')
    expect(p.querySelector('p, div, ul, ol, li, table, details, section, h1, h2, h3')).toBeNull()
    expect(errors.mock.calls.filter((c) => String(c[0]).includes('validateDOMNesting'))).toEqual([])
    errors.mockRestore()
  })

  it('opens on Enter, is reached with Tab, and closes on Escape with focus back on the "?"', async () => {
    const Term = await freshTerm()
    const user = userEvent.setup()
    render(
      <p>
        of <Term id="context_window" /> at once
      </p>,
    )
    const toggle = screen.getByRole('button', { name: /what is context window/i })
    await user.tab()
    expect(toggle).toHaveFocus()
    await user.keyboard('{Enter}')
    expect(toggle).toHaveAttribute('aria-expanded', 'true')

    await user.tab()
    expect(screen.getByRole('note')).toHaveFocus() // scrollable from the keyboard

    await user.keyboard('{Escape}')
    expect(screen.queryByRole('note')).toBeNull()
    expect(toggle).toHaveAttribute('aria-expanded', 'false')
    expect(toggle).toHaveFocus()
  })

  it('closes on the "?" again and on a tap outside', async () => {
    const Term = await freshTerm()
    const user = userEvent.setup()
    render(
      <div>
        <p>
          of <Term id="context_window" /> at once
        </p>
        <button type="button">elsewhere</button>
      </div>,
    )
    const toggle = screen.getByRole('button', { name: /what is context window/i })
    await user.click(toggle)
    expect(screen.getByRole('note')).toBeInTheDocument()
    await user.click(toggle)
    expect(screen.queryByRole('note')).toBeNull()

    await user.click(toggle)
    await user.click(screen.getByRole('button', { name: 'elsewhere' }))
    expect(screen.queryByRole('note')).toBeNull()
  })

  it('stays inside the screen at phone width: shifted left off the right edge, and below the "?"', async () => {
    Object.defineProperty(window, 'innerWidth', { value: 375, configurable: true })
    Object.defineProperty(window, 'innerHeight', { value: 700, configurable: true })
    const rect = { left: 340, right: 355, top: 100, bottom: 115, width: 15, height: 15, x: 340, y: 100, toJSON: () => ({}) } as DOMRect
    vi.spyOn(HTMLButtonElement.prototype, 'getBoundingClientRect').mockReturnValue(rect)
    vi.spyOn(HTMLElement.prototype, 'offsetWidth', 'get').mockReturnValue(300)
    vi.spyOn(Element.prototype, 'scrollHeight', 'get').mockReturnValue(200)
    try {
      const Term = await freshTerm()
      const user = userEvent.setup()
      render(
        <p>
          of <Term id="context_window" /> at once
        </p>,
      )
      await user.click(screen.getByRole('button', { name: /what is context window/i }))
      const note = screen.getByRole('note')
      expect(note.style.left).toBe('59px') // 375 − 16 gutter − 300 wide
      expect(note.style.top).toBe('121px') // 115 + 6 below the "?"
    } finally {
      vi.restoreAllMocks()
      Object.defineProperty(window, 'innerWidth', { value: 1024, configurable: true })
      Object.defineProperty(window, 'innerHeight', { value: 768, configurable: true })
    }
  })

  it('puts it above the "?" when there is more room there', async () => {
    Object.defineProperty(window, 'innerWidth', { value: 375, configurable: true })
    Object.defineProperty(window, 'innerHeight', { value: 700, configurable: true })
    const rect = { left: 20, right: 35, top: 600, bottom: 615, width: 15, height: 15, x: 20, y: 600, toJSON: () => ({}) } as DOMRect
    vi.spyOn(HTMLButtonElement.prototype, 'getBoundingClientRect').mockReturnValue(rect)
    vi.spyOn(HTMLElement.prototype, 'offsetWidth', 'get').mockReturnValue(300)
    vi.spyOn(Element.prototype, 'scrollHeight', 'get').mockReturnValue(200)
    try {
      const Term = await freshTerm()
      const user = userEvent.setup()
      render(
        <p>
          of <Term id="context_window" /> at once
        </p>,
      )
      await user.click(screen.getByRole('button', { name: /what is context window/i }))
      // 600 − 6 gap − 200 tall
      expect(screen.getByRole('note').style.top).toBe('394px')
    } finally {
      vi.restoreAllMocks()
      Object.defineProperty(window, 'innerWidth', { value: 1024, configurable: true })
      Object.defineProperty(window, 'innerHeight', { value: 768, configurable: true })
    }
  })

  it('bare shows only the "?", for a place that already prints the term', async () => {
    const Term = await freshTerm()
    render(
      <p data-testid="sentence">
        41.3 tok/s <Term id="tokens_per_sec" bare />
      </p>,
    )
    expect(screen.getByTestId('sentence')).toHaveTextContent(/^41\.3 tok\/s \?$/)
    expect(screen.getByRole('button', { name: en.term.whatIs('tok/s') })).toBeInTheDocument()
  })
})
