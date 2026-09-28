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

describe('the picture purpose', () => {
  // "Vision" is the field's word, not the customer's: the purpose is named for
  // what it does (backlog o). If the word ever comes back into a string the
  // person reads, it needs a glossary entry and a <Term> — this test is the
  // reminder.
  it('never shows the word "vision" in the UI copy', () => {
    const found: string[] = []
    const walk = (v: unknown, path: string) => {
      if (typeof v === 'string') {
        if (/\bvision\b/i.test(v)) found.push(`${path} = ${JSON.stringify(v)}`)
      } else if (typeof v === 'function') {
        try {
          const r = (v as (...a: unknown[]) => unknown)('x', 'y', 'z', 'w')
          if (typeof r === 'string' && /\bvision\b/i.test(r)) found.push(`${path}() = ${JSON.stringify(r)}`)
        } catch {
          /* not a plain string builder */
        }
      } else if (v && typeof v === 'object') {
        for (const [k, x] of Object.entries(v)) walk(x, `${path}.${k}`)
      }
    }
    walk(en, 'en')
    expect(found).toEqual([])
  })

  it('every purpose has a label and a one-line description, in both lists', () => {
    const keys = ['chat', 'writing', 'coding', 'reasoning', 'long_context', 'vision', 'agentic'] as const
    for (const k of keys) {
      for (const text of [en.screens.recommend.purposes[k], en.onboarding.purposes.labels[k]]) expect(text.length).toBeGreaterThan(3)
      for (const text of [en.screens.recommend.purposeDescriptions[k], en.onboarding.purposes.descriptions[k]]) {
        expect(text.length).toBeGreaterThan(10)
        expect(text).not.toContain('\n')
        expect(text).toMatch(/[.!]$/)
      }
    }
    expect(en.screens.recommend.purposes.vision).toBe('Understanding pictures and screenshots')
    expect(en.onboarding.purposes.labels.vision).toBe(en.screens.recommend.purposes.vision)
    expect(en.onboarding.purposes.descriptions.vision).toBe(
      'Describe a photo, read the text in a screenshot or scanned page, explain a chart. You attach the picture in your chat app. It does not create images.',
    )
  })
})
