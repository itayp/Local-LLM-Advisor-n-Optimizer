import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { en } from '../copy/en'
import { publicEntry } from '../test/publicFixtures'
import { PublicFigure } from './PublicFigure'

const c = en.publicData

describe('PublicFigure', () => {
  it('links to the source when the link is an https address', () => {
    render(<PublicFigure entry={publicEntry()} advanced={false} />)
    const link = screen.getByRole('link', { name: c.readAtSource })
    expect(link).toHaveAttribute('href', 'https://huggingface.co/datasets/lmarena-ai/leaderboard-dataset')
    expect(link).toHaveAttribute('rel', expect.stringContaining('noreferrer'))
  })

  // A public value's link is someone else's words (D-67): anything but an
  // https address is not a link at all.
  it.each(['javascript:alert(1)', 'http://example.com/results', 'data:text/html,hi', 'not a url'])('shows no link for %s', (url) => {
    const e = publicEntry()
    render(<PublicFigure entry={{ ...e, value: { ...e.value, origin: { ...e.value.origin, url } } }} advanced={false} />)
    expect(screen.queryByRole('link')).toBeNull()
  })
})
