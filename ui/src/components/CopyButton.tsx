import { useEffect, useRef, useState } from 'react'
import { en } from '../copy/en'

const c = en.copyButton

/** How long "Copied" (or the fallback note) stays up. */
const FLASH_MS = 2000

type State = 'idle' | 'copied' | 'failed'

/**
 * CopyButton puts `value` — a model's exact name, the thing a person pastes
 * into a chat app — on the clipboard. It says "Copied" for two seconds, then
 * goes back to "Copy".
 *
 * When the clipboard is refused (a browser that blocks it, an insecure
 * origin) it does not raise an alert: it tries the old selection-based copy,
 * and if that is refused too it shows a short muted note beside the button
 * and leaves the name where it is, selectable. Nothing else changes.
 *
 * The visible word is just "Copy"; its spoken name carries the value, so a
 * table of them is not a table of identical buttons (en.copyButton.labelFor).
 * One implementation for every screen that shows a name: Recommend's cards,
 * the model's own page, Benchmarks, Models, onboarding's "Use it".
 */
export function CopyButton({ value, className }: { value: string; className?: string }) {
  const [state, setState] = useState<State>('idle')
  const timer = useRef<number | undefined>(undefined)

  useEffect(() => () => window.clearTimeout(timer.current), [])

  const flash = (next: State) => {
    setState(next)
    window.clearTimeout(timer.current)
    timer.current = window.setTimeout(() => setState('idle'), FLASH_MS)
  }

  const copy = async () => {
    let ok: boolean
    try {
      await navigator.clipboard.writeText(value)
      ok = true
    } catch {
      // Refused, or there is no Clipboard API at all: the older way, then a note.
      ok = copyBySelection(value)
    }
    flash(ok ? 'copied' : 'failed')
  }

  return (
    <>
      <button
        type="button"
        className={className ?? 'link-button'}
        aria-label={state === 'copied' ? c.copied : c.labelFor(value)}
        onClick={() => void copy()}
      >
        {state === 'copied' ? c.copied : c.label}
      </button>
      {state === 'failed' ? (
        <span className="screen__note copy-note" role="status">
          {' '}
          {c.failed}
        </span>
      ) : null}
    </>
  )
}

/** The pre-Clipboard-API way: select the text of a throwaway field and ask the page to copy it. */
function copyBySelection(value: string): boolean {
  try {
    const field = document.createElement('textarea')
    field.value = value
    field.setAttribute('readonly', '')
    field.style.position = 'fixed'
    field.style.opacity = '0'
    document.body.appendChild(field)
    field.select()
    const ok = typeof document.execCommand === 'function' && document.execCommand('copy')
    document.body.removeChild(field)
    return ok
  } catch {
    return false
  }
}
