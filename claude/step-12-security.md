# Step 12: security and privacy review, full code review — closed

**Status (2026-09-27):** the findings are below, the fixes are in, and
`SECURITY.md` is written for the download page. Everything that can run on
Linux ran for real here, including the built binary driven through
Chromium. What still needs a Mac, a Windows PC or the next CI run is listed
at the end. The decisions are ARCHITECTURE.md D-64 to D-70.

Read: everything under `internal/` and `ui/src/`, `cmd/advisor`, CLAUDE.md
rule 7, PRD §21, ARCHITECTURE.md end to end, the CI workflow and the
packaging scripts.

## Findings, most severe first

| # | Severity | Finding | Fix |
|---|---|---|---|
| F-1 | High | **Any page on another port of this computer could use the API.** D-12's Origin check admitted any `http://127.0.0.1:*` or `http://localhost:*` origin (its own test asserted `http://localhost:5173` → 200). A page served by any local program — a dev server, another local web app — could POST and PUT: start a download, start an install, change settings, remove a model. A cross-site `<img>`/`<script>` reached GET endpoints (they carry no Origin). Nothing stopped another site framing the app and steering a click. | D-67: Origin must equal the daemon's own origin exactly; `Sec-Fetch-Site` must be same-origin/none on `/api/`; CSP with `connect-src 'self'` and `frame-ancestors 'none'`, `X-Frame-Options: DENY`, `nosniff`, `no-referrer`, COOP/CORP; body cap. |
| F-2 | High | **The Ollama installer was run unchecked.** `downloadFile` checked the first URL's host, then followed any redirect to any host, including https→http (Go follows downgrades). No checksum was checked, though Ollama publishes `sha256sum.txt` with every release (D-29 said it didn't). The file went to a predictable name in the temp folder, which on Linux is shared: another user could plant a symlink there, or swap the archive before `tar`. | D-66: pin the release from the redirect chain, read its `sha256sum.txt`, download into a private `advisor-ollama-*` folder with `O_EXCL`/0600, hash on the way, delete and refuse on mismatch or no checksum, all through `egress`. |
| F-3 | Medium | **Four allow-lists and twelve hand-built HTTP clients.** A thirteenth call used `http.DefaultClient`. The update check followed any redirect, and nothing stopped a new package from calling anywhere. | D-64: `internal/egress`, with `hosts.go` as the one list, checked at the transport on every hop. `internal/archtest` fails the build on any other client, dial, listener, `InsecureSkipVerify`, credential header, cookie jar or download tool. |
| F-4 | Medium | **`OLLAMA_HOST` was honoured for any host.** A remote Ollama would have been sent the benchmark (and its timings recorded against this computer's hardware), and an Ollama the advisor started inherited a LAN-wide `OLLAMA_HOST`. | D-65: only loopback/`localhost` is honoured, and anything else is set aside with a sentence in `Detect`. `egress.Local` refuses non-loopback at the socket, and an Ollama the advisor starts gets a loopback `OLLAMA_HOST`. |
| F-5 | Medium | **The daily watch made first contact on its own.** 30 seconds after every start, including the very first, it refreshed the model list (several MB) and read the public sources. It did this before the person had clicked "Fetch the model list", against product rule 5 and D-54. | D-68: a scheduled check does nothing until the list has been fetched once by the person (`ErrNotFetchedYet`); test counts zero requests. |
| F-6 | Medium | **PowerShell injection in Windows notifications.** PowerShell ends a single-quoted literal at ‘ ’ ‚ ‛ as well as `'`. `psString` escaped only `'`, and the app's own copy uses ’, so a notification containing "advisor’s" broke out of its literal. Today only templated text reaches it, so it fails rather than being exploitable, but any future remote string would have been code execution. | All four quote characters are doubled; test. `notify-send` gets `--`. |
| F-7 | Medium | **No "delete everything"** (D-13 promised it for step 12), and the data was readable by other local users: folder 0755, database 0644. It holds the computer's name, hardware, installed models and every test. | D-68: 0700/0600 (tightened on existing installs, only for the advisor's own folder). `POST /api/data/delete` plus a two-step Settings button: refused while anything runs; forgets start-at-login (without stopping itself), the AUMID and the adapter's files; deletes exactly the database files, then the folder if empty; says what it kept and why; then quits. The page clears its localStorage copy. |
| F-8 | Low | `POST /api/models/pull` accepted any string, which handed free text to Ollama's registry (or to Hugging Face through `hf.co/…` tags). | Only a tag the curated catalogue lists (`not_in_catalogue` otherwise). |
| F-9 | Low | `backend.GenerateRequest` had `Prompt string`, `System string` and `map[string]any` options. Only the harness used it, correctly, but nothing made that the only possibility. | D-65: `suite.Prompt` (sealed; only the embedded suite makes one), numeric `suite.Options`, no `System`. `archtest` pins the shape and that only `internal/bench` calls `Generate`. |
| F-10 | Low | Links from other people's data became `<a href>` unchecked: the update feed's `html_url`, and a Hugging Face result's own source link (written by whoever published the result). React 19 blocks `javascript:`, but any other scheme or host was shown. | The update link is kept only when it's this project's release page; hfevals keeps only https links; `PublicFigure` renders only https links. |
| F-11 | Low | **Supply chain.** UI versions were ranges and install scripts were allowed. CI actions used moving tags, GoReleaser was `~> v2`, and `appimagetool` came from the moving `continuous` build, unverified, executed in the release job. The DMG, installer and AppImage had no checksums. | D-69: exact versions and `.npmrc` (`save-exact`, `ignore-scripts`); actions pinned to SHAs; GoReleaser v2.18.2; appimagetool 1.9.1 + SHA-256; `.sha256` beside each packaged file; a `security` job (`go mod verify`, govulncheck ×3 OSes, `npm audit`) that the release waits for. |
| F-12 | Low | **The docs said the wrong thing.** README said the only traffic is "a lookup to Hugging Face" (it's also Epoch, ollama.com/GitHub, api.github.com, and daily when the watch is on). INSTALL.md said downloaded models live in the app's data folder (they're Ollama's). | README and INSTALL.md corrected; `SECURITY.md` has the complete list. |
| F-13 | Low | Unbounded `io.ReadAll` of Ollama's answers; no server-wide body cap, `MaxHeaderBytes` or idle timeout. | 32 MiB cap on Ollama answers (64 KiB on error bodies); 1 MiB body cap; `MaxHeaderBytes` 64 KiB; `IdleTimeout` 2 min. |
| F-14 | Info | CLAUDE.md's copy rule (no glossary term without its explainer) was enforced for Go reasons but not the UI. One bare case: Settings' Advanced help ("quantization names, tokens per second"). | `ui/src/copy/glossaryRule.test.ts`; the help rewritten. |
| F-15 | Info | D-11's dependency direction, D-8 (only the harness asks a model anything) and D-22/D-26's named dependencies were stated but not tested. All held. | `internal/archtest` (imports, model, deps tests). |

## Checked and sound (no change)

- The loopback bind (`LoopbackHost` const, `tcp4`, `Serve` refusing
  non-loopback) and the Host check: as D-12 describes.
- No CORS header anywhere. That is now a test.
- The Hugging Face client: range reads, a whole-file answer refused,
  redirects pinned to Hugging Face, no token. The public-data fetcher: its
  host list, no cookies. Both are now also bounded by egress.
- No telemetry, crash reporting or analytics code path. The update check
  is manual only. The benchmark stores timings and counts, never a model's
  answer text.
- Hardware detection needs no root and runs local tools only. Autostart
  and the AUMID are HKCU/user-only. macOS notifications escape AppleScript
  correctly.
- govulncheck v1.8.0 (DB 2026-09-24) for linux, darwin and windows: no
  vulnerabilities. `npm audit`: 0.

## What ran, where

In this session (Linux, Go 1.27.1, Node 22):

- `go test ./...` with the UI embedded and with `-tags noui`: green.
  `go vet` for linux, darwin and windows: clean. `gofmt`: clean.
- UI: `tsc -b`, `vitest`: 17 files green (`npm test`).
- **The built binary in Chromium** (Playwright, Chromium 1194):
  - Every screen (`/`, `/recommend`, `/models`, `/benchmarks`,
    `/computer`, `/ollama`, `/watch`, `/settings`) loaded under the new
    CSP with no violation and no console error.
  - An attacking page served from `127.0.0.1:27300` tried a no-cors
    `fetch` POST to `/api/data/delete`, a form POST, a cross-origin read
    of `/api/settings`, an `<img>` GET of `/api/update/check`, and an
    `<iframe>` of `/settings`. The daemon refused every one: "rejected
    cross-origin request" ×3 and "rejected cross-site request
    sec_fetch_site=same-site" in its log, and the browser's "refused to
    connect" for the frame. The database survived.
  - Then Settings → Delete everything → confirm: the data folder was
    gone, localStorage cleared, and the daemon exited.
  - The data folder on the real binary was 0700, and `advisor.db`,
    `-wal` and `-shm` were 0600.
- **Mutation check of the scans:** planting `http.Get`, `&http.Client{}`,
  a `Generate` call, `"Authorization"` and `"curl …"` in
  `internal/server/update.go` failed `internal/archtest` on each line.
  Reverted.
- **Ollama's real release** (curl, 2026-09-27):
  `ollama.com/download/Ollama.dmg` → 307 → `…/releases/latest/download/…`
  → 302 → `…/releases/download/v0.34.4/…` → 302 →
  `release-assets.githubusercontent.com`. `sha256sum.txt` lists
  `./Ollama.dmg`, `./OllamaSetup.exe` and `./ollama-linux-amd64.tar.zst`.
  The tests' fake reproduces exactly that chain.

Not verified here — for `scripts/verify.command`, the Windows PC and the
next CI run:

- A real **Install Ollama** end to end on macOS and Windows (the pinned
  download, the checksum, then the installer opening).
- **Windows:** `autostart.Forget` and `winapp.UnregisterIdentity` against
  the real registry (the logic is tested through the fake registry), and
  a toast whose text contains ’.
- **CI:** the pinned action SHAs (resolved with `git ls-remote` on
  2026-09-27), the `security` job, the smoke test's new Origin,
  Sec-Fetch-Site and CSP checks, and the `.sha256` uploads, on the next
  push and tag.
- **`make dev`:** the Vite proxy's `origin` header was read in Vite 8.3's
  proxy code, not run.

## Open, for Itay

- Turn on **Private vulnerability reporting** in the GitHub repo's
  Security settings. `SECURITY.md` sends reporters there.
- **Pin Inno Setup** in `package-windows` (`choco install innosetup
  --version …`). The current version couldn't be read from this session.
- **The watch is on by default** (step 10's decision). It no longer makes
  first contact, and `SECURITY.md` discloses the daily requests. Whether
  it should also be off until the person turns it on is a product
  decision, not a fix.
- `backends.installed_version` is never written. The release tag the
  verified install now knows (`download` returns it) could fill it. That's
  not a security issue.
- `scripts/probe0` builds its own HTTP client to `OLLAMA_HOST`. It's a dev
  tool, not shipped, and outside `internal/archtest`'s scan.
