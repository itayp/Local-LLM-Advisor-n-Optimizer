import { describe, expect, it } from 'vitest'
import { en } from './en'

/**
 * CLAUDE.md's copy rule, held over every string the UI shows: no term from
 * {VRAM, quantization, GGUF, KV cache, context window, tokens/sec, offload}
 * without its explainer (product rule 2). A string that names one is
 * allowed only
 *   - as a table row's label or explanation, where its `explain` sibling is
 *     shown beside it (the Advanced panels), or
 *   - at a place the screens render inside <Term>, which gives it the
 *     glossary's one-line explainer — listed below, with the screen that
 *     does it, so the list is the review.
 */
const renderedInsideTerm: Record<string, string> = {
  'en.screens.modelDetail.quant': 'ModelDetail.tsx: <Term id="quantization">{c.quant}</Term>',
  'en.screens.models.advanced.quantization': 'Models.tsx: <Term id="quantization">{c.advanced.quantization}</Term>',
  'en.onboarding.ollama.leadTermVram': 'OllamaStep.tsx: <Term id="vram">{c.leadTermVram}</Term>',
  'en.onboarding.tryit.leadTermTokens': 'TryIt.tsx: <Term id="tokens_per_sec">{c.leadTermTokens}</Term>',
}

const glossaryTerm = /\bVRAM\b|quantiz|\bGGUF\b|KV cache|context window|\btok(ens)?\s*\/\s*s(ec)?\b|tokens per second|\boffload/i

function bareTerms(): string[] {
  const out: string[] = []
  const walk = (v: unknown, path: string, parent: Record<string, unknown> | null) => {
    if (typeof v === 'string') {
      if (glossaryTerm.test(v) && !(parent && 'explain' in parent) && !(path in renderedInsideTerm)) {
        out.push(`${path} = ${JSON.stringify(v)}`)
      }
    } else if (typeof v === 'function') {
      let r: unknown
      try {
        r = (v as (...a: unknown[]) => unknown)('x', 'y', 'z', 'w')
      } catch {
        return
      }
      if (typeof r === 'string' && glossaryTerm.test(r) && !(path in renderedInsideTerm)) out.push(`${path}() = ${JSON.stringify(r)}`)
    } else if (Array.isArray(v)) {
      v.forEach((x, i) => walk(x, `${path}.${i}`, null))
    } else if (v && typeof v === 'object') {
      for (const [k, x] of Object.entries(v)) walk(x, `${path}.${k}`, v as Record<string, unknown>)
    }
  }
  walk(en, 'en', null)
  return out
}

describe('the copy rule', () => {
  it('never shows a technical term without its explainer', () => {
    expect(bareTerms()).toEqual([])
  })

  it('checks every place it exempts', () => {
    // An exemption for a key that no longer exists is a stale review line.
    for (const path of Object.keys(renderedInsideTerm)) {
      const v = path
        .split('.')
        .slice(1)
        .reduce<unknown>((o, k) => (o && typeof o === 'object' ? (o as Record<string, unknown>)[k] : undefined), en)
      expect(typeof v, path).toBe('string')
    }
  })
})
