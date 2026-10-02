import { useEffect, useState } from 'react'
import { api } from '../api/client'
import type { DataDeleteResponse, HardwareResponse, Health, ModelsFolder, NotifyMode, UpdateCheckResponse } from '../api/types'
import { formatBytes } from '../components/Figure'
import { en } from '../copy/en'
import { forgetLocalSettings, useSettings } from '../state/settings'

const c = en.screens.settings

/** The order the notification mode is offered in: the everyday choice first. */
const modeOrder: NotifyMode[] = ['on', 'quiet', 'never']

/**
 * Settings (build-plan step 8): the Advanced toggle, where things live on
 * disk and a button that opens each (D-16), the version, the new-model
 * watch's own settings (build-plan step 10), checking for updates (step
 * 11), and "Your data" with the button that deletes everything the app
 * stored (step 12, D-68).
 */
export function Settings() {
  const { settings, setAdvanced, setWatch } = useSettings()
  const [health, setHealth] = useState<Health | null>(null)
  const [hw, setHw] = useState<HardwareResponse | null>(null)
  const [dataDir, setDataDir] = useState<string | null>(null)
  // Where Ollama says its models are (D-72); the profile's reading is only the fallback.
  const [folder, setFolder] = useState<ModelsFolder | null>(null)

  useEffect(() => {
    const ac = new AbortController()
    api
      .health(ac.signal)
      .then(setHealth)
      .catch(() => undefined)
    api
      .hardware(ac.signal)
      .then(setHw)
      .catch(() => undefined)
    api
      .modelsFolder(ac.signal)
      .then(setFolder)
      .catch(() => undefined)
    api
      .settings(ac.signal)
      .then((s) => setDataDir(s.data_dir))
      .catch(() => undefined)
    return () => ac.abort()
  }, [])

  return (
    <section className="screen" aria-labelledby="screen-title">
      <h1 id="screen-title">{c.title}</h1>

      <div className="setting">
        <label className="setting__label">
          <input
            type="checkbox"
            checked={settings.advanced}
            onChange={(e) => setAdvanced(e.target.checked)}
            aria-describedby="advanced-help"
          />{' '}
          {c.advancedLabel}
        </label>
        <p id="advanced-help" className="setting__help">
          {c.advancedHelp}
        </p>
      </div>

      <div className="settings-section">
        <h2>{c.notificationsTitle}</h2>
        <p className="screen__note">{c.notificationsHelp}</p>

        <div className="setting">
          <label className="setting__label">
            <input
              type="checkbox"
              checked={settings.watch.enabled}
              onChange={(e) => setWatch({ ...settings.watch, enabled: e.target.checked })}
              aria-describedby="watch-enabled-help"
            />{' '}
            {c.watchEnabledLabel}
          </label>
          <p id="watch-enabled-help" className="setting__help">
            {c.watchEnabledHelp}
          </p>
        </div>

        <fieldset className="purposes" disabled={!settings.watch.enabled}>
          <legend className="screen__lead">{c.watchModeLegend}</legend>
          <ul>
            {modeOrder.map((mode) => (
              <li key={mode}>
                <label>
                  <input
                    type="radio"
                    name="watch-mode"
                    checked={settings.watch.mode === mode}
                    onChange={() => setWatch({ ...settings.watch, mode })}
                  />{' '}
                  {c.watchMode[mode]}
                </label>
              </li>
            ))}
          </ul>
        </fieldset>
      </div>

      <div className="settings-section">
        <h2>{c.updatesTitle}</h2>
        <UpdateCheck />
      </div>

      <div className="settings-section">
        <dl>
          <div className="settings-row">
            <dt>{c.modelsFolder}</dt>
            <dd>
              <span>
                {folder?.path ? folder.path : hw?.profile ? hw.profile.storage.models_dir : c.modelsFolderUnknown}
                {folder?.path
                  ? folder.free_known
                    ? ` — ${c.freeSpace(formatBytes(folder.free_bytes))}`
                    : ` — ${c.freeSpaceUnknown}`
                  : hw?.profile?.storage.free_known
                    ? ` — ${c.freeSpace(formatBytes(hw.profile.storage.free_bytes))}`
                    : ` — ${c.freeSpaceUnknown}`}
              </span>
              {folder && !folder.known && folder.path ? <span className="screen__note"> {c.modelsFolderNotYet}</span> : null}
              <OpenButton open={() => api.openModelsDir()} />
            </dd>
          </div>
          <div className="settings-row">
            <dt>{c.dataFolder}</dt>
            <dd>
              <span>{dataDir || c.modelsFolderUnknown}</span>
              <OpenButton open={() => api.openDataDir()} />
            </dd>
          </div>
          <div className="settings-row">
            <dt>{en.app.title}</dt>
            <dd>{health ? `${c.version(health.version)} · ${c.versionPlatform(health.os, health.arch)}` : '…'}</dd>
          </div>
        </dl>
      </div>

      <div className="settings-section">
        <h2>{c.dataTitle}</h2>
        <p className="screen__note">{c.dataHelp}</p>
        <DeleteEverything />
      </div>
    </section>
  )
}

/**
 * "Delete everything" (build-plan step 12, ARCHITECTURE.md D-68): product
 * rule 5's button that says what it does — a first click shows exactly
 * what goes and what stays, a second one does it. The daemon quits once
 * it has answered, so the answer is the last thing this page shows.
 */
function DeleteEverything() {
  const [state, setState] = useState<'idle' | 'confirming' | 'deleting' | 'done' | 'failed'>('idle')
  const [result, setResult] = useState<DataDeleteResponse | null>(null)
  const [error, setError] = useState<string | null>(null)

  const run = () => {
    setState('deleting')
    setError(null)
    api
      .deleteEverything()
      .then((resp) => {
        forgetLocalSettings()
        setResult(resp)
        setState('done')
      })
      .catch((err: unknown) => {
        setState('failed')
        setError(err instanceof Error ? err.message : String(err))
      })
  }

  if (state === 'done' && result) {
    return (
      <div className="setting" role="status">
        <p className="notice">{c.deleted}</p>
        {result.kept.length > 0 ? (
          <>
            <p className="screen__note">{c.deletedKept}</p>
            <ul>
              {result.kept.map((k) => (
                <li key={k}>{k}</li>
              ))}
            </ul>
          </>
        ) : null}
        {result.problems && result.problems.length > 0 ? (
          <>
            <p className="notice notice--warning">{c.deleteProblems}</p>
            <ul>
              {result.problems.map((p) => (
                <li key={p}>{p}</li>
              ))}
            </ul>
          </>
        ) : null}
      </div>
    )
  }

  if (state === 'idle' || state === 'failed') {
    return (
      <div className="setting">
        <button type="button" className="button button--secondary" onClick={() => setState('confirming')}>
          {c.deleteEverything}
        </button>
        {state === 'failed' ? (
          <span className="notice notice--warning" role="alert">
            {c.deleteFailed(error ?? '')}
          </span>
        ) : null}
      </div>
    )
  }

  return (
    <div className="setting" role="group" aria-labelledby="delete-everything-title">
      <p id="delete-everything-title" className="setting__label">
        {c.deleteConfirmTitle}
      </p>
      <ul>
        {c.deleteWhat.map((w) => (
          <li key={w}>{w}</li>
        ))}
      </ul>
      <p className="setting__help">{c.deleteKeeps}</p>
      <p className="setting__help">{c.deleteCloses}</p>
      <button type="button" className="button" disabled={state === 'deleting'} onClick={run}>
        {state === 'deleting' ? c.deleting : c.deleteConfirm}
      </button>{' '}
      <button type="button" className="button button--secondary" disabled={state === 'deleting'} onClick={() => setState('idle')}>
        {c.deleteCancel}
      </button>
    </div>
  )
}

/**
 * "Check for updates" (build-plan step 11): one manual, on-click look at
 * the release feed — never on a timer (internal/update's own doc comment,
 * D-62). Three outcomes: up to date, a newer version with a link to it, or
 * — for a from-source ("dev") build — the latest release shown with no
 * claim about whether this build is behind it, since there is nothing
 * honest to compare.
 */
function UpdateCheck() {
  const [state, setState] = useState<'idle' | 'checking' | 'done' | 'failed'>('idle')
  const [info, setInfo] = useState<UpdateCheckResponse | null>(null)
  const [error, setError] = useState<string | null>(null)

  const check = () => {
    setState('checking')
    setError(null)
    api
      .checkUpdate()
      .then((resp) => {
        setInfo(resp)
        if (resp.checked) {
          setState('done')
        } else {
          setState('failed')
          setError(resp.error ?? '')
        }
      })
      .catch((err: unknown) => {
        setState('failed')
        setError(err instanceof Error ? err.message : String(err))
      })
  }

  return (
    <div className="setting">
      <button type="button" className="button button--secondary" disabled={state === 'checking'} onClick={check}>
        {state === 'checking' ? c.updatesChecking : c.updatesCheck}
      </button>
      {state === 'done' && info ? (
        <p className="screen__note">
          {info.current === 'dev' && info.latest ? (
            c.updatesDevBuild(info.latest)
          ) : info.update_available && info.latest ? (
            <>
              {c.updatesAvailable(info.latest)}{' '}
              <a href={info.url} target="_blank" rel="noreferrer">
                {c.updatesDownload}
              </a>
            </>
          ) : (
            c.updatesUpToDate(info.current)
          )}
        </p>
      ) : null}
      {state === 'failed' ? (
        <span className="notice notice--warning" role="alert">
          {c.updatesFailed(error ?? '')}
        </span>
      ) : null}
    </div>
  )
}

function OpenButton({ open }: { open: () => Promise<unknown> }) {
  const [state, setState] = useState<'idle' | 'opening' | 'failed'>('idle')
  const [error, setError] = useState<string | null>(null)
  const click = () => {
    setState('opening')
    setError(null)
    open()
      .then(() => setState('idle'))
      .catch((err: unknown) => {
        setState('failed')
        setError(err instanceof Error ? err.message : String(err))
      })
  }
  return (
    <>
      <button type="button" className="button button--secondary" disabled={state === 'opening'} onClick={click}>
        {state === 'opening' ? c.opening : c.open}
      </button>
      {state === 'failed' ? (
        <span className="notice notice--warning" role="alert">
          {c.openFailed(error ?? '')}
        </span>
      ) : null}
    </>
  )
}
