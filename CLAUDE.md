# CLAUDE.md — Local LLM Advisor & Optimizer

The working reference for every session in this repo. Read it first, then
`ARCHITECTURE.md` (the decisions and why), then the step you are on in
`BUILD_PLAN.md`. The PRD is `PRD — Local LLM Advisor & Optimizer.md`.

**Who this is for, in one line:** someone who has heard you can run AI on
their own computer, owns a laptop or a gaming PC, and has no idea what a GGUF
is. Itay's machines are the test fleet, not the customer.

## Product rules

Verbatim from `BUILD_PLAN.md`. They are the spec; when code and a rule
disagree, the code is wrong.

1. **The user never needs a terminal.** A step that needs one is a step the
   product has failed.
2. **Plain language first.** Every technical term the UI shows has a one-line
   explainer a tap away; the technical columns live behind an "Advanced"
   toggle, off by default.
3. **Every recommendation says why**, in words the user can act on ("fits your
   graphics card with room for long documents"), and what it costs ("a 5 GB
   download").
4. **Estimated is never dressed as measured.** Two visual treatments,
   everywhere, always. A measurement replaces an estimate the moment it exists.
5. **Nothing changes on the machine without a click on a button that says what
   it will do** — "Download 5 GB", "Start Ollama", "Run a two-minute test". No
   automatic model switching (PRD §12).
6. **Weak hardware is a tier, not an error.** A laptop with no graphics card
   gets an honest answer ("small models only, roughly a few words a second"),
   not a failure screen.
7. **The daemon binds to 127.0.0.1 only.** No prompt, no file, no usage data
   leaves the machine.
8. **Counts and model names in prose rot.** The catalogue is data; the docs say
   "the curated families", never "the 12 families".

## Rules the code enforces (do not weaken them)

- **Rule 7 is a constant.** `server.LoopbackHost` is `const`; `server.Listen`
  takes only a port; `Serve` refuses a non-loopback listener; every request
  needs a loopback `Host` header, a browser's `Origin` must be the daemon's
  own origin exactly, and `Sec-Fetch-Site` must be same-origin on the API
  (ARCHITECTURE.md D-67). There is no bind flag and there must never be one.
  Tests: `internal/server/server_test.go`.
- **Rule 7's other half is a package.** Every connection to another
  computer is made by a client from `internal/egress` (`egress.Client(purpose)`),
  whose transport refuses anything but HTTPS to a host
  `internal/egress/hosts.go` lists for that purpose — that file is the
  audit (D-64). The runtime is reached only through `egress.Local`, which
  dials loopback addresses only. `internal/archtest` fails the build if any
  other package builds a client, dials, listens, turns off TLS checks,
  sends a credential or names a download tool. One purpose,
  `ModelSearch`, carries what the person typed or picked, and only on
  their click (D-75, built in P2-8). Every other purpose's requests are a
  function of the data files and the version.
- **The benchmark sends the suite's text and nothing else.** A benchmark
  prompt is a `suite.Prompt`, which only the embedded suite can make.
  `backend.GenerateRequest` has no free-text field, and only
  `internal/bench` calls `Generate` (D-8, D-65; `internal/archtest`).
- **The chat sends the person's turns to a model on this computer, and
  keeps nothing** (D-74, built in P2-4; until then there is no chat). A
  turn is a `conversation.Turn`, made only by decoding the chat page's
  request, which cannot be printed, logged or marshalled.
  `backend.ChatRequest` carries turns and numbers only, and only
  `internal/server/chat.go` calls `Chat`. Nothing is stored or measured.
  The benchmark never sees a turn and the chat never sends the suite.
- **A model on this computer means its weights run here** (D-73, built in
  P2-4). Every path that sends text to a model refuses a remote one
  (Ollama's cloud models: `remote_host`), and the Ollama adapter names
  the model with Ollama's `:local` suffix so that Ollama refuses it too.
- **A download is a curated tag or a file the advisor resolved itself.**
  `POST /api/models/pull` takes an `ollama_tag` the curated catalogue
  lists, or a `resolved` id the server issued for a Hugging Face file it
  listed and read the header of during this run (D-65, D-75; the id path
  is built in P2-9). `POST /api/models/import` takes a `found` id from the
  scan or the path check (D-76, P2-10). The server builds the source and
  the name. A request never carries a tag, repository, file or path that
  reaches a runtime.
- **Rule 4 is a type.** A number a user will see about this machine is a
  `figure.Bytes` or `figure.Rate` with `source: "estimated" | "measured"`;
  a number someone else published about a model is a `figure.Public`, which
  has no source, carries its origin (publisher, the source's own date,
  attribution) and never shares a struct with the other two
  (`TestPublicAndLocalNeverShareAStruct`; ARCHITECTURE.md D-53). A numeric API field
  that is neither (an id, a count, a timestamp, configuration, a value read
  from the OS) carries the struct tag `source:"n/a"` and a comment saying
  which. `server.APITypes()` lists every API type and
  `TestEveryUserFacingNumberHasASource` fails the build otherwise. When you
  add an API type, add it to that list. In the UI, a number with provenance
  is rendered by `<Figure>` and nothing else, and a public value by
  `<PublicFigure>` and nothing else — in its own block, never the same
  column, row or sentence as a local number.

## Layout

```
cmd/advisor/            main: opens the store, starts the server on 127.0.0.1, opens the browser once
internal/server/        HTTP API (/api/*), Host/Origin checks, embedded UI (go:embed), APITypes()
internal/store/         SQLite via modernc.org/sqlite; migrations/NNNN_*.sql; DefaultDataDir
internal/hardware/      Profile + Detect: per-OS probes behind an env seam, runtime-support rules, tier, fingerprint
internal/backend/       Backend interface + registry; internal/backend/ollama (step 3) drives Ollama through egress.Local, and its installer download is pinned to a release and checked against Ollama's published sha256sum.txt (download.go, D-66)
internal/catalog/       curated families: YAML loader + validation, repo-file grouping, installed-model matching (step 4)
internal/catalog/gguf/  the GGUF header parser (stops at the tokenizer; real header fixtures in testdata/)
internal/catalog/parquet/  a minimal Parquet reader for Arena's leaderboard files: flat tables, plain and dictionary encodings, Snappy (D-56)
internal/catalog/hf/    the Hugging Face client: listings with ETags, header range reads, rate limits
internal/catalog/refresh/  Run (YAML → Hub → catalog_models/catalog_files, then the public sources) and MapInstalled
internal/catalog/external/ public benchmark data (step 9b): the approved sources' clients (Hugging Face Eval Results, Arena, Epoch AI), a polite fetcher limited to PermittedHosts, the coverage report, and the View the screens and the engine read
internal/estimate/      Fit (memory terms, split, category + the threshold that decided), the speed range, placement, gpus.yaml loader; Config holds every constant
internal/recommend/     Recommend: at most three cards with templated reasons, versus-current, confidence; Config holds every weight
internal/bench/         the benchmark harness (step 6): the suite (internal/suite), plan + spill refusal, one-at-a-time runner with a cancel that unloads, 1 Hz resource sampler (per-OS probes behind a sysEnv seam), medians + spread, write-back and calibration evidence
internal/watch/         new-model watch state, notifications, log (step 10); notify_windows.go posts a real AUMID-attributed toast (step 11, D-63), falling back to the legacy balloon
internal/tray/          the tray icon + menu (step 11, D-61): wraps gogpu/systray behind Run(ctx, Options); a fake backend makes menu construction and the autostart toggle testable without a real OS tray
internal/autostart/     "start at login" (step 11): one file per OS behind a cmdRunner/registry seam — a LaunchAgent plist (macOS), the HKCU Run key (Windows, the same value the installer's own checkbox writes), a systemd user unit (Linux, no sudo)
internal/update/        Check (step 11, D-62): one GET to GitHub's release feed, only on a manual "Check for updates" click, through egress.Client(egress.UpdateCheck)
internal/egress/        the advisor's whole outbound network (step 12, D-64): hosts.go — the one allow-list, by purpose; Client (HTTPS to listed hosts only, every redirect checked) and Local (loopback only, no proxy)
internal/suite/         the benchmark suite (data/bench/) and suite.Prompt, the sealed type that is the only text the benchmark can send a model (step 12, D-65; the chat's own sealed path is D-74)
internal/archtest/      tests that read the source (step 12, D-64, D-65, D-69, D-70): only egress reaches the network, D-11's import direction, only bench calls Generate (and, from P2-4, only the chat handler calls Chat, D-74), dependencies and CI actions pinned
internal/winapp/        Windows-only identity (step 11, D-63): registers the AUMID and its display name so toast notifications, the tray and the installer all read as one app; a no-op on macOS/Linux
internal/figure/        Source, Bytes, Rate, Public, Check and CheckSeparation — product rule 4, and public data kept apart
internal/version/       Version (set by -ldflags), GoVersion
ui/                     Vite + React + TypeScript; builds into internal/server/ui/dist
data/                   data.go embeds the data files (package advisor/data)
data/catalog/           families.yaml — the curated catalogue (data, never counted in prose); external.yaml — the approved public-data sources and the metric → purpose map; aliases.yaml — each source's names for catalogue sizes
data/hardware/          runtime-support.yaml — which GPU path Ollama should use per card; gpus.yaml — memory bandwidth per graphics part and processor family; both with sources and dates
data/bench/             suite.yaml + text.txt — the benchmark suite: the advisor's own prose, three prompts, the options; versioned and pinned by digest (internal/suite/suite_test.go; the schema line in suite.yaml still says internal/bench.Suite, which is now an alias — editing the file would move its pinned digest)
data/icon/              the tray icon PNGs, embedded (step 11); scripts/gen_icon.py is the one source for these and for every per-OS derivative under packaging/
scripts/probe0/         step 0's estimator experiment, unchanged, with its reports in results/ (internal/estimate's tests replay them)
scripts/calibrate/      the dev-side speed instrument: llama-bench JSON → the speed model's factors, results/ to commit; README says how
scripts/gen_icon.py     generates packaging/icon/master.png and every size derived from it (data/icon's PNGs, packaging/windows/icon.ico, packaging/linux/icons/hicolor/*) — rerun and commit the output if the design changes (RELEASING.md)
packaging/              per-OS packaging scripts (step 11): macos/ (Info.plist template, build_app.sh, build_dmg.sh), windows/ (setup.iss, the Inno Setup installer), linux/ (the .desktop entry, the hicolor icon set, build_appimage.sh, the .deb's pre/post install scripts) — RELEASING.md is the walkthrough, claude/step-11-packaging.md says what was verified where
.goreleaser.yaml        builds + archives + the .deb + checksums + the GitHub Release; what it does and doesn't cover is in its own header comment and RELEASING.md
claude/                 step-by-step build history: one doc per BUILD_PLAN.md step (closed once its gate passes), plus backlog.md and ci-failures-to-fix.md; real repo files, not just Project docs
.github/workflows/      CI: ubuntu, macos, windows; a `v*` tag additionally runs the release job and three packaging jobs (step 11, RELEASING.md)
```

Dependency direction is one way (`ARCHITECTURE.md` D-11): `cmd` → `server`
→ domain packages → `figure`/`version`. Nothing imports `server`.

## How to run, test, build

Needs Go (the version in `go.mod`; a newer toolchain fetches it) and Node 22+.

```
make dev        # daemon (-tags noui, no browser) + Vite dev server with /api proxied; Ctrl-C stops both
make ui         # build the UI into internal/server/ui/dist
make test       # UI build, go test ./... (UI embedded), UI tests — what CI runs
make test-go    # go test -tags noui ./... — no Node needed
make check      # gofmt, go vet, tsc
make build      # dist/advisor-{linux-amd64,darwin-arm64,darwin-amd64,windows-amd64[.exe]}
make dmg        # macOS only: dist/macos/Local LLM Advisor.dmg (needs Xcode's command line tools)
make installer  # Windows only: the Inno Setup installer (needs iscc on PATH)
make appimage   # Linux only: a self-contained dist/*.AppImage (needs appimagetool on PATH)
```

On a Mac, `scripts/verify.command` (double-clickable) runs `go mod tidy`,
`make test`, `make build` and a smoke test of the built binary, and writes
the output to `verify.log` — the same sequence as CI, for a machine where
a session cannot run Go itself.

- A fresh clone needs `make ui` once before a plain `go build ./...` works,
  because `go:embed` needs the built UI to exist. Until then use `-tags noui`.
- `make dev` keeps its data in `./.dev-data` (`ADVISOR_DATA_DIR`), never in
  the real data folder.
- The daemon: `advisor [-port N] [-data-dir DIR] [-no-browser] [-v] [-version] [-tray]`.
  The port is the only network setting. `-tray` (step 11) is what the
  packaged app's launchers set — a tray icon and menu instead of a plain
  terminal process, with the first-ever launch opening the browser once
  and every launch after that not; `make dev` and a bare invocation leave
  it off, unchanged.
- The curator's catalogue tools (never the customer's):
  `advisor catalog check [FILE]` validates `families.yaml`, `external.yaml`
  and `aliases.yaml` offline;
  `advisor catalog refresh [-data-dir DIR] [-family ID,...] [-json]`
  resolves every size against Hugging Face (listings + header range reads,
  no weights), stores it, and lists installed models the catalogue does not
  know, then reads the public benchmark sources that are due
  (`-no-external` skips them); it exits 1 if a size did not resolve. The
  daemon's `POST /api/catalog/refresh` runs the same thing, answering once
  the list is in and reading the public sources in the background
  (`GET /api/catalog/status` follows both).
  `advisor catalog external [-data-dir DIR] [-force] [-report] [-capture DIR]`
  reads the approved public sources alone and prints the coverage report
  (each source's hits and misses across the curated sizes); `-report` is
  offline; `-capture` saves every answer, for fixtures.
- The developer's view of a running daemon's recommendations, as text:
  `advisor recommend [-port N] [-purposes chat,coding] [-current NAME] [-detail]` —
  what `scripts/verify.command` prints for build-plan step 5's gate;
  `-detail` adds the top pick's detail view (public data and this machine,
  apart — step 9b's gate).
- The developer's benchmark client for a running daemon (build-plan step 6's
  gate): `advisor bench [-model NAME] [-num-ctx N] [-prompts 500,2000]
  [-runs 2] [-measure-anyway]` runs the suite and prints it (exit 3 when
  consecutive runs differ by more than `-agree`, 5%; without `-model`, the
  smallest installed curated model of 3B parameters or more that fits —
  ARCHITECTURE.md D-50); `advisor bench -cancel-during loading|measuring`
  checks a cancel leaves nothing loaded (asking Ollama's own `/api/ps`
  too); `advisor bench -history`. The customer's is the Benchmarks screen,
  on the same API.
- `go run ./scripts/calibrate -label NAME bench.json` scores a llama-bench
  run against the speed model (scripts/calibrate/README.md).
- UI alone: `cd ui && npm run dev | build | test | check`.
- This machine's hardware profile, as the daemon would read it:
  `ADVISOR_PRINT_PROFILE=1 go test ./internal/hardware -run TestDetectOnThisMachine -v`,
  or `GET /api/hardware` on a running daemon.

## Conventions

**Go.** `gofmt`, `go vet`, standard library first. Errors are wrapped with
context (`fmt.Errorf("store: open %s: %w", …)`); package-prefixed messages.
`log/slog` for logging. Contexts on anything that waits. Tests live beside
the code, use `t.TempDir()`, and never touch the network or the real data
folder. Fixture files, not live tools, for every parser: hardware
detection reads the machine through `internal/hardware`'s `env` seam, and a
machine is one `testdata/<os>/<name>.txtar` holding each tool's output in its
real format (`-- $ nvidia-smi --`, `-- sys/bus/pci/devices/…/class --`). A
fixture says in its header whether values were captured or chosen. Each
scenario has a golden profile in `testdata/golden/`; after an intended change
run `go test ./internal/hardware -run Scenario -update` and review the diff
like code. No symlinks and no colons in fixture file names (Windows checks
them out). GGUF fixtures are real headers — the first megabytes of a model
file, gzipped, never a whole file — with their provenance in
`internal/catalog/gguf/testdata/README.md`; the Hugging Face client and the
refresh are tested against an `httptest` hub that serves them by range. The
benchmark sampler reads the machine through `internal/bench`'s `sysEnv` seam
(fixtures in `testdata/sampler/`, each tool's real format), and the Ollama
adapter's load report parses server-log fixtures shaped from the format
strings of the Ollama and llama.cpp builds it names
(`internal/backend/ollama/testdata/README.md` says which). The public-data
clients are tested against an `httptest` fake of the three sources; their
fixtures (`internal/catalog/external/testdata/`) are cut from real answers
captured by `scripts/verify.command` (`.captures/external/`), except the few
cases no real answer has shown yet — the README there says which is which.

**API.** JSON, snake_case keys, `GET /api/…`. Register endpoints through
`Server.api("METHOD /api/path", handler)` so a wrong method is a 405. Errors
are `{"error": {"code", "message"}}`. Numbers a user sees are figures
(above). Strings the UI shows come from the API in words, not codes, when
they are for the user; codes are for the UI's logic.

**Local data.** The data folder is 0700 and the database 0600 (`store.Open`).
Anything new the advisor writes goes in the data folder. A file written
anywhere else is listed in ARCHITECTURE.md D-68 and in `SECURITY.md`, and
"delete everything" (`POST /api/data/delete`, `internal/server/deletedata.go`)
removes it. The advisor sets no environment variable and never writes
another app's settings: where Ollama keeps models is Ollama's own setting,
read back from Ollama (D-72). A backend that writes files implements
`backend.DataForgetter`.

**Store.** Every table has integer `id` + RFC 3339 UTC `created_at`;
booleans 0/1; a `*_json` column for a struct that will grow beside plain
queryable columns; `source` columns are `CHECK`ed. A schema change is a new
`internal/store/migrations/NNNN_name.sql`; never edit a shipped one. Typed
methods on `*store.Store`, not ad-hoc SQL in other packages.

**Unknown is unknown.** A value the code cannot read is `"unknown"` / `0`
with `*_known: false` / `NULL` — never a default, never a guess. The UI turns
it into a sentence, never `0 GB`. A speed the advisor cannot estimate is an
absent `Rate` and a sentence (`estimate.Speed.Unknown`); a memory budget it
cannot read is category `unknown`.

**Constants live in configs.** Every number the estimator uses is in
`estimate.Config` (`internal/estimate/config.go`), marked MEASURED (fleet),
MEASURED (public) or CHOSEN with what would settle it; every weight and
threshold of the recommendation engine is in `recommend.Config`; every
constant of the benchmark harness is in `bench.Config`, and what it sends to
the model is the suite (`data/bench/`) — change either and bump the suite's
version when the change alters what a run measures. No magic
number anywhere else in those packages. The memory formula is pinned by
`step0_test.go` — a change that moves a step 0 row is a change to
ARCHITECTURE.md D-20, not a refactor — and the outcomes the weights must
produce are pinned by `recommend_test.go` on the golden hardware profiles.

**What the customer reads is templated.** Reasons, warnings and the
confidence sentence are built in `internal/recommend/reasons.go` from the
facts the rules used, and arrive at the UI as sentences; the screen adds
labels, not claims. A test holds the copy rule over every reason.

**UI.** Every string in `ui/src/copy/en.ts` (i18n-ready, English only);
screens carry no prose of their own. Every screen is in `ui/src/screens/
index.ts`, which generates both the navigation and the routes. Settings
state (`ui/src/state/settings.tsx`) owns the Advanced toggle; read it with
`useAdvanced()`. API types are mirrored by hand in `ui/src/api/types.ts` —
change both sides together. `localStorage` is guarded and per-viewer only;
durable settings go through the daemon's `settings` table (step 8).

**Copy.** No term from {VRAM, quantization, GGUF, KV cache, context window,
tokens/sec, offload} without its explainer (step 7 adds the glossary).
Buttons say what they will do and what it costs. Never count the catalogue.

**Data files** (`data/`) are data: the schema is in the file's header
comment and mirrored by a Go type; entries carry the date a person reviewed
them and where they came from. They are embedded through `data/data.go` and
decoded strictly (`goccy/go-yaml`, unknown keys are errors).
`runtime-support.yaml` is checked against the Ollama release it names — its
build presets, not only its docs; `support_test.go` is its contract.
`gpus.yaml` rows state the memory configuration their bandwidth derives
from (the parser refuses one that does not equal rate × width ÷ 8), are
ORDERED (laptop parts above desktop parts of the same name), and carry an
efficiency or prompt-ratio override only with the measurement behind it;
`devices_test.go` lists real device names and the rows they must land on —
add the name when you add a row. A part that is not in the file gets no
speed estimate, by design.

**Dependencies.** Go: stdlib + `modernc.org/sqlite`, `github.com/goccy/go-yaml`
(data files) and `golang.org/x/sys` (CPUID, and the Windows registry calls
`internal/autostart`/`internal/winapp` make) — ARCHITECTURE.md D-26. Step
11 (D-61) adds the tray icon's three: `github.com/gogpu/systray` (pure Go,
wrapped behind `internal/tray`'s own interface since it's a young
library), its `github.com/go-webgpu/goffi` dependency, and
`github.com/godbus/dbus/v5` (its Linux D-Bus backend) — still zero cgo.
UI: React, React Router, Vite, Vitest, Testing Library. A new one is a
sentence in the PR saying what it replaces. Nothing that needs cgo, ever.
Versions are pinned (D-69): `go.mod`/`go.sum`, and exact versions in
`ui/package.json` (`ui/.npmrc`: `save-exact`, `ignore-scripts`); CI's
`security` job runs `go mod verify`, govulncheck and `npm audit`, and every
action is pinned to a commit. `internal/archtest/deps_test.go` names the
direct dependencies; adding one means adding it there too.

**Network.** Outbound requests go only to the hosts in
`internal/egress/hosts.go`, the one allow-list (ARCHITECTURE.md D-64):
Hugging Face (`huggingface.co`, `hf.co` and their CDNs) for the model list
and the public data (Arena's files are on the Hub, D-56), `epoch.ai`, Ollama's
download (`ollama.com/download/`, `github.com/ollama/ollama/releases/` and
GitHub's two download hosts), and `api.github.com` for this project's
releases. Each is listed for its purposes only. Take a client from
`egress.Client(purpose, timeout)` — never build one; a test fails if you do.
A new host is a line in `hosts.go` with what is fetched and why, reviewed
as a product decision. `external.PermittedHosts` stays the per-source
ceiling that `data/catalog/external.yaml` cannot exceed (D-53), and is
tested to sit inside egress's list. The requests are a function of the data
files and the version alone, the same on every install, with one exception:
`ModelSearch` (D-75, from P2-8) sends the Hub's search with the words the
person typed, and the listing and header reads of a repository they
opened, only on their click, only to `huggingface.co` and `hf.co`, with the
screen saying so. The daily watch never uses it. Nothing else the user
typed is sent anywhere; the chat's words go only to the runtime on this
computer (D-74). A Hugging Face download of an uncurated model is made by
Ollama, from a tag the server built (D-75). No telemetry, no token, no
cookie. The Hugging
Face client (`internal/catalog/hf`) follows redirects only to Hugging
Face's own hosts, reads GGUF headers with range requests and refuses a
whole-file answer, and honours the Hub's rate limits (D-34). The daily watch
never makes the first contact: until the person has fetched the model list,
a scheduled check does nothing (D-68). Tests give their fake servers a plain
transport (`c.HTTP.Transport = http.DefaultTransport`); only test files may.

**The advisor calls no LLM to do its own job.** Estimation is arithmetic,
recommendation is rules over data. Wanting a model to decide means the
catalogue is missing a field.

## Git

`main` is the integration branch. One build-plan step per session; a step
ends with a commit that names it (`step 1: architecture record, skeleton,
CI`). Do not commit `dist/`, `internal/server/ui/dist/`, `ui/node_modules/`
or any `*.db`; `.gitignore` has them. Line endings are LF everywhere
(`.gitattributes`).

## What each step reads

Before a step, read `CLAUDE.md`, `ARCHITECTURE.md`, and the packages the
step's prompt names. Step 0's findings that bind later steps are
`ARCHITECTURE.md` D-20. When a decision changes, add a superseding entry to
`ARCHITECTURE.md` rather than editing the old one.
