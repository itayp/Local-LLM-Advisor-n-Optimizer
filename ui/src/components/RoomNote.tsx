import { useEffect, useState } from 'react'
import { Link } from 'react-router'
import { api } from '../api/client'
import type { RoomCheck } from '../api/types'
import { en } from '../copy/en'
import { formatDownload } from '../onboarding/format'
import { useAdvanced } from '../state/settings'
import { Figure } from './Figure'

const c = en.room

/** What a download is, for the check: a curated model by its Ollama tag, or a runtime's own installer. */
export type RoomTarget = { kind: 'pull'; tag: string } | { kind: 'install'; backend: string }

/** Where a code in RoomCheck.actions leads. P2-7 adds "keep_on_another_drive". */
const actionLinks: Record<string, string> = {
  remove_models: '/models',
}

/**
 * RoomNote is the one place a download's free-space check is shown, beside
 * every button that starts a download (product rule 5: the button's cost,
 * before the click). It asks the daemon, which reads the drive now, and
 * prints the daemon's sentence — with both numbers — plus the actions the
 * daemon names. A check that cannot be made says so and never blocks.
 *
 * onResult hands the check to the screen, which may switch its button off on
 * `not_enough` (the daemon would refuse the click anyway). quiet hides a
 * check that found nothing to say (room, or unknown), for a card with no
 * button of its own. refreshKey asks again (after a removal, say).
 */
export function RoomNote({
  target,
  onResult,
  quiet = false,
  refreshKey,
}: {
  target: RoomTarget
  onResult?: (r: RoomCheck | null) => void
  quiet?: boolean
  refreshKey?: unknown
}) {
  const advanced = useAdvanced()
  const [check, setCheck] = useState<RoomCheck | null>(null)
  const [failed, setFailed] = useState(false)
  const key = target.kind === 'pull' ? `pull:${target.tag}` : `install:${target.backend}`

  useEffect(() => {
    const ac = new AbortController()
    setCheck(null)
    setFailed(false)
    const ask = target.kind === 'pull' ? api.pullCheck(target.tag, ac.signal) : api.installCheck(target.backend, ac.signal)
    ask
      .then((r) => {
        // An answer that is not a check (a daemon too old to have one) is no answer.
        if (!r || typeof r.message !== 'string' || !r.verdict) throw new Error('not a room check')
        setCheck({ ...r, actions: r.actions ?? [] })
        onResult?.(r)
      })
      .catch((err: unknown) => {
        if ((err as { name?: string })?.name === 'AbortError') return
        setFailed(true)
        onResult?.(null)
      })
    return () => ac.abort()
    // onResult is the screen's setter; asking again for it would loop.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key, refreshKey])

  if (failed) return quiet ? null : <p className="screen__note room room--unknown">{c.unavailable}</p>
  if (!check) return quiet ? null : <p className="screen__note room room--checking" role="status">{c.checking}</p>
  if (quiet && (check.verdict === 'enough' || check.verdict === 'unknown')) return null

  const bad = check.verdict === 'not_enough' || check.verdict === 'low'
  return (
    <div
      className={bad ? 'notice notice--warning room' : 'screen__note room'}
      data-testid="room-note"
      data-verdict={check.verdict}
      role={check.verdict === 'not_enough' ? 'alert' : undefined}
    >
      <p>
        {check.verdict === 'not_enough' ? <strong>{c.notEnoughTitle}. </strong> : null}
        {check.message}
      </p>
      {check.actions.length > 0 ? (
        <p>
          {check.actions.map((a, i) =>
            actionLinks[a] && (c.actions as Record<string, string>)[a] ? (
              <span key={a}>
                {i > 0 ? ' · ' : ''}
                <Link to={actionLinks[a]}>{(c.actions as Record<string, string>)[a]}</Link>
              </span>
            ) : null,
          )}
        </p>
      ) : null}
      {advanced ? (
        <dl className="facts">
          {check.need ? (
            <>
              <dt>{c.advancedNeeds}</dt>
              <dd>
                <Figure value={formatDownload(check.need.value)} source={check.need.source} />
              </dd>
            </>
          ) : null}
          {check.free_known ? (
            <>
              <dt>{c.advancedFree}</dt>
              <dd>{formatDownload(check.free_bytes)}</dd>
            </>
          ) : null}
          {check.left ? (
            <>
              <dt>{c.advancedLeft}</dt>
              <dd>
                <Figure value={formatDownload(check.left.value)} source={check.left.source} />
              </dd>
            </>
          ) : null}
        </dl>
      ) : null}
    </div>
  )
}
