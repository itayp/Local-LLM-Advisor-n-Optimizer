import { useEffect, useId, useLayoutEffect, useRef, useState, type ReactNode } from 'react'
import { api } from '../api/client'
import type { SpeedNeedsResponse } from '../api/types'
import type { GlossaryTermId } from '../copy/glossary'
import { glossary } from '../copy/glossary'
import { en } from '../copy/en'

// The tokens_per_sec explainer's extra content (backlog item (b)) is read
// once and shared by every <Term id="tokens_per_sec">, however many the
// page has open — GET /api/speed-needs is the same curated table on every
// call, so there is nothing to invalidate.
let speedNeedsCache: Promise<SpeedNeedsResponse> | null = null
function loadSpeedNeeds(): Promise<SpeedNeedsResponse> {
  if (!speedNeedsCache) speedNeedsCache = api.speedNeeds()
  return speedNeedsCache
}

const termCopy = en.term

// The popover's distance from the "?", and from the edge of the screen (the
// same 16px gutter the page keeps at phone width); MIN_ROOM is the shortest
// it will be squeezed to before the page has to scroll.
const POPOVER_GAP = 6
const POPOVER_GUTTER = 16
const MIN_ROOM = 120

/**
 * Term shows a technical word with its one-line explainer a tap away
 * (CLAUDE.md rule 2: no term from the glossary list appears without one).
 * Written once per term in copy/glossary.ts; a screen using a glossary
 * term renders it through this component instead of restating the
 * explanation itself.
 *
 * The explainer is a popover, not part of the sentence (backlog p): opened,
 * it floats over the page next to the "?" — position: fixed, placed from the
 * "?"'s own rectangle and kept inside the viewport with a gutter, so it
 * never pushes or splits the words around it and is not clipped by a table's
 * horizontal scroll. Below the "?" when it fits there, above it when that
 * has more room, and scrollable when the room is short (the speed table).
 * It closes on Escape (focus goes back to the "?"), on a tap outside, and on
 * the "?" again. It sits right after the "?" in the DOM, so Tab reaches it
 * (it is focusable, so its scrolling works from the keyboard too).
 *
 * A toggle button, not <details>/<summary>: Term appears inline inside
 * ordinary sentences (a <p>), and <details> is not valid content there —
 * a button and a span are. For the same reason every node this renders,
 * including the popover and tokens_per_sec's extra table, stays phrasing
 * content (<span>, never <p>/<div>/<ul>): CSS gives the popover its box and
 * the rows their own line, so the markup remains valid wherever a screen
 * drops <Term> into a sentence.
 *
 * `bare` shows only the "?", for a place where the term itself is already
 * printed right beside it (a speed figure ends in "tok/s" — repeating the
 * word would read "45 tok/s tok/s").
 */
export function Term({ id, children, bare }: { id: GlossaryTermId; children?: ReactNode; bare?: boolean }) {
  const g = glossary[id]
  const [open, setOpen] = useState(false)
  const [speedNeeds, setSpeedNeeds] = useState<SpeedNeedsResponse | null>(null)
  const [speedNeedsFailed, setSpeedNeedsFailed] = useState(false)
  const rootRef = useRef<HTMLSpanElement>(null)
  const toggleRef = useRef<HTMLButtonElement>(null)
  const popoverRef = useRef<HTMLSpanElement>(null)
  const popoverId = useId()

  useEffect(() => {
    if (id !== 'tokens_per_sec' || !open || speedNeeds || speedNeedsFailed) return
    let cancelled = false
    loadSpeedNeeds()
      .then((r) => {
        if (!cancelled) setSpeedNeeds(r)
      })
      .catch(() => {
        if (!cancelled) setSpeedNeedsFailed(true)
      })
    return () => {
      cancelled = true
    }
  }, [id, open, speedNeeds, speedNeedsFailed])

  // Escape and a tap outside close it.
  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return
      setOpen(false)
      // Give focus back to the "?" only when it was here (or nowhere): never
      // pull it out of a field the person is typing in.
      const active = document.activeElement
      if (!active || active === document.body || rootRef.current?.contains(active)) toggleRef.current?.focus()
    }
    const onPointer = (e: Event) => {
      if (e.target instanceof Node && !rootRef.current?.contains(e.target)) setOpen(false)
    }
    document.addEventListener('keydown', onKey)
    document.addEventListener('pointerdown', onPointer)
    return () => {
      document.removeEventListener('keydown', onKey)
      document.removeEventListener('pointerdown', onPointer)
    }
  }, [open])

  // Placement: before paint, and again whenever the page scrolls or resizes
  // or the content (the speed table arriving) changes the popover's height.
  useLayoutEffect(() => {
    if (!open) return
    const place = () => {
      const pop = popoverRef.current
      const anchor = toggleRef.current
      if (!pop || !anchor) return
      const vw = window.innerWidth
      const vh = window.innerHeight
      const a = anchor.getBoundingClientRect()
      // The "?" scrolled out of sight: the explanation has nothing to point at.
      if (a.height > 0 && (a.bottom < 0 || a.top > vh)) {
        setOpen(false)
        return
      }
      const width = pop.offsetWidth
      const left = Math.max(POPOVER_GUTTER, Math.min(a.left, vw - POPOVER_GUTTER - width))
      const below = vh - a.bottom - POPOVER_GAP - POPOVER_GUTTER
      const above = a.top - POPOVER_GAP - POPOVER_GUTTER
      const wanted = pop.scrollHeight
      const placeAbove = wanted > below && above > below
      const room = Math.max(placeAbove ? above : below, MIN_ROOM)
      const height = Math.min(wanted, room)
      pop.style.left = `${left}px`
      pop.style.maxHeight = `${room}px`
      pop.style.top = `${placeAbove ? Math.max(POPOVER_GUTTER, a.top - POPOVER_GAP - height) : a.bottom + POPOVER_GAP}px`
    }
    place()
    window.addEventListener('resize', place)
    window.addEventListener('scroll', place, true)
    return () => {
      window.removeEventListener('resize', place)
      window.removeEventListener('scroll', place, true)
    }
  }, [open, speedNeeds, speedNeedsFailed])

  const name = termCopy.whatIs(g.term)
  return (
    <span className="term" ref={rootRef}>
      {bare ? null : (children ?? g.term)}
      <button
        type="button"
        className="term__toggle"
        ref={toggleRef}
        aria-label={name}
        aria-expanded={open}
        aria-controls={open ? popoverId : undefined}
        onClick={() => setOpen((o) => !o)}
      >
        ?
      </button>
      {open ? (
        <span
          className={`term__explain${id === 'tokens_per_sec' ? ' term__explain--wide' : ''}`}
          id={popoverId}
          role="note"
          aria-label={name}
          tabIndex={0}
          ref={popoverRef}
          // Out of the line's flow by construction, not by stylesheet alone: the
          // popover is never a part of the sentence it was opened from.
          style={{ position: 'fixed' }}
        >
          {g.explain}
          {id === 'tokens_per_sec' ? <SpeedNeedsExplainer data={speedNeeds} failed={speedNeedsFailed} /> : null}
        </span>
      ) : null}
    </span>
  )
}

/** What a speed is good for, per purpose (GET /api/speed-needs, D-58). */
function SpeedNeedsExplainer({ data, failed }: { data: SpeedNeedsResponse | null; failed: boolean }) {
  const c = en.speedNeeds
  const purposeLabel = en.screens.recommend.purposes as Record<string, string>
  if (failed) return <span className="term__speedneeds-note">{c.failed}</span>
  if (!data) return <span className="term__speedneeds-note">{c.loading}</span>
  return (
    <span className="term__speedneeds" data-testid="speed-needs">
      <span className="term__speedneeds-heading">{c.heading}</span>
      <span className="term__speedneeds-intro">{c.intro}</span>
      <span className="term__speedneeds-intro" data-testid="speed-needs-provisional">
        {c.provisional}
      </span>
      {data.purposes.map((row) => {
        const label = purposeLabel[row.purpose] ?? row.purpose
        return (
          <span className="term__speedneeds-row" key={row.purpose}>
            {row.stream ? (
              <>
                <span className="term__speedneeds-line">
                  {c.streamLine(
                    label,
                    row.stream.excellent * data.words_per_token,
                    row.stream.good * data.words_per_token,
                    row.stream.usable * data.words_per_token,
                  )}
                </span>
                <span className="term__speedneeds-line">{c.waitLine(label, row.wait_s.excellent, row.wait_s.good, row.wait_s.usable)}</span>
              </>
            ) : (
              <span className="term__speedneeds-line">{c.stepLine(label, row.wait_s.excellent, row.wait_s.good, row.wait_s.usable)}</span>
            )}
          </span>
        )
      })}
    </span>
  )
}
