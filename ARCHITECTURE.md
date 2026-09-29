# Architecture decision record — Local LLM Advisor & Optimizer

**Status:** accepted, step 1 (2026-09-18); D-23 to D-26 added in step 2,
D-27 to D-31 in step 3, D-32 to D-37 in step 4, D-38 to D-43 in step 5,
D-44 to D-51 in step 6, D-52 in step 7, D-53 to D-56 in step 9b, D-57 in
step 10, D-58 to D-59 in the recommendation-ranking backlog work that
followed, D-60 to D-63 in step 11, D-64 to D-70 in step 12 (security and
privacy review), D-71 to D-78 in phase 2 step P2-2 (the phase's decisions).
Every later step inherits this shape. A change to a decision here is a new
numbered entry that supersedes the old one — the old entry stays, marked
superseded, so the reasoning survives.

**Read with:** `PRD — Local LLM Advisor & Optimizer.md` (what and why),
`BUILD_PLAN.md` (the fourteen steps), `BUILD_PLAN_PHASE2.md` (phase 2),
`CLAUDE.md` (the product rules and the repo conventions — the working
reference for every session).

## Context

The product is the MVP of PRD §17: a desktop application for someone who has
heard you can run AI on your own computer, owns a laptop or a gaming PC, and
has no idea what a GGUF is. It looks at the machine, finds what is installed,
recommends models and settings that will fit, tests them, and says when
something new is worth trying. Ubuntu, macOS and Windows 11.

Step 0 (`scripts/probe0/`) established that the product's central claim
holds: memory use can be predicted from model metadata within 15% on 27 of
28 dense rows across CUDA, Metal and Vulkan. The formula that passed, and the
findings that came with it, are constraints on this architecture (D-20).

Decisions D-1 to D-10 restate what `BUILD_PLAN.md` decided before this step,
with the consequences made explicit. D-11 onward are taken here.

---

## D-1. A local daemon serves an ordinary web UI at `http://127.0.0.1:<port>`

**Decision.** Not fully web, not Electron, not Tauri. A process on the machine
is unavoidable — hardware probing, GPU readings, driving Ollama, running
benchmarks — and that process serves the UI over HTTP on the loopback
interface, the same shape as Ollama and Open WebUI. A tray icon plus "open in
browser" is the desktop experience (step 11).

**Consequences.**
- The UI is a normal single-page app; there is no IPC layer, no native
  bridge, no second build toolchain per OS.
- The daemon must be safe to leave running: it binds to loopback only (D-12)
  and defends against the browser being used against it (Host/Origin checks).
- The user's browser is a dependency the product does not control. The UI
  targets current evergreen browsers and nothing else.
- "Opening the app" means opening a URL. The first launch opens the browser
  once; later launches must not spawn a new tab every time (step 11).

## D-2. One Go binary per OS, React + TypeScript UI embedded, SQLite for everything local

**Decision.** Go for the daemon: it cross-compiles to all targets from one
machine and needs no runtime on the customer's. The UI is Vite + React +
TypeScript, built to static files and embedded in the binary with `go:embed`.
Local state is one SQLite file through `modernc.org/sqlite` (pure Go), so
there is no cgo and no C toolchain per platform. Not TypeScript end-to-end:
Node has no clean single-binary story across three OSes, and native modules
need a build per platform.

**Consequences.**
- `CGO_ENABLED=0` is a build invariant. A dependency that needs cgo is
  rejected, whatever it offers.
- The build is `make build`: four binaries (`linux/amd64`, `darwin/arm64`,
  `darwin/amd64`, `windows/amd64`), each self-contained. There is nothing to
  install beside the binary.
- Two toolchains (Go, Node) exist at development time only. The customer
  sees neither.
- Anything that must be shared between the Go and TypeScript sides — API
  types, enums — is mirrored by hand and kept in step by review, not by a
  code generator (D-19).

## D-3. Ollama is the only backend in the MVP; the backend interface exists from step 3

**Decision.** One runtime, driven through its HTTP API. The `Backend`
interface is defined once, in step 3, after checking every method against
the LM Studio local API and llama-server's API, so llama.cpp and LM Studio
(PRD §18) are a second file each, not a rewrite.

**Consequences.**
- `internal/backend` owns the interface and a registry; implementations live
  in `internal/backend/<name>` and register themselves. Step 1 ships the
  registry and the part of the interface the skeleton needs (`Name`,
  `Detect` → `Status`); step 3 completes it without renaming anything.
- Where a model comes from — an Ollama library tag versus a GGUF file on
  Hugging Face — is the seam most likely to leak. `Pull` takes a
  `ModelSource`, not a string (step 3).
- An Ollama-shaped assumption found in a caller is fixed in the interface,
  not in the caller.

## D-4. Runtimes and chat apps are two lists

*Amended by D-74: the product still never becomes a chat interface. "Use
it" becomes "Start chatting": the advisor loads the model with no text,
then opens the chat this computer already has (the runtime's own page, a
web chat already running, or the runtime's app) on a click, and never
drives it.*

**Decision.** A runtime is what runs the model (Ollama now; llama.cpp, LM
Studio later) and the advisor drives it. A chat app is where the person talks
to the model (Ollama's own app, LM Studio, the Open WebUI desktop app, Jan,
AnythingLLM); the advisor only detects it and hands off. The MVP drives one
runtime and installs no chat app.

**Consequences.**
- `internal/backend` never grows chat-app detection. Step 7 adds a small
  `internal/chatapps` package that knows well-known install locations per OS
  and nothing else.
- The product never becomes a chat interface (PRD §22). "Use it" ends with
  the model's exact name and a copy button.

## D-5. The runtime path is a fact the app establishes, not an assumption

**Decision.** A GPU existing and Ollama using it are different things. Per
GPU, `hardware` records the *expected* path (cuda, metal, rocm, vulkan, none)
from vendor + model against a data file, and `backend` records the path
*actually taken* after a load, from `/api/ps` and the runtime's own device
discovery lines. The two are shown side by side when they differ.

**Consequences.**
- `hardware.RuntimePath` is one type shared by both packages (defined in
  `hardware`, which `backend` imports; never the reverse).
- "You have a graphics card and Ollama is not using it" is a distinct,
  detectable state, because it is the state an AMD or Intel owner is most
  likely in. The recommendation engine builds for the CPU in that state and
  says so first (step 5).
- The estimator's overhead term is keyed by the runtime path (D-20), so an
  estimate made before any load is made against the expected path and
  labelled as such.

## D-6. AMD and Intel are customers, not a later phase

**Decision.** Detection, estimation and the runtime path cover every vendor
from step 2. What leans on NVIDIA and Apple Silicon is only the *gates* —
where the estimator can be checked against real VRAM readings.

**Consequences.**
- `hardware.Vendor` has nvidia, amd, intel and apple from day one; nothing
  in the skeleton special-cases NVIDIA.
- Where a vendor has no reading (AMD and Intel on Windows have no
  per-process VRAM counter), the row says so rather than showing zero.
- Step 0 answered the open question: the Ubuntu Mac Pro's D700s run on
  Vulkan under Ollama, and it is the AMD test box.

## D-7. The candidate set is curated; Hugging Face is the metadata source

**Decision.** A maintained list of trusted model families with purpose tags
(`data/catalog/families.yaml`), with Hugging Face as the source of per-quant
GGUF metadata. Open discovery across all of Hugging Face is Phase 2. A
beginner shown fifty thousand repos has been shown nothing.

**Consequences.**
- The catalogue is data. Docs, copy and code never state its size (product
  rule 8).
- The YAML is the schema of record; `catalog.Family` mirrors it. Step 4 adds
  the loader and the Hugging Face client; the schema in the file's header
  comment is what it loads.
- An installed model the catalogue does not know is a signal for the
  curator, stored as `installed_models.catalog_file_id IS NULL`, not an error.

## D-8. The advisor calls no LLM to do its own job

**Decision.** Estimation is arithmetic; recommendation is rules over data.
The only models that run are the ones being tested.

**Consequences.**
- There is no inference client in the daemon besides the backend adapter,
  and the adapter's `Generate` is used by the benchmark harness only.
- If a step wants a model to make a decision, the catalogue is missing a
  field. Add the field.
- Explanations ("fits your graphics card with room for long documents") are
  templated from structured reasons (`recommend.Reason`), which keeps them
  true and testable.

## D-9. No telemetry and no community database in the MVP

*Amended by D-75: a search sends its words to Hugging Face on the person's
click, with the screen saying so.*

**Decision.** Nothing leaves the machine except requests to the model
sources. PRD §11 is Phase 2 and needs its own privacy note before a line of
code.

**Consequences.**
- There is no analytics, crash reporting or update-check-with-payload code
  path. "Check for updates" (step 11) compares a version string against a
  release feed and links to a download.
- Benchmark prompts are the suite's own text; nothing the user typed is ever
  sent anywhere, and the code should make that impossible rather than merely
  true (step 12 reviews it).

## D-10. Official APIs and permitted sources only

*Step 12 built the one-file allow-list this entry asked for: D-64
(`internal/egress/hosts.go`), enforced at the transport.*

*Step 9a decided the list: D-53 records it, with the research note
(`research/EXTERNAL_SOURCES.md`) that argues it.*

**Decision.** Until step 9a decides otherwise, Hugging Face is the only
external source. Scraping a page whose terms do not permit it is excluded
regardless of usefulness.

**Consequences.**
- Every outbound host is on an allow-list in one file (step 12); the list is
  the audit.
- Public data and local measurements never share a table or a column
  (`catalog_external` versus `benchmark_runs`), and never a UI column (PRD
  §10).

---

## D-11. Package layout and dependency direction

**Decision.** The repo is laid out as `BUILD_PLAN.md` step 1 specifies, with
two small additions (`internal/figure`, `internal/version`) justified below.
Dependencies point one way:

```
cmd/advisor ──▶ internal/server ──▶ store, hardware, backend, catalog,
                                     estimate, recommend, bench, watch
                                     (all of which may import figure, version)

hardware ◀── backend            (RuntimePath lives in hardware)
catalog  ◀── estimate ◀── recommend ◀── watch
figure   ◀── estimate, recommend, bench
```

`internal/server` is the only package that knows HTTP. No `internal/*`
package imports `server`, and nothing imports `cmd/advisor`. The domain
packages (`catalog`, `estimate`, `recommend`, `bench`, `watch`) are
types-only in step 1 and gain behaviour in their own steps without moving.

**Why.** One direction means one place to look for any concern, and no
package can reach back into HTTP handlers or the process. The order matches
the workflow: hardware → backend → catalogue → estimate → recommend →
bench → watch.

**Consequences.**
- A new concern gets a new package under `internal/`; nothing accretes in
  `server` or `main`. `cmd/advisor/main.go` stays a wiring file.
- Storage access is behind `internal/store` (D-13); other packages take a
  `*store.Store` and add typed methods to it rather than writing SQL where
  they live.
- The additions: `internal/figure` (D-14) because provenance is a cross-
  cutting type every domain package needs, and `internal/version` because
  both `main` and `server` need the build stamp and `-ldflags -X` needs a
  fixed import path.

## D-12. The daemon binds to `127.0.0.1` — a constant, refused otherwise, and checked twice

*Tightened by D-67: the Origin must be the daemon's own origin, not any
loopback origin, and Sec-Fetch-Site is checked on the API.*

**Decision.** `server.LoopbackHost` is a `const`. `server.Listen(port)` is
the only listener constructor and takes only a port. `Server.Serve` refuses a
listener whose address is not loopback (`ErrNotLoopback`). Above the socket,
every request's `Host` header must be `127.0.0.1` or `localhost` (421
otherwise) and a browser-sent `Origin`, when present, must agree (403
otherwise). All of this is tested in `internal/server/server_test.go`.

**Why.** Product rule 7 says "binds to 127.0.0.1 only". A flag would make it
a default; a constant makes it a property. The Host check exists because
loopback binding alone does not stop a web page in the user's browser from
addressing the daemon through DNS rebinding.

**Consequences.**
- There is no bind-address flag, environment variable or setting, and there
  must never be one. A future request for LAN access is a product decision
  that reopens this entry, not a flag.
- IPv4 loopback only (`tcp4`): one address to reason about, one address the
  Host check admits.
- The Vite dev server proxies `/api` with `changeOrigin: true` so its
  requests carry the daemon's own Host.
- Step 12 reviews this layer; it does not need to introduce it.

## D-13. Storage: one SQLite file, numbered embedded migrations, schema v0 now

**Decision.** One database file in the per-OS application-data folder
(`store.DefaultDataDir`: `~/.local/share/advisor`, `~/Library/Application
Support/Advisor`, `%LOCALAPPDATA%\Advisor`; `ADVISOR_DATA_DIR` overrides).
WAL journal, `busy_timeout`, foreign keys on, a single connection. Schema
changes are `internal/store/migrations/NNNN_name.sql`, embedded, applied in
order in a transaction and recorded in `schema_migrations`. A shipped
migration is never edited. Schema v0 (`0001_schema_v0.sql`) creates every
table the fourteen steps need: `hardware_profiles`, `backends`,
`installed_models`, `catalog_models`, `catalog_files`, `catalog_external`,
`estimates`, `benchmark_runs`, `benchmark_samples`, `watch_state`, `settings`.

Column conventions: every table has an integer `id` and an RFC 3339 UTC
`created_at`; booleans are 0/1; a `*_json` column holds a Go struct expected
to grow, beside the plain columns that are queried; `source` columns are
`CHECK (source IN ('estimated','measured'))`.

**Why.** Every later step needs a place to write, and a schema that exists
before the code that fills it makes each step's data model a review item now
rather than a surprise later. Plain `database/sql` with hand-written SQL and
no ORM keeps the dependency count at one and the SQL readable.

**Consequences.**
- Step N adds columns or tables with `000N_*.sql`; it does not touch
  `0001_schema_v0.sql`. Tests check that migrations are consecutive and that
  the v0 table set survives.
- History is kept on purpose: `hardware_profiles` gets a row per daemon
  start, `installed_models` marks rows `present = 0` rather than deleting
  them, so a benchmark from last month stays attributable.
- `estimates` rows are updated in place when a measurement arrives (their
  `source` flips), which is product rule 4's second sentence expressed as a
  schema.
- The data folder is the one place the daemon writes (D-16); "delete
  everything" (step 12) deletes it.

## D-14. Provenance lives in the type system: `internal/figure`

**Decision.** A number a user will see is a `figure.Bytes` or a `figure.Rate`,
each carrying `Source ∈ {estimated, measured}` in a JSON field `source`.
`Source` refuses to marshal or unmarshal any other value, including the zero
value, so a forgotten provenance is a failing test or a 500, never a number
on screen that looks measured. Numeric API fields that are neither estimated
nor measured — ids, counts, timestamps, configuration such as `num_ctx`,
values read from the OS — carry the struct tag `source:"n/a"` with a comment
saying which. `internal/server.APITypes()` lists every type the API serves,
and `TestEveryUserFacingNumberHasASource` runs `figure.Check` over it: a new
numeric field without a figure type or the tag fails the build.

On the UI side, `ui/src/api/types.ts` mirrors `Source`, `Bytes` and `Rate`,
and `components/Figure.tsx` is the one component that renders such a value —
two visual treatments, the `source` prop required by the type.

**Why.** Product rule 4 ("estimated is never dressed as measured") is the
product's most-cited risk (PRD §21) and the easiest rule to erode one field
at a time. A convention in the UI is exactly that kind of erosion. A type
that cannot be serialised without provenance, plus a test that inspects every
API type, cannot be forgotten.

**Consequences.**
- An estimate is a range (`Rate.Low..High`) and a measurement is a point;
  the type says which, and the UI never guesses.
- Adding an API type means adding it to `APITypes()`; the reviewer looks for
  that line. Adding a numeric field means deciding, in the type, whether it
  is a figure.
- `hardware.Profile`'s numbers are facts read from the OS and are tagged
  `n/a`; what is derived from them lives in `estimate` as figures.
- The `estimates` table's `CHECK` (D-13) is the same rule at the storage
  layer.

## D-15. The UI is embedded with `go:embed`; a `noui` build tag keeps development honest

**Decision.** Vite builds to `internal/server/ui/dist` (gitignored);
`internal/server/ui.go` embeds it with `//go:embed all:ui/dist` and a SPA
handler serves files or falls back to `index.html`. `ui_noui.go` (`-tags
noui`) serves a one-paragraph placeholder instead, so the Go side compiles
and tests before the UI has ever been built. `make dev` runs the daemon with
`-tags noui` and the Vite dev server with `/api` proxied to it; `make build`
and CI build the UI first and embed it.

**Why.** `go:embed` cannot be conditional and refuses an empty directory.
The alternatives — committing a placeholder `index.html` that every build
overwrites, or a copy step — leave the working tree dirty after every build.
A build tag is explicit and cheap.

**Consequences.**
- A fresh clone needs `make ui` once (or `-tags noui`) before plain
  `go build ./...` works; `CLAUDE.md` says so. CI runs both configurations.
- Client-side routes work when served by the daemon because every non-file
  path returns `index.html`; the API is matched first, so `/api/*` never
  falls through to the SPA.
- Hashed assets under `/assets/` are cached for a year; `index.html` is not.

## D-16. Where things live on the customer's disk

*Superseded in part by D-72: the models folder shown and checked is the
one Ollama reports, not `OLLAMA_MODELS`, and the advisor sets no
environment variable.*

**Decision.** The daemon writes to one folder, `store.DefaultDataDir()`
(D-13), and reads model files from wherever the runtime keeps them (the
default Ollama models folder per OS, or `OLLAMA_MODELS`). The Linux
user-space Ollama install (step 3) goes under the same data folder.

**Consequences.**
- The Settings screen can show one path for "your data" and one for "your
  models", and a button that opens each.
- No sudo, no system service, no writes outside the user's profile.
- `make dev` sets `ADVISOR_DATA_DIR=./.dev-data` so development never
  touches a real install.

## D-17. Versioning and `/api/health`

**Decision.** `internal/version.Version` defaults to `dev` and is set at link
time (`-ldflags -X advisor/internal/version.Version=<git describe>`) by the
Makefile and CI. `GET /api/health` returns `{version, os, arch, go_version}`
and is the whole API in step 1. Every API response is JSON with snake_case
keys; every error is `{"error": {"code", "message"}}`; a wrong method is 405
and an unknown `/api/*` path is a JSON 404.

**Consequences.**
- A developer's build says `dev`; a binary that says `dev` in the field was
  not built by CI.
- Benchmark rows record `daemon_version` (D-13), so results stay comparable
  across releases.
- The UI shows the version in the footer from the first screen, which is
  also the "is the daemon up" check.

## D-18. Build and CI

**Decision.** `make build` cross-compiles the four targets from any machine
with `CGO_ENABLED=0 -trimpath -ldflags "-s -w -X …Version"`. CI
(`.github/workflows/ci.yml`) is a matrix on `ubuntu-latest`, `macos-latest`
and `windows-latest`; each runner builds the UI, runs the UI tests, `go vet`,
`go test ./...` with the UI embedded and again with `-tags noui`, builds its
own OS's binary natively, starts it and checks `/api/health`, the SPA and the
foreign-Host refusal, then uploads the binary as an artifact. The ubuntu
runner also runs `make build` and uploads all four. `darwin/amd64` is
cross-compiled on the arm64 macOS runner (there is no Intel runner to rely
on) and smoke-tested under Rosetta when present.

Toolchain versions come from the repo: Go from `go.mod` (`go 1.27.1`), Node
22 from the workflow, npm packages from `ui/package-lock.json`.

**Consequences.**
- Green CI means: tests pass on three operating systems, and a binary
  started on each one served the UI and answered the API. That is the step 1
  gate, minus a human opening the browser.
- Windows runners skip the `gofmt` check (checkout may rewrite line endings);
  `.gitattributes` forces LF so it should never matter.
- Release packaging (installers, signing, tray) is step 11 and builds on this
  job; it does not replace it.

## D-19. Two type systems, mirrored by hand

**Decision.** API types are written twice — Go in `internal/*`, TypeScript
in `ui/src/api/types.ts` — and kept in step by review. No OpenAPI, no
generator.

**Why.** The API is small and shaped by the product, not by a schema
language; a generator is a third toolchain and a source of drift of its own.
The Go side is the source of truth; the TypeScript side is a faithful copy
with the same names.

**Consequences.**
- A PR that changes an API type touches both files, or the reviewer asks
  why. Enums (`Source`, `Purpose`, `Category`, `State`) are copied as string
  unions with identical values.
- If the API grows past what hand-mirroring can hold, this entry is
  superseded with a generator — not silently.

## D-20. Constraints inherited from step 0 (the estimator experiment)

*Generalised, not changed, by D-38: every step 0 row still computes to the
byte. Finding 5's "excluded from recommendations until handled explicitly"
is discharged there — mixture-of-experts, hybrid and vision models are
handled explicitly, and marked as not yet measured.*

**Decision.** The estimator (step 5) implements exactly the formula that
passed the step 0 gate on three machines and three runtime backends, and the
schema, the catalogue types and the GGUF parser (step 4) carry the fields it
needs:

```
predicted = weights_bytes + KV + overhead
KV        = 2 · block_count · head_count_kv · head_dim · ctx · 2 B      (f16 cache)
head_dim  = attention.key_length where the model states one,
            else embedding_length / head_count
ctx       = min(num_ctx, the model's trained context_length)          (Ollama clamps)
overhead  = per runtime path: cuda 250 MiB · metal 0 · vulkan 50 · rocm 50 · cpu 0
```

Result: 27 of 28 dense rows within 15% (mean |err| 5.5%, median 3.9%);
worst row `llama3.2:3b` @ 4096 on the M1 Pro at −20.8%, a measurement-noise
row on a laptop in use.

**Findings that shape later steps.**
1. `attention.key_length` is the real head dimension where stated
   (`qwen3:4b` states 128 against a computed 80). `catalog_files.key_length`
   and `catalog.GGUFHeader.KeyLength` exist for this; the parser must read it.
2. `num_ctx` is clamped to the trained context; `estimates.effective_ctx`
   records what the estimate is actually for.
3. The overhead is a property of the runtime backend, not the model's width
   (fitted against width the slope is negative). The term is keyed by
   `runtime_path` — which is why D-5 matters to the estimator.
4. `/api/ps size` is Ollama's own estimate, not a measurement (0.09% apart
   across Metal and Vulkan for the same model). Measurements come from a
   device-memory delta; `/api/ps` is context.
5. Blob size ≠ resident weights for vision and elastic models; for dense
   text models the blob is sound. MoE and vision are excluded from the gate
   and from recommendations until handled explicitly.
6. On Apple Silicon, wired memory (`vm_stat`) tracks the model; `ioreg`'s
   "In use system memory" does not. The Vulkan sysfs counter under-reads for
   some models (part of the model in GTT).
7. Sliding-window architectures over-predict; step 5 reads
   `attention.sliding_window` and caps the KV term.
8. VRAM is never summed across devices: the budget is the largest single
   device, and unified memory is compared against the OS's stated GPU limit,
   not RAM.

**Consequences.**
- `estimate.Memory` exposes the terms separately so the Advanced view can
  show the arithmetic, and every constant lives in one exported config with
  the measurement that justifies it.
- The step 0 reports in `scripts/probe0/results/` are the calibration set;
  step 5's tests reproduce every dense row within the gate. Reports from the
  Windows/NVIDIA machine and the Mac Pro belong in that folder too.

## D-21. Values the code cannot read are unknown, never defaulted

**Decision.** Following probe0: a string field the detector could not read
is `"unknown"`; a numeric field is `0` with a sibling `*_known: false`; a
nullable column is `NULL`. The UI renders unknown as words ("could not read
how much memory the graphics card has"), never as `0 GB`.

**Consequences.**
- `hardware.Detect` in step 1 returns a profile that is unknown everywhere
  and a `Problems` list saying why; step 2 fills the values in and keeps the
  contract.
- An unknown GPU yields fit categories without speed numbers (step 5), and
  the UI says so — a missing number is a sentence, not a blank.

## D-22. Dependencies are few and named

*Superseded in part by D-26 (step 2 adds the YAML parser and `golang.org/x/sys`).*

**Decision.** Go: the standard library plus `modernc.org/sqlite`. UI: React,
React Router, and the Vite/Vitest/Testing Library toolchain. A new dependency
is a line in the PR description saying what it replaces and why writing it
would be worse.

**Consequences.**
- No HTTP framework, no ORM, no state-management library, no component
  library in the skeleton. Step 7 may add a component approach and says so
  in this file.
- YAML parsing for the catalogue (step 4) is the first expected addition;
  the candidates are `gopkg.in/yaml.v3` or `github.com/goccy/go-yaml`, both
  pure Go.

---

Step 2 (hardware detection, 2026-09-18) adds D-23 to D-26.

## D-23. Hardware detection reads the OS behind one seam, and every OS path runs on every runner

**Decision.** `internal/hardware` reads the machine through an `env`
interface (commands, a root `fs.FS`, environment, disk space, CPUID). The
real one shells out and reads files; the tests' one answers from fixtures —
one `testdata/<os>/<machine>.txtar` per machine, holding each tool's output
in its real format — so the Windows, macOS and Linux paths are all exercised
on every CI runner, and the parsers never need the tool they parse. Each
fixture machine also has a golden profile (`testdata/golden/*.json`) that is
the reviewable answer to "what does the advisor say about this machine".

The sources, per OS, and what is authoritative for what:

| | Windows | macOS | Linux |
|---|---|---|---|
| OS, CPU, RAM, form factor | one PowerShell query: CIM + registry (`-EncodedCommand`, no console window) | `sysctl`, `sw_vers`, `system_profiler -json` | `/etc/os-release`, `/proc`, `/sys/class/dmi/id/chassis_type`, sysfs CPU topology |
| GPU list | `Win32_VideoController` (present devices only) → its display-class registry key | `SPDisplaysDataType` | `/sys/bus/pci/devices` class 0x03, names from `pci.ids` |
| GPU memory | registry `HardwareInformation.qwMemorySize` (never `AdapterRAM`, which wraps at 4 GB) | `spdisplays_vram`; Apple Silicon: D-25 | amdgpu `mem_info_vram_total` |
| NVIDIA | `nvidia-smi` (memory, driver, compute capability, PCI id) overrides the registry | — | `nvidia-smi`, merged by PCI address |
| AMD LLVM target | from the name (D-24 table) | from the name | KFD topology `gfx_target_version` |
| AVX2 / AVX-512 | CPUID via `golang.org/x/sys/cpu` (x86); not applicable on ARM | same | same |

**Consequences.**
- Every value that could not be read is unknown with the reason in
  `Problems` (D-21). An unread device list is not "no GPU": the tier is then
  unknown, not "cpu only".
- The GPU list is ordered so the answer comes first: devices the runtime can
  use, discrete before integrated, most memory first. An iGPU beside a
  discrete card is listed and marked; two discrete cards produce a note that
  multi-GPU is not optimised in the MVP. The budget is the largest single
  device, never a sum (D-20, finding 8).
- Display-only devices (virtual and remote displays, BMC chips, VM adapters)
  are named in `filtered_adapters`, never counted as GPUs. A card with no
  vendor driver (Linux: nothing bound; Windows: Microsoft Basic Display
  Adapter) is still listed, from its PCI id, with what would unlock it.
- The Intel build under Rosetta and the x64 build on ARM Windows describe the
  machine, not the emulator (`arch` is arm64, with a note).
- `Tier` is one of unknown, cpu_only, integrated, gpu_small (< 7 GiB),
  gpu_medium (< 14), gpu_large (< 30), gpu_xl, sized by the best device's
  memory; the thresholds and their reasons are in `derive.go`. `Summary` is
  one sentence built from the profile; it is prose, the estimator decides fits.
- Detection runs in the background after the listener is up, so the browser
  never waits for `system_profiler` or PowerShell; `GET /api/hardware` waits
  for it. Each daemon start inserts a `hardware_profiles` row (with the
  daemon version, migration 0002) and never updates one. `Fingerprint`
  identifies hardware, not state — OS family, arch, CPU model, RAM and each
  GPU's PCI id and memory, rounded to the GiB — so a driver update is the same
  machine and a swapped GPU is a new configuration; old benchmarks keep
  pointing at the old row. `GET /api/hardware/history` lists configurations;
  `GET /api/hardware/profiles/{id}` returns any stored profile.

## D-24. The expected runtime path is data, checked against what Ollama ships

**Decision.** `data/hardware/runtime-support.yaml` holds ordered rules
(vendor, OS, arch, AMD LLVM target, NVIDIA compute capability and driver
floor, Linux kernel driver, "no driver", name patterns) → expected backend,
with a plain sentence, a source and a `checked` date per row; a table of AMD
marketing names → LLVM targets for the OSes that do not report one; and a
table classifying integrated versus discrete by name where the OS does not
say. Rows were checked against Ollama v0.34.2's own build presets
(`llama/server/CMakePresets.json`: the CUDA architectures and ROCm
`AMDGPU_TARGETS` each shipped build compiles) and its docs. Where the two
disagree — the Windows ROCm build compiles gfx1030 and RDNA 4, the docs list
only the RX 7000 series — the build wins ("as Ollama ships it") and the row
says so; step 3 records what actually happened. The file is embedded through
a small `advisor/data` package (`go:embed` cannot reach `../../data`), parsed
strictly (unknown keys are errors) and validated on load; the last rule must
be a catch-all, so no GPU leaves without an expectation, even if it is
"unknown".

**Consequences.**
- Updating for a new Ollama release is a data change reviewed like code; the
  contract tests (`support_test.go`) show every customer-visible answer that
  moves.
- A condition on a value the detector could not read does not hold, so
  unknown falls through to a later rule rather than matching by accident.
- Rules marked `actionable` (a missing or old driver) also become a note on
  the profile, because the fix is the user's to make.
- An OS below Ollama's floor (macOS 14, Windows 10 22H2) expects nothing on
  every GPU, and the tier is unknown with a sentence saying why.

## D-25. Apple Silicon's GPU budget is what macOS says, never a percentage

**Decision.** `gpu_usable_bytes` on Apple Silicon is `iogpu.wired_limit_mb`
when it is set (the OS enforces exactly that), otherwise Metal's
`recommendedMaxWorkingSetSize` — the platform default for that Mac's RAM and
macOS release, and the number Ollama itself schedules against
(`discover/gpu_info_darwin.m`). It is read by asking Metal through
`osascript -l JavaScript` (JavaScript for Automation can bind
`MTLCreateSystemDefaultDevice`), which every Mac has: no cgo, no compiler, no
Metal in the daemon's own process. If neither can be read, the budget is
unknown.

**Why not a table or a ratio.** The widely quoted rule (two-thirds of RAM up
to 36 GB, three-quarters above) does not hold on the fleet's M1 Pro under
macOS 26.6.2: Ollama logs Metal `total="11.8 GiB"` for 16 GiB (0.74). The
value is the OS's to choose and to change; reading it is the only way to be
right on the next release.

**Consequences.** Confirmed on the fleet's M1 Pro (2026-09-18,
`scripts/verify.command`): the Metal query answered 12,713,115,648 bytes
(11.84 GiB, 0.74 of RAM), the figure Ollama logs, and the M1 Pro fixture
carries that value. The other Apple Silicon fixtures' Metal values are
illustrative and say so; replace them when a machine of that size is at hand.
`scripts/verify.command` prints `/api/hardware` and stays the live check.

## D-26. Two dependencies: a YAML parser and golang.org/x/sys

**Decision.** `github.com/goccy/go-yaml` parses the data files (pure Go, no
dependencies of its own). It replaces the `gopkg.in/yaml.v3` D-22 expected: its
author labelled that project unmaintained in April 2025. Step 4's catalogue
loader uses the same parser. `golang.org/x/sys/cpu` reads AVX2 and AVX-512F
by CPUID, including the OS's support for the wider registers, the same way on
all three OSes; `x/sys` was already in the module graph through
`modernc.org/sqlite`, so this adds nothing to the build.

**Consequences.** No cgo, still; `go.sum` gains one module. A future data
file uses the same parser and strict decoding (`DisallowUnknownField`).

## D-27. The Backend interface is frozen against three runtimes, not one

*Amended by D-78: six more methods (`Capabilities`, `Load`, `ChatPage`,
`Import`, `ModelsFolder`, `SetModelsFolder`), checked against the same
three runtimes.*

**Decision.** `internal/backend.Backend` (`Name`, `Detect`, `Models`, `Show`,
`Running`, `Pull`, `Generate`, `Unload`, `Install`, `Start`) was checked
against Ollama's HTTP API, LM Studio's local REST API (`/api/v1/models`,
`/models/load`, `/models/unload`, `/models/download`) and its `lms` CLI, and
llama-server's API (`/props`, `/v1/models`, `/slots`, and its newer
`--models-dir` router mode) before the Ollama adapter was written, not after
— D-3 asked for the interface to be defined once, for every runtime the PRD
names, and getting that wrong is a rewrite, not an edit, once a second
backend exists. The seam D-3 called out as most likely to leak an Ollama
assumption — what a model *is* to fetch — is `Pull`'s `ModelSource`: a
`SourceKind` (`ollama_tag` or `huggingface_gguf`) with only the fields for
that kind set, and `ErrUnsupportedSource` when a backend cannot fetch from a
kind, never a best-effort translation between an Ollama tag and a Hugging
Face repo. `Generate` stays deliberately thin (no chat history, tools,
images — D-4) because its only caller is the benchmark harness (step 6).
`ModelInfo.Details` carries a model's GGUF metadata with the architecture-
prefixed keys the file format itself uses (`<arch>.block_count`,
`<arch>.attention.key_length`, …), not Ollama's shape — Ollama's `/api/show`
already re-exposes those keys close to verbatim, so this is reading the
file format, not the runtime.

**Consequences.** `internal/backend/ollama` is the only implementation; nothing
in the interface references Ollama's Modelfile concept except the two fields
(`Modelfile`, `Template`) that only Ollama populates, left empty by any
backend without the concept. A llama.cpp or LM Studio adapter (step 4/18)
implements the same ten methods against its own API with no interface
change expected — the open item this closes.

## D-28. The Ollama adapter: HTTP first, filesystem as fallback only

**Decision.** `internal/backend/ollama` talks to Ollama over its HTTP API on
`OLLAMA_HOST` (default `http://127.0.0.1:11434`). `Detect` tries
`/api/version` first — the only way to learn the running version — and only
falls back to looking for the binary on `$PATH` or this app's managed
install location when that fails; the filesystem check never starts
anything, matching the interface's "Detect must be cheap and must never
start anything." `Pull` streams `/api/pull` and reports progress in bytes
(`completed`/`total` from Ollama's own NDJSON), so the UI can say "2.1 of
4.7 GB" per the task, not a percentage with no denominator until Ollama has
sized the download. `Unload` is a `/api/generate` call with `keep_alive: 0`
and no prompt — Ollama has no dedicated unload endpoint; that is the
documented way to free a model's memory now. The HTTP client, the wire
shapes, and the `model_info` parsing helpers (`miValue`/`toNum`, D-20's
"prefer `attention.key_length`, max over a per-layer array") are ported from
`scripts/probe0/main.go`, which validated them against real machines across
three runtime backends in step 0 — not re-derived from the docs alone.

**Consequences.** A `TestShowPrefersKeyLengthOverSpec` regression test keeps
D-20's finding enforced at the adapter boundary, not only in the estimator
that will consume it (step 5). `Detect`'s 2-second timeout means a stalled
Ollama process reads as unreachable quickly rather than hanging a page load.

## D-29. Install and Start: a button, per OS, no sudo, no system service

*Superseded in part by D-66: Ollama does publish a checksum with every
release (`sha256sum.txt`), and the install is now pinned to a release and
refused unless the file matches it.*

*Superseded in part by D-72: where Ollama's app is installed, Start opens
the app (hidden) instead of running `ollama serve`, because the app stops
any server it did not start.*

**Decision.** Per product rule 5, `Install` and `Start` only ever run from a
UI action that already said what it would do; the methods themselves change
nothing until called. macOS and Windows: download the official installer
over HTTPS to a temp folder with progress, verify what can be verified (the
host, TLS, a published checksum where Ollama publishes one — it does not,
today, and the code says so rather than pretending to check one), then
launch it and wait for `Detect` to change; the user still clicks through the
installer, because this app cannot and should not silently accept another
program's EULA for them. Linux: a genuine user-space install — the official
`ollama-linux-<arch>.tar.zst` extracted under this app's own data folder,
run later as a child process this daemon supervises (`ollama serve`,
output to a log file `runtimepath.go` also reads) — no `sudo`, no
`curl | sh`, no system service, matching the Linux convention CLAUDE.md
already sets. Both the version this app installed and the version `Detect`
subsequently reports are kept (`backends.installed_version` vs. the row's
`version`), so "what we put there" and "what is actually running" can
disagree visibly if the user upgrades or replaces it themselves.

**Consequences.** The user-space Linux install cannot start at boot or run
as a background service outside this app — a system service would need a
password prompt, and product rule 1 ("the user never needs a terminal") and
this product's customer (CLAUDE.md: someone who "has no idea what a GGUF
is") make never asking for a password the higher priority. `Install`'s
progress callback and `Pull`'s share the same shape (`Status`, `Completed`,
`Total` bytes) so the UI can reuse one progress-bar component for both.

## D-30. The inventory: four states, refreshed on every check, never deleted

**Decision.** `backend.State` has four values — `not_installed`,
`installed_not_running`, `running`, `unsupported` — so "installed but not
running" and "not installed" are different values the API returns, not a
boolean the UI has to combine with a guess (the task's own requirement).
`GET /api/backends` calls `Detect` (and, when a backend is running,
`Models`) fresh on every request rather than serving a cached value: `Detect`
is documented to be cheap and to never start anything, so re-checking on
every page load is the point. Each check is stored as a new `backends` row
(insert-only, mirroring `hardware_profiles` — D-23's history-not-update
shape), so a benchmark can still answer "what version of Ollama was this
run against" after an upgrade. `installed_models` is upserted the same way
on every successful `Models()` call: existing rows for that backend are
marked `present = 0`, then every model the runtime reports is inserted or
updated back to `present = 1` — a model the user removed from Ollama drops
out of `GET /api/models/installed` but its row, and its name, stay
referenceable by history (a past benchmark, a past estimate), never deleted.

**Consequences.** `GET /api/backends` on a `Detect` failure (a real error,
not "not installed") falls back to the last stored row rather than
reporting a guessed state — D-21 applies to a runtime's state exactly as it
does to a hardware value. `GET /api/models/installed?backend=<name>` filters
to one runtime; without the parameter it spans all of them, backend then
name ordered.

## D-31. The runtime path is a fact established after a load, never assumed

**Decision.** `hardware.RuntimePath` per GPU index on a backend's `Status`
(`cuda`/`metal`/`rocm`/`vulkan`/`cpu`) is empty until a model has actually
loaded — it is what happened, not `hardware.GPU.ExpectedBackend` (D-24's
rule-derived expectation) repeated in a new place. After a load, it is read
two ways and only trusted when they agree with what evidence exists:
`/api/ps`'s `size_vram` says whether anything is on a GPU at all (0 means
CPU, unambiguously), and the server's own log — read from the file this
daemon wrote when it started Ollama itself, or Ollama's documented default
location otherwise — is parsed for its device-discovery lines
(`library=cuda`, `ggml_vulkan: Found …`, `no compatible GPUs were
discovered`, …; the last matching line wins, since a failed probe of one
backend followed by a successful one is a real sequence Ollama's own log
produces). When VRAM is used but the log cannot confirm which named backend
did it, the path is left out of the map entirely — D-21 again: an omitted
entry, never a guessed one. The environment variables that steer which path
a runtime takes (`OLLAMA_VULKAN`, `GGML_VK_VISIBLE_DEVICES`,
`HSA_OVERRIDE_GFX_VERSION`) are captured on every `Detect` — read, never
set; this app has no UI to set them yet, and D-21 forbids guessing a good
value even if it did — and only variables actually present in the
environment are included.

**Consequences.** `GET /api/backends` returns `runtime_paths` beside
`expected_backend` (on the hardware profile), so the UI can show "expected
CUDA, actually ran on CUDA" or flag a mismatch, once step 11 builds that
screen. The log-parsing logic (`pathFromLog`, `runtimePathsFromPS`) is
ported from `scripts/probe0/main.go`'s validated detector, not re-derived.

---

---

Step 4 (the model catalogue, 2026-09-19) adds D-32 to D-37.

## D-32. The catalogue's schema: sizes carry what the estimator cannot guess

**Decision.** `families.yaml` keeps the shape step 1 sketched (family: id,
display name, maintainer, licence, purposes, reviewed date; size: parameters,
context length, Ollama tag, Hugging Face repo) and adds four things:

- a top-level `quants` list — the quant variants the advisor tracks
  (`Q3_K_M`, `IQ4_XS`, `Q4_K_M`, `Q5_K_M`, `Q6_K`, `Q8_0`, and `MXFP4` for
  models released in it). A repo publishes twenty-odd quants; a beginner is
  choosing between a handful, and every tracked quant is a header read;
- `source` per family — the model card the entry was checked against,
  beside `reviewed_at` (CLAUDE.md: data carries its date and its source);
- `active_parameters` per size — set for mixture-of-experts sizes and for
  Gemma's per-layer-embedding "E" sizes, omitted for dense ones;
- `notes` per family for the curator and for step 5 (hybrid attention,
  multi-head latent attention, sliding windows).

Mixture-of-experts and multimodal families are **in** the catalogue. The
step 1 header comment excluded them "until step 5 handles them"; that is
superseded here, because a careful person recommending local models in
September 2026 recommends several of them first, and the estimator cannot
learn to handle models the catalogue does not carry. They are marked
(`active_parameters`, purpose `vision`, the header's `expert_count`,
`full_attention_interval`, projector files), so step 5 can decide per
model, and D-20 finding 5 still keeps them out of its gate until it does.
The Llama 3 sizes stay although they are older than the rest: they are the
most-installed family and the models step 0 calibrated on.

`hf_repo` is an ungated repo publishing the tracked quants under llama.cpp's
standard names — bartowski's where one exists, else ggml-org or the
maintainer's own — with the vision encoder as an `mmproj` file. `load.go`
validates everything it can offline (`advisor catalog check`); whether each
repo resolves is the refresh's report.

**Consequences.** Adding a model is a YAML edit plus `advisor catalog
refresh`. The purpose enum is checked against `ui/src/api/types.ts` by a
test, and a test fails if any of the docs counts the catalogue (rule 8).

## D-33. A header read stops at the tokenizer

*Superseded in part by D-37: `general.file_type` is no longer a required
key, because the live gate showed current quantizers write it last.*

**Decision.** `internal/catalog/gguf` parses a GGUF header from any
`io.Reader`: magic, version (2 and 3; version 1 and big-endian files are
refused with a sentence), tensor count, then the key/value metadata in
order. `tokenizer.*` values are skipped, never stored. With
`StopAtTokenizer` the parse ends at the first `tokenizer.` key once every
required field (architecture, file type, block count, context length,
embedding length, head count, KV head count) has been read — llama.cpp's
writer puts all of them first, so a header read is one 64 KiB range of a
multi-gigabyte file. When a writer put a required key after the tokenizer
(Ollama's own files put `general.file_type` last), the parse reads on to
the end rather than stop without it: same answer, more bytes.

The tokenizer is most of a header — 6 to 11 MB of vocabulary and merges —
and none of the estimator's inputs. Reading every tracked quant's full
header would cost well over a gigabyte per first refresh on the customer's
connection; stopping costs about ten megabytes for the whole catalogue.

A header that is not GGUF fails loudly (`*gguf.FormatError`, wrapping
`ErrMalformed`, with the byte offset and key); one that is merely cut short
fails with `ErrTruncated`. Limits no real file reaches (64 Ki metadata
pairs, 64 MiB strings, 64 Mi-element arrays, 4 Mi tensors) keep a hostile
header from asking for gigabytes. The typed view (`Metadata`) takes the
largest value of a per-layer array (probe0's convention) and keeps the
list, uses `head_count` when `head_count_kv` is absent — llama.cpp's rule,
flagged `head_count_kv_stated: false` — and keeps `key_length` (D-20).

**Consequences.** Tests parse real headers captured from the fleet (the
first megabytes of Ollama blobs, `gguf/testdata/`), including one whose
file type comes after the tokenizer and one hybrid architecture, plus
synthetic malformed ones. `catalog_files.header_complete` says which kind
of read each row came from; `header_json` holds every pair that was kept.

## D-34. The Hugging Face client is polite, header-only and cached

*The host list moved to the one allow-list (D-64); `hf.allowedHost` now
reads it.*

**Decision.** `internal/catalog/hf` makes two kinds of request. The
model-info listing (`/api/models/{repo}?blobs=true`: files, sizes, LFS
hashes, the Hub's own parameter count) is sent with `If-None-Match` and its
body cached per repo (`hf_listing_cache`), so an unchanged repo is a 304.
Header reads go to `/{repo}/resolve/{commit}/{file}` — pinned to the
listing's commit — as range requests from byte 0 in doubling chunks
(64 KiB up to 8 MiB) that the parser consumes as they arrive. A server that
answers a range with the whole file is refused unread (unless the file is
smaller than the range asked for). Parsed headers are cached by (repo,
file, LFS sha256) in `hf_header_cache`: a file's content hash, not the
repo commit, so a README edit costs nothing.

Requests are sequential with a minimum gap; 429 and 5xx honour
`Retry-After` or the Hub's `RateLimit` header (`t=`), up to a bounded wait
and a bounded number of attempts; a longer requested wait fails with
`ErrRateLimited` rather than stall. Redirects are followed only to
`huggingface.co`, `hf.co` and their subdomains (the LFS and Xet CDNs), over
https. When every attempt fails below HTTP, the error is `ErrUnreachable`
and the refresh stops instead of failing every size slowly. No token is
ever sent: a gated repo is a curator error ("pick an ungated repo"). The
User-Agent is `local-llm-advisor/<version>`.

**Consequences.** A second refresh of an unchanged catalogue makes one
request per repo and reads nothing. The host list lives in
`hf.allowedHost`, which step 12's allow-list audit reads.

## D-35. Refresh: one row per tracked quant, never deleted, one at a time

**Decision.** `internal/catalog/refresh.Run` syncs the YAML into
`catalog_models` (one row per family size; sizes that left the YAML are
marked `present = 0`), then per size: listing → files grouped (split
`-0000N-of-0000M` parts summed, part 1's header read; imatrix, multi-token
prediction and draft files ignored) → one weights file per tracked quant and
one vision encoder (F16 preferred) → headers → `catalog_files`. Migration
0003 adds the columns step 5 needs beyond schema v0 (`value_length`,
`full_attention_interval`, `expert_used_count`, `head_count_kv_stated`),
the file's `role` (`model` | `projector`) and `parts`, `present` flags on
both tables, and the size's refresh state (`hf_sha`, `parameters_counted`,
`refreshed_at`, `refresh_error`). `bits_per_weight` is bytes × 8 over the
Hub's own parameter count when it gives one, else the model card's.
`file_type` is −1 when a header does not state it (0 is F32). A size that
fails keeps what an earlier refresh stored; its error is recorded in
words. Disagreements that do not stop a size — the YAML's context length
against the header's, a file named `Q8_0` whose header says `Q4_K_M` — are
warnings in the report. Each refresh is a `catalog_refreshes` row.

It runs from `advisor catalog refresh` (the curator's command; exits 1 if a
size did not resolve) and `POST /api/catalog/refresh`, one at a time (a
second gets 409), detached from the request so closing the tab does not
abandon it. The daemon syncs the YAML at every start (no network), so a new
build's catalogue exists before its first refresh. Step 10 calls the same
`Run` nightly.

**Dependency direction.** `store` imports `catalog` for its types (as it
does `hardware` and `backend`); `catalog/refresh` imports `store`, `hf` and
`gguf`; `catalog` itself imports neither `store` nor `hf`.

## D-36. Installed models map by family, size and quant; unknown is a signal

**Decision.** `catalog.MatchInstalled` maps an installed model onto the
catalogue: a Hugging Face pull (`hf.co/owner/repo:quant`) by its repo; else
the exact Ollama tag; else the library name before the colon as the family
and the size whose parameter count is nearest the runtime's
`parameter_size` (within 25%) — which is what makes `llama3.2:latest` and
`qwen3.5:9b-q8_0` land on the right size — and then the quant picks the
file. The result is `file` (known size and quant), `model` (known size, a
quant the catalogue does not track or has not read yet) or `unknown`, with a
sentence for the curator, stored on `installed_models` (`catalog_model_id`,
`catalog_file_id`, `catalog_match`, `catalog_note`). It runs after every
inventory refresh and every catalogue refresh, costs no network, and
`GET /api/catalog/unknown` lists the unknown ones — the curator's signal,
never an error (D-7).

## D-37. The file type is not worth the tokenizer (the gate's finding)

**Decision.** `general.file_type` leaves `gguf.RequiredKeys`. The first live
refresh (the M1 Pro, 2026-09-19: 22 of 22 sizes resolved, 139 files) read
1.5 GB of headers instead of the ~10 MB D-33 predicted: current llama.cpp
quantizers write `general.file_type` and `general.quantization_version`
after the tokenizer (setting a key removes it and appends it at the end), so
nearly every file failed D-33's early-stop condition and was read through
6–11 MB of vocabulary. The file type is a cross-check, not an estimator
input: the quant a user sees and pulls is the one in the file name, and the
bytes come from the listing. A read now stops at the tokenizer once the
structural keys are in (architecture, block count, context length,
embedding length, head count, KV head count — all written before the
tokenizer); a file type stated earlier is kept and checked against the name,
one stated later is not read and is `file_type = -1` ("not stated", D-21).
A required key after the tokenizer still makes the read go on.

**Consequences.** A first refresh of the whole catalogue is one 64 KiB range
per file again (about 10 MB); headers already cached by a full read stay
valid. The name-versus-header warning now fires only for files that state
their type early. The fixture whose writer puts the type last
(`minicpm-v4.6`) is the test.

---

Step 5 (fit estimator + recommendation engine, 2026-09-19) adds D-38 to D-43.

## D-38. The memory estimate is D-20's formula, generalised through a layer layout

**Decision.** `estimate.Fit` computes `weights + KV + overhead` exactly as
D-20 states it, and `internal/estimate/step0_test.go` replays every step 0
row through it: the recorded result is pinned (27 of 28 dense rows within
15%, worst row −20.8%), so a change to the formula is a change to D-20, not
a refactor. What D-20 could not cover is most of the catalogue: models whose
layers are not a plain stack of identical attention layers. `catalog.Layout`
(`internal/catalog/layout.go`) describes what each layer keeps as the
context grows, and the cache term sums over it:

- hybrid models keep a cache on some layers and a small fixed state (kept in
  32-bit floats) on the rest — `full_attention_interval`, a per-layer
  `recurrent_layers` list, or per-layer `head_count_kv` with zeros;
- sliding-window layers stop growing at `window + n_ubatch` cells, rounded up
  to 256 — per-layer `sliding_window_pattern` where the header states one
  (with `key_length_swa` for the shorter sliding heads), else the period
  llama.cpp hard-codes for the architecture;
- the last `shared_kv_layers` layers own no cache; `nextn_predict_layers`
  appended to a file are not run and keep nothing (this is the "block_count
  is one more than the model card" of step 4's gate);
- multi-head latent attention (`key_length_mla` present) caches one
  compressed key per token and no value;
- keys and values may differ in length: the term is `key_length +
  value_length`, which is D-20's `2 · head_dim` wherever they are equal.

Every rule follows llama.cpp's own loader and cache code (checked against
ggml-org/llama.cpp 5b59b83, 2026-09-19) — the runtime's arithmetic, not a
guess at it. A Layout is **derived on every read** from the typed columns
plus `header_json`, never stored, so a better reading of a header needs no
catalogue refresh and the rows a step 4 build wrote serve as they are. Its
`Basis` says how far it can be trusted: `uniform` (the shape step 0
measured), `stated`, `architecture` (the header states a window and the
pattern is the one llama.cpp hard-codes for the architecture), or
`incomplete` (something the layout needs is missing; what could be read is
used and the note says what could not). An architecture llama.cpp has no
sliding-window pattern for is run with full attention whatever window its
header mentions — step 0's phi3 rows — so the plain formula is exact there.

Around the formula: a vision encoder's bytes are added to the weights (the
runtime loads it beside them; step 0's one multimodal model had it
resident); Gemma's per-layer-embedding sizes keep their lookup tables in
system memory by design, so only `bytes × active ÷ total` counts against the
graphics memory and the rest is reported as living off the card without
making the model "split"; a mixture of experts needs every expert resident.
A quantised cache uses GGML's block sizes (34/32 and 18/32 bytes an element).

**The five categories, and the threshold that decided.** Against the
placement's budget (D-39): at or under `HeadroomFraction` (80%) fits with
headroom; at or under `FitsFraction` (92%) fits; over that, a shorter context
from the ladder that fits makes it `reduced_context_only` (with
`suggested_ctx`) — looked for *before* splitting, because a shorter context
keeps the whole model where it runs fastest; otherwise whole layers go to the
graphics until it is full and the rest must fit in RAM less the OS reserve
(`needs_cpu_offload`), else `not_recommended`. `Estimate.Threshold` is the
comparison in words. A sixth value, `unknown`, exists for a budget that
could not be read (D-21): never a fit, never a misfit.

**Every constant is in `estimate.Config`**, each marked MEASURED (fleet),
MEASURED (public) or CHOSEN, with what would settle the chosen ones. The
overheads are step 0's medians. The two fractions, the OS reserve and the
processor's speed range are CHOSEN and say so.

**Evidence beyond the gate.** Step 0 measured one hybrid model on Metal and
Vulkan and excluded it as multimodal; with the layout its four rows land
within 10% (with every layer counted the long-context rows over-predict by
40%), and the test holds them to 15%. Its Gemma rows on CUDA grow by 17 KB
per token of context, against 16 KB for a shared-cache, mostly-sliding
layout. That is support, not validation: `Basis.MemoryModel` is `validated`
only for the shape inside step 0's gate, `modelled` for everything above,
and the confidence (D-42) follows it.

## D-39. Placement: which memory, which path, and what the runtime was seen to do

**Decision.** `Estimator.Place` decides once per machine where a model would
run and what it is compared against, so Fit and the engine cannot disagree:
Apple Silicon against `gpu_usable_bytes`, never RAM (D-25); a graphics card
against the largest single device (D-20 finding 8); a machine whose models
run on the processor against RAM less `Config.OSReserve`; graphics built
into a processor against system memory too, at that memory's speed, and
without being called "a graphics card that is not used". The runtime path is
the one the backend **established** after a load (D-31) when there is one,
else the rule-derived expectation (D-24), and the estimate says which
(`Basis.PathSource`).

When a graphics card exists and the plan is nevertheless for the processor,
`Placement.Unused` says which of four things is true — seen running on the
processor, cannot be used as set up, not known yet, memory unreadable — with
the why as far as the facts go: the support rule's own sentence, Vulkan
switched off when `OLLAMA_VULKAN` says so, otherwise the usual causes named
as such. An operating system older than the runtime supports blocks
everything and the result carries the profile's sentence.

An observation outlives the load: most backend checks see no model loaded
and record no path, so the engine plans against the **last check that saw
one, on this hardware**. Migration 0004 adds `backends.hardware_fingerprint`
for that — what Ollama did with the old graphics card says nothing about the
new one, and timestamps at one-second resolution cannot tell a swap from a
restart. `GET /api/recommend` and the fit endpoint re-check the runtimes
first (Detect is cheap and starts nothing — D-30), so a model loaded a minute
ago in the user's chat app is what the answer is built on.

## D-40. Speed is a range, from memory bandwidth, and absent when the part is unknown

**Decision.** `generation tok/s ≈ bandwidth ÷ bytes read per token ×
efficiency`, both ends estimated: the high end reads only the weights a token
uses (an empty context), the low end adds the whole cache at the context
asked for (a full one). Prompt processing is estimated separately — it is
bound by arithmetic, so it is the part's generation speed on the reference
model scaled to this model's active parameters, times the path's measured
prompt-to-generation ratio. `estimate.Speed` carries both as `*figure.Rate`
with `Low < High`; when the part is not in the table they are **absent** and
`Unknown` is the sentence — a missing number is a sentence, never a zero and
never a default. `WithMeasurement` turns a rate into a measured point and
flips its source: product rule 4's second sentence, ready for step 6.

Bandwidth is data: `data/hardware/gpus.yaml`, embedded and strictly decoded
like the other data files. Graphics rows carry vendor, name patterns,
optional memory-size and GPU-core conditions (the same name ships with
different memory), the vendor's bandwidth, and — for GDDR parts — the data
rate and bus width it is derived from; the parser refuses a row whose
bandwidth is not `rate × width ÷ 8`, so a typo cannot survive. Rows are
ordered (laptop parts above desktop parts of the same name); a name that
lists several cards, which is how Linux's pci.ids names a chip id, takes the
slowest of every matching row and says so. `system_memory` rows give a
processor family's supported memory as a **range** — one module of the
slowest supported speed to every channel at the fastest — because the
advisor does not read what is installed; that alone makes the processor's
estimate the widest.

Efficiency and prompt ratio are ranges keyed by the runtime path
(`estimate.Config.Paths`), Vulkan refined by vendor because its three
populations disagree. As shipped they come from llama.cpp's own llama-bench
scoreboards (read 2026-09-19) and each entry's `Basis` lists the figures;
parts a scoreboard shows outside their path's range get a per-row override
in gpus.yaml with its source (the two-die Apple chips, the Max chips, an HBM
card). Mixture-of-experts models read `bytes × active ÷ total` per token and
are scaled by a measured factor. The processor's range is CHOSEN — nothing
public covers it — and is the first thing `scripts/calibrate` exists to
replace: it turns a llama-bench run on a fleet machine into the same two
factors, says whether they fall inside the configured range, and writes a
results file to commit. It is the dev-side instrument; the customer's
measurement is step 6's benchmark of Ollama itself.

## D-41. Recommendation is a product of four factors over the default download

**Decision.** `score = purposeFit^wP × fitFactor^wF × speedFactor^wS ×
sizeFactor^wZ`, every weight and threshold in `recommend.Config`. A product,
so a zero anywhere is a veto.

- *Purpose fit* reads the order of a family's `purposes` in families.yaml —
  most credible first, a gentle step per rank (the file's header now says the
  order is read) — averaged over the purposes asked for, zero for a purpose
  the machine cannot serve (long documents at a short context, images without
  an encoder), and scaled down when the context has to be shorter than the
  purpose wants. The engine chooses the context: the longest on the ladder,
  up to what the purposes want, that still fits wholly.
- *Fit* is the category's worth. Models that only run split compete only
  when nothing fits at all (or `allow_split` is asked for): a beginner is not
  steered to a model several times slower while one that fits exists.
- *Speed* is the geometric middle of the estimated range over a comfortable
  reading speed, capped at 1, falling all the way down below it and weighted
  1.5 — which is what sends small models to weak hardware (product rule 6)
  instead of the largest model that happens to fit in RAM.
- *Size* is the only quality proxy the catalogue carries until step 9b: log
  of effective parameters (total; √(total × active) for experts; the model
  card's effective count for per-layer-embedding sizes). It is what keeps a
  24 GB card from being handed a 1B model.

The engine recommends **one file per size: the one its Ollama tag pulls**
(`DefaultQuants`). The Ollama adapter cannot pull another quant yet, and
choosing quants automatically is PRD §18. At most three cards, one per
family. Every sentence on a card is templated from the facts the rules used
(`reasons.go`, D-8), in a fixed order — the graphics card that is not part of
the numbers first, with its explainer id and the why; then fit, purpose,
speed, cost, change — and a test holds the copy rule: none of the glossary's
terms reaches a reason.

If the user has a model, the one that serves the purposes best (or the one
named) is assessed like a candidate, and a recommendation must name a
noticeable change against it — size, speed, a purpose its family is not for,
a context at least twice as long, fitting where it does not — or be dropped;
the model itself is never recommended to its owner, and an installed model
that is a real change costs "nothing to download". When the list is empty
the result says why in words and as a code (`empty_code`); the UI offers the
one action that fixes it — fetching the model list, with its cost on the
button.

**Consequences.** `internal/recommend/recommend_test.go` holds the outcomes
the weights have to produce on the golden hardware profiles — the fleet's
top picks among them. A change to a weight is judged by those outcomes, and
by Itay reading `advisor recommend` on each machine.

## D-42. Confidence is derived, three-valued, and says what limits it

**Decision.** From `estimate.Basis`: **low** when something is unknown (no
speed estimate; a model description the memory arithmetic could not read in
full); **high** when the memory arithmetic has been measured
(step 0's shape, or a benchmark of this configuration), the runtime has been
seen taking this path, and the speed is measured or is an estimate on a path
whose parts behave alike (cuda, metal); **medium** otherwise — all inputs
known, at least one only expected, modelled or wide. `confidence_why` names
the limiting inputs in plain words and the UI shows both on every card. Most
of today's catalogue is therefore medium at best until step 6 measures it,
which is the honest reading of PRD §21's last risk.

## D-43. The API for step 5, and what is deliberately not stored yet

**Decision.** `GET /api/recommend?purposes=a,b[&current=][&min_context=]
[&gpu_only=1][&allow_split=1]` → `recommend.Result`; no purposes means
everyday chat. `GET /api/models/{id}/fit[?ctx=][&kv=]` → one estimate per
tracked weights file of a catalogue size, `{id}` being the size's id in
`GET /api/catalog`; without `ctx` the context is what Ollama itself would
use on this machine (4k, 32k or 256k by graphics memory — its
`server/routes.go`), and the response says which. Both wait for hardware
detection like `GET /api/hardware`. `advisor recommend` prints a running
daemon's answer as text for the developer; `scripts/verify.command` and CI
call it.

Estimates are computed per request and **not written to `estimates`**: the
table's rows exist to be flipped to `measured`, and writing on a GET would
make every page view a write. Step 6 owns the row: it inserts or updates it
when a benchmark completes, and passes what it measured to the engine through
`Engine.Measurements` / `Estimate.WithMeasurement`.

**Dependency direction** (D-11, made concrete): `estimate` imports `catalog`,
`hardware`, `figure` and `data`; `recommend` imports `estimate`; `server`
imports both; `scripts/calibrate` imports `estimate`. `hardware` gains exported
helpers (`MatchName`, `HumanGB`, `PrimaryGPU`, `AppleGPUCores`) and no
dependency.

---

Step 6 (benchmark harness, 2026-09-19) adds D-44 to D-49.

## D-44. The suite is data: the advisor's own text, sent raw, versioned and pinned

*The suite moved to `internal/suite`, where a prompt is a sealed type only
the embedded suite can make (D-65).*

**Decision.** `data/bench/suite.yaml` (schema in its header, mirrored by
`bench.Suite`, decoded strictly) names the text (`text.txt`: original
English prose written for the suite — an allotment year, no markup, plain
ASCII so every tokenizer sees the same bytes), three prompts cut from it by
paragraph count (≈ 475, 2,047 and 7,408 tokens with Llama 3's tokenizer — a
reference count; each model's own count comes back from the runtime and is
kept), the answer's budget (256), temperature 0, a fixed seed, one warm-up
and three timed requests per prompt. Nothing the user typed is ever sent
(product rule 7): the harness has no input for text at all.

How a request is sent, and why (checked against Ollama v0.34.2's source,
`server/routes.go` and `llm/llama_server.go`, and the llama.cpp it ships,
b10969):

- **raw**: no chat template, no system prompt. Templates differ per model and
  would add a different number of tokens to each; raw times the model on the
  suite's text and nothing else. The text ends mid-essay, so the model
  continues the prose and uses the whole budget.
- **a numbered lead line** (`"{n}."`, n = the request's number in the run):
  Ollama runs llama-server with `cache_prompt: true`, which reuses the prefix
  a request shares with the previous one on the same slot — three identical
  prompts in a row would time one token of reading. The number makes every
  request differ at its first token; what is still reused (the begin-of-text
  token) is reported as `prompt_eval_cached_count` and left out of the rate.
- **`truncate: false`, `shift: false`**: with truncation on, Ollama cuts a
  prompt longer than the context without saying so, and the harness would time
  a different prompt. Off, the runtime refuses it, and the prompt is skipped
  with the runtime's words as the reason.
- Before a run, a prompt whose reference count plus the answer plus a margin
  (`bench.Config.ContextMargin`) exceeds the context is planned out; after the
  warm-up, the model's own tokens-per-word ratio re-checks the rest.

The suite's version is part of every run's comparability key (D-46), and
`suite_test.go` pins each version to a digest of the YAML and the text: an
edit without a version bump fails the build.

**Consequences.** At Ollama's default context on machines under 23 GiB of
graphics memory (4k), the long prompt does not run; a run at 8,192 or more
runs all three. The prompt lengths are a reference, not a promise, and the
results carry each model's own counts.

## D-45. A run: one at a time, timed by the runtime, and a cancel that unloads

**Decision.** `bench.Harness` runs one benchmark at a time (a second request
is 409, never queued), detached from the HTTP request that started it —
closing the page does not stop it; cancel does. A run is:

1. **prepare** — if this model is loaded (by the user's chat app, at another
   context) it is unloaded; other loaded models are noted on the run; after
   `Config.Settle` the sampler (D-47) takes its baseline;
2. **warm-up** — the first prompt, untimed: the load and everything done
   once. Its `load_duration` is kept as the run's load time;
3. **observe the load** — what the runtime said about it (D-46), where it put
   the model (`/api/ps` size vs size_vram → gpu, split or cpu; the log's
   "offloaded N/M layers" wins when the two disagree), and a fresh `Detect`,
   recorded in `backends` like any check, so the path a benchmark's load
   established is what the recommendation engine plans against next (D-39);
4. **timed requests** — three per prompt; generation = `eval_count /
   eval_duration`, prompt = `(prompt_eval_count − cached) /
   prompt_eval_duration`, both Ollama's own counters; time to first token at
   the client, request sent to the first streamed chunk of answer (or of
   reasoning — it is output too). Each prompt's result is the median and the
   spread, (max − min) ÷ median, with notes when the runs disagree by more
   than 5% (the gate's tolerance), the answer stopped early, or the cache was
   reused;
5. **unload** — with a context of its own, so a cancelled run still does it:
   ask, then poll `/api/ps` until the model is absent on two readings in a
   row, asking again whenever it reappears — a load still in progress when a
   run is cancelled finishes and shows up after the first request. The run
   records `unloaded` true, or false with why.

The headline of a run is the shortest prompt's result: a short prompt and an
almost empty cache are what the estimator's speed figures describe
(llama-bench's 512-token prompt). A daemon that stops mid-run leaves a row
that says "running"; the next start marks it failed (`Recover`).

**The backend interface grows by two fields and one optional interface**
(D-27 said `Generate` stays thin; it does): `GenerateRequest.Raw` and
`NoTruncate`, which every runtime D-27 checked has (llama-server's
`/completion`, LM Studio's `/v1/completions`), and `GenerateEvent.Thinking`
and `PromptEvalCached`. `backend.LoadObserver` is optional: a backend that
can read its own load (Ollama: the server log) implements it; one that
cannot leaves the run's path, cache type and flash attention unknown.

## D-46. What makes two runs comparable, read from the runtime, never assumed

**Decision.** `bench.RunConfig` is the whole context of a run, and
`RunConfig.Key()` is a digest of the part that decides comparability: the
hardware fingerprint, the backend and its version, the runtime path the load
took, the model's digest (else name, quant and size), the context asked for,
the cache type, flash attention (unknown is its own value), the parallel
slots, and the suite's version and digest. The daemon's version is stored,
not keyed: what the harness does to time a request belongs to the suite's
version. Runs are compared — "−0.4% against run 12" — only when their keys
are equal; a vulkan run and a rocm run on one card are two configurations.

The runtime path, the cache type, flash attention, the layers offloaded and
the context the runtime was started with are **read** from the runtime's own
account of the load: Ollama v0.34 runs llama-server with `--log-verbosity 4`,
so its load lines reach Ollama's server log — `offloaded N/M layers to GPU`,
`<device> model buffer size` (CUDA0, ROCm0, MTL0, Vulkan0, CPU_Mapped: the
device every part of the weights went to), `flash_attn = auto` and
`resolve_fused_ops: Flash Attention enabled` (auto alone decides nothing),
`llama_kv_cache: … K (f16) … V (f16)`, and Ollama's `starting llama-server
… -c N -np P`. `ObserveLoad` marks the log's length before the warm-up and
reads only what follows; on Linux under systemd it reads the journal from
the same moment. Where nothing can be read, each value is `unknown` (D-21):
the run is stored and compared as unknown, and does not replace an estimate
(D-48). `pathFromLog` (D-31) learned the model-buffer lines too, and — when
`/api/ps` already says the model is in graphics memory — ignores the
processor's lines, which builds that load backends as libraries print last.

**Consequences.** Migration 0005 adds what schema v0 lacked:
`hardware_fingerprint`, `model_digest`, `flash_attention_known`,
`config_key`, `config_json` (the whole RunConfig and the run-level figures),
`model_json` (the header facts the estimator used, D-48), `estimate_json`
(the estimate the run is shown against), `notes_json`, `resident`, the
runtime's own `ps_size_bytes` / `ps_size_vram_bytes` (its estimate, kept as
context and as evidence for where "fits" ends — D-20 finding 4),
`effective_ctx`, `memory_source`, `unloaded`, `measure_anyway`, and a
`device` column on samples.

## D-47. The resource sampler: the counters that exist, without root, and words where none do

**Decision.** Once a second (`bench.Config.SampleInterval`) every probe the
machine has reads its tool, through a `sysEnv` seam the tests answer from
fixtures in each tool's real format (`internal/bench/testdata/sampler/`):

| | graphics memory (the footprint's counter) | also |
|---|---|---|
| NVIDIA (Windows, Linux) | `nvidia-smi` memory.used, summed over cards | utilisation, temperature, power |
| AMD on Linux | amdgpu sysfs `mem_info_vram_used` + `mem_info_gtt_used` (D-20 finding 6: part of a Vulkan model lands in GTT) | `gpu_busy_percent`, hwmon temperature and power |
| Apple Silicon | `vm_stat` wired memory (D-20 finding 6: it tracks a model; ioreg's figure does not) | ioreg's "Device Utilization %", memory in use |
| everything | — | system memory (`/proc/meminfo`, `vm_stat`, `GlobalMemoryStatusEx`), and the runtime's `/api/ps` |

rocm-smi and amd-smi read the same sysfs counters; reading them directly
needs no tool, no output format and no root. `powermetrics` (a Mac's
temperature and power) needs root and is not used. Where nothing reads the
graphics memory — AMD and Intel cards on Windows (D-6), Intel on Linux,
graphics built into the processor, an NVIDIA card without nvidia-smi — the
run's `sampler_note` says so in words; nothing is shown as zero.

The model's footprint (`peak_vram`) is the counter's peak rise over its
baseline before the load, summed over devices: step 0's measurement, taken
continuously. It is not reported when another model came or went during the
run (`/api/ps` is watched for that), and is marked partial when the model was
split. Utilisation and power are means over the timed requests; temperature
is the hottest reading; system memory is the peak in use (absolute: a mapped
model sits in the page cache, which Linux counts as available).

## D-48. A measurement replaces its estimate, and narrows the others

**Decision.** Product rule 4's second sentence, twice over:

- **The configuration measured.** When a run finishes, the `estimates` row for
  (this hardware profile, the catalogue file, num_ctx, the cache type read,
  the path read) is inserted or updated in place with `source = 'measured'`,
  the measured rates (low = high), the measured footprint where the model was
  wholly on the graphics, and `measured_run_id` — D-13 and D-43 said step 6
  owns the row; it does. Only a configuration the estimator can be asked
  about is written: a model the catalogue maps to a file (step 4), and a
  cache type and path that were read. `GET /api/recommend` and
  `GET /api/models/{id}/fit` read the latest measurement of every
  configuration on this hardware (by fingerprint, across daemon starts) and
  pass it through `Estimate.WithMeasurement`: the speed becomes a point, the
  memory the measured figure, the confidence high (D-42).
- **Everything similar.** `estimate.Calibrate` turns every finished,
  wholly-resident, dense run into what this machine achieved: memory
  bandwidth while answering (tok/s × the bytes a token reads, the cache at the
  run's average fill included) and the rate it reads a prompt (tok/s ×
  parameters). Other models on the same path are then estimated from the
  measurement nearest in size, ± `CalibrationMargin` (5%, the gate's own
  tolerance) + `CalibrationMarginPerDoubling` (6%) for every doubling of size
  between them, capped at 30% and never wider than the published range was. A
  part that is not in gpus.yaml — which had no estimate at all — gets one;
  the processor's range, the widest of all, collapses to this machine's. The
  estimate stays an estimate (`Speed.Calibrated`, `CalibratedFrom`), the
  reason on the card says which test it came from, and the confidence treats
  it as a tight path. Mixture-of-experts and split runs do not calibrate. The
  constants are CHOSEN in `estimate.Config`, with what settles them.

**Consequences.** One benchmark of llama3.1:8b on a Windows PC makes every
other model's range on that card this card's, not the population's. The
measured model itself, benchmarked through a name the catalogue does not
know, still calibrates — `model_json` keeps the header facts from Ollama's
`/api/show` (`catalog.HeaderFromRuntime`).

## D-49. The API, the refusal, and the screen

**Decision.**

- `GET /api/bench/plan?model=&num_ctx=&prompts=` — what a run would do,
  without loading anything: the prompts that fit, the estimate, the duration
  (estimated, from the speed range, the load and the settle), and the
  **refusal** (step 6, item 5). Not in the build plan's list; added because
  product rule 5 needs the button to say how long it takes, and a refusal is
  better shown before the click than after it.
- `POST /api/bench` `{model, num_ctx, prompts, measure_anyway}` → 202 and the
  run. Refused with 409 (`would_spill`, `not_recommended`) when the estimate
  says the configuration spills onto the processor, only fits at a shorter
  context (the plan names it), or does not fit at all — unless
  `measure_anyway`; a run made anyway says so in its notes. A model planned
  for the processor because there is no usable graphics is not spilling
  (product rule 6), and a budget the advisor cannot read refuses nothing:
  measuring is what it needs. 422 `nothing_fits` when no prompt fits the
  context; 404 for a model not installed; 409 while a run is in progress.
- `GET /api/bench/{id}` — the run with its samples; with `Accept:
  text/event-stream` (a browser's `EventSource`) its progress as server-sent
  events (`event: progress`, each carrying the whole run, so a reader that
  falls behind loses nothing), a keep-alive comment every 15 s, and one event
  for a run already finished.
- `POST /api/bench/{id}/cancel` — stops the run, unloads the model (D-45),
  and answers with the run as it ended, `unloaded` included.
- `GET /api/bench/history[?model=][&limit=]` — newest first, each run
  compared with the previous run of its configuration.

The literal paths beside the `{id}` wildcard are registered with
`Server.apiLiteral`: the wildcard's own fallback already answers a wrong
method with 405. `advisor bench` is the developer's text client
(`-runs 2`: the gate's repeatability; `-cancel-after`: the gate's cancel,
checked against Ollama's own `/api/ps` as well as the daemon's word);
`scripts/verify.command` runs both. A minimal **Benchmarks screen** plans,
runs, follows, cancels and lists — so the gate runs on the Windows PC without
a terminal; step 8 builds the full screen (compare two runs side by side).

## D-50. The fleet's first runs: prompts cut mid-sentence, short answers not timed, a gate model that the machine paces

Supersedes parts of D-44 (how prompts are cut), D-45 (the headline) and
D-49 (the CLI's cancel), after the step 6 gate's first runs on the fleet
(2026-09-19, Ollama 0.34.2, suite 1).

**What the runs showed.**

- Repeatable where the machine paces the model: the RTX 5070 Ti agreed
  within 0.4% (llama3.2:1b at 4k and 32k, qwen3:14b at 32k), the Mac Pro's
  D700s on Vulkan within 0.1% (llama3.2:1b). The M1 Pro did not:
  llama3.2:1b ran 109.5, 101.8 and 103.7 tok/s in three runs, while each
  run's own three timings agreed within 3.4%. At 100+ tok/s a 1B model is
  paced by the processor's work per token (and Ollama's two HTTP hops per
  token to llama-server), not by memory, and a laptop with 13 GB of its
  16 GB in use moves that between one load and the next.
- Suite 1 cut prompts at paragraph ends, and its longest prompt was the
  whole essay. Models answered it with an end-of-text after 1 token, and
  stopped after 24 and 46 tokens on others. An answer of 24 tokens at
  430 tok/s is 56 ms, which a rate cannot be built on. The M1 Pro's 46-token
  answers spread 7% and 36%, against 2.5–3.4% for its 256-token ones.
- The cancel check fired after a fixed 20 s. On the M1 Pro a whole run of
  llama3.2:1b took 20 s, so the run had finished and there was nothing left
  to cancel.
- The plan shown above a finished run still carried the estimate the run
  had just replaced (the Mac Pro's screen: ≈ 100–176 tok/s above a measured
  53.2). The estimated ranges also printed decimals they do not have
  ("≈ 47.0–57.0").

**Decision.**

1. **Suite 2.** Each prompt is the text's first N `words`, cut mid-sentence;
   the loader refuses a cut after punctuation. The text gains four
   paragraphs in the second February (the seed swap, the annual meeting,
   the work party, a late snow), so the long prompt ends in the middle of
   the narrative, well before the essay's two closing paragraphs. The
   prompts are ≈ 501, 1,970 and 7,472 tokens with Llama 3's tokenizer
   (7,472 + 256 + the margin fits 8,192). Temperature 0 cannot be told to
   ignore the end-of-text token: Ollama 0.34.2 forwards no `ignore_eos` to
   llama-server (checked in `llm/llama_server.go`). So a model can still stop
   early, but it has to finish a sentence first.
2. **Short answers time the reading, not the answering.** An answer under
   `bench.Config.MinAnswerTokens` (64) is left out of the answering median.
   When every answer of a prompt is that short, the prompt keeps its reading
   speed and time to first token, and its answering speed is absent with
   the reason in words (`generation_unknown`). The run's headline is the
   shortest prompt that has an answering speed. A run where no prompt has
   one finishes, says so, and replaces no estimate.
3. **The gate measures a model the machine paces.** Without `-model`,
   `advisor bench` picks the smallest installed curated model of at least
   3 billion parameters whose plan is not refused. If there is none, it
   takes the largest smaller one and says why. On the M1 Pro that is
   llama3.1:8b. The customer's screen is unchanged: any installed model,
   with the spread shown.
4. **The cancel check follows the run.** `-cancel-during loading|measuring`
   (replacing `-cancel-after`) cancels when the progress stream reaches that
   phase. If the run ends first, the check reports that it was not tested,
   rather than passing or failing it.
5. **The plan shows the measurement.** `Plan.Measured` is the latest
   finished run of the same model file, context, runtime version and suite
   on this machine. The screen shows it in the estimate's place (product
   rule 4) and asks for the plan again when a run ends. Estimated ranges
   print whole numbers from 10 up.

**Consequences.** Runs of suite 1 and suite 2 are never compared, because
the suite version is in the key. The M1 Pro's gate has to be run again on
suite 2 with llama3.1:8b. The Windows and Mac Pro results stand as the first
evidence for repeatability, but they were measured on suite 1.

## D-51. Step 6's gate is met: the M1 Pro on suite 2, and what the margin leaves open

Closes the step 6 gate that D-50 reopened, after the M1 Pro's re-run
(2026-09-19, `scripts/verify.command`, Ollama 0.34.2, suite 2, daemon
`3ca6b70`).

**What the run showed.**

- **Repeatable, with 0.6 points to spare.** Two consecutive runs of
  llama3.2:3b at a context of 4,096 answered at 54.9 and 52.5 tok/s: 4.4%
  against the 5% limit. Both read the metal path and the f16 cache from
  Ollama's log and unloaded afterwards, and both replaced their estimate.
  The second run's plan had already narrowed onto the first run's
  measurement — 44–60 tok/s, where run 1's plan had said 53–85 — which is
  build-plan step 6's item 6 working on a real machine rather than in a
  test.
- **The gate model is llama3.2:3b, not llama3.1:8b.** D-50 §3 named
  llama3.1:8b as this laptop's pick. The rule it states — the smallest
  installed curated model of at least 3 billion parameters whose plan is
  not refused — picks llama3.2:3b, which families.yaml puts at 3.21
  billion. Both models are installed here. The code
  (`gateMinParameters = 3e9` in `cmd/advisor/bench.go`) does what D-50
  decided; D-50's example of it was wrong.
- **The margin belongs to the short prompt.** The headline is the shortest
  prompt that has an answering speed (D-50 §2), here the 501-token one. Its
  three timings inside the second run disagreed by 20.6%, and the harness
  said so in words. The 1,970-token prompt, which nothing headlines, moved
  51.1 to 51.5 tok/s across the same two runs — 0.8%.
- **Both cancel phases pass.** `-cancel-during loading` and
  `-cancel-during measuring` each ended with the daemon and Ollama's own
  `/api/ps` agreeing that nothing was left loaded. The check D-50 §4 put in
  place of the fixed 20 s is exercised in both phases, on the machine whose
  short runs defeated the old one.
- **Suite 2's answers are long enough to time.** Every timed answer ran to
  the 256-token budget, so nothing fell under `MinAnswerTokens` and no
  prompt lost its answering speed. The 7,472-token prompt was skipped at a
  context of 4,096, with the reason in words.
- **Wired memory moves between runs.** The same model reported 2.8 GB and
  then 3.2 GB of graphics memory taken while Ollama's own size stayed at
  2.4 GB. On Apple Silicon that figure is wired memory (D-47), which counts
  what else the machine has wired, so it carries a few hundred megabytes of
  other processes with it.

**Decision.** The step 6 gate is met and step 6 is closed. Nothing about
one laptop's run is evidence enough to move a constant or a rule.

**Consequences.**

- The fleet's evidence is one machine on suite 2 and two on suite 1: the
  RTX 5070 Ti's pass (+0.4%, −0.1%, +0.2%) and the Mac Pro's (−0.1%) were
  measured before the suite changed, and suites are never compared. A
  Windows run on suite 2 is a confirmation for step 7 to take when that
  machine is next up, not a gate that is owed.
- **The thin margin and the headline's noisy prompt are one open item, not
  two fixes.** Raising the 3 billion bar so the gate lands on a model the
  memory paces, or headlining the longest prompt that has an answering
  speed instead of the shortest, would each likely tighten 4.4%. Neither is
  worth a change on a single run. Every run stores its per-prompt spread,
  so the fleet's next benchmarks say whether 4.4% was this laptop that
  afternoon or the pace of a 3B model.
- The Apple Silicon memory reading carries other processes' wired pages, so
  a `measure_anyway` run at the edge of "fits" is weaker evidence for the
  92% threshold on a Mac than on a machine with its own graphics memory.
  The NVIDIA machine is where that constant should be settled.

## Open items, for the steps that own them

- **Port.** `server.DefaultPort = 27182` with fallback to an OS-chosen port.
  Step 11 decides whether the port is persisted so "open the app" always
  lands on the same URL.
- **Logging to a file** in the data folder: step 11, with the tray.
- **Notarisation and code signing**: Itay's decision in step 11; changes
  install-page copy, not code.
- **llama.cpp and LM Studio adapters** (step 4/18): D-27 expects no
  interface change; `ModelSource{Kind: huggingface_gguf}` is exercised for
  the first time by whichever comes first.
- **A UI for Install/Pull progress and the backends/models inventory**: step
  11; step 3 only adds a temporary shell status line.
- **What step 5 read from the catalogue** — closed by D-38. What stays open
  is measurement: no model with a sliding window, shared layers, experts or
  latent attention is inside a gate yet. `scripts/probe0` on the fleet with
  the catalogue's sizes of those kinds is the check, and
  `advisor recommend` prints each card's layout notes so a wrong reading of a
  real header is visible.
- **The constants that are CHOSEN** (estimate.Config says which): where
  "fits" ends (92%), the OS reserve, the processor's efficiency range and the
  no-AVX2 factor. Every benchmark run now stores Ollama's size against
  size_vram, the log's layers offloaded, and the peak of system memory
  (D-46, D-47): a `measure_anyway` run at the border of "fits" is the
  evidence for 92%. `scripts/calibrate` with `-ngl 0` on the Windows PC and
  the Mac Pro still settles the processor's population range; on a machine
  that has run a benchmark, calibration (D-48) already replaces it. The Mac
  Pro's D700s on Vulkan are older than anything in the public scoreboard —
  benchmark them first.
- **llama.cpp is not Ollama.** The population ranges come from llama-bench;
  on a machine that has been benchmarked, Ollama's own rates replace them
  (D-48). The fleet's benchmark rows against their uncalibrated estimates are
  now the measurement of the gap.
- **Where Ollama's log is** (D-46): `~/.ollama/logs/server.log` on a Mac,
  `%LOCALAPPDATA%\Ollama\server.log` on Windows (checked against Ollama's
  troubleshooting page, not yet on the Windows PC), the journal on a systemd
  Linux install (readable only by a user in the `systemd-journal` or `adm`
  group). Where it cannot be read, runs keep path and cache type unknown and
  replace no estimate — the first Windows run is the check.
- **The step 6 gate is met** (D-51) — two consecutive runs agree within 5%
  on generation speed on the NVIDIA machine and the Apple Silicon one; a
  cancelled run leaves nothing loaded. The RTX 5070 Ti passed on suite 1
  (+0.4%, −0.1%, +0.2%; a cancelled run left nothing loaded) and the Mac Pro
  too (−0.1%); the M1 Pro passed on suite 2 with llama3.2:3b (−4.4%, and a
  cancel in each phase), through `scripts/verify.command`. What stays open is
  confirmation rather than the gate: a Windows run on suite 2 when that
  machine is next up, and whether a memory-paced gate model or a longer
  headline prompt would tighten the M1 Pro's 4.4%. The per-prompt spreads
  every run stores are the evidence for both.
- **Load time.** Two Windows runs at different contexts both reported a
  load of 1,675 ms. That is Ollama's `load_duration` for the warm-up,
  stored per run; the Mac's two runs differed (1,865 and 2,800 ms). It is
  probably a coincidence, and `GET /api/bench/{id}` shows each request's
  `load_ms` if it happens again.
- **The quant an Ollama tag pulls.** The engine assumes the usual default;
  at least one small size in Ollama's library defaults to a larger quant.
  A per-size field in families.yaml is the fix when it matters — step 9b
  added it (`ollama_quant`, D-53); a curator fills it in by hand.
- **Quality beyond size.** Until step 9b brings public signals, a larger
  model of an older family can out-rank a smaller one of a newer family;
  the purposes order is the curator's only lever. Step 9b brings them
  (D-53), into the purpose term only; how much they move depends on the
  coverage the first live read reports.
- **The live gate** passed on the M1 Pro (2026-09-19, `verify.command`):
  every size resolved, no weights downloaded. `advisor catalog refresh` in
  `scripts/verify.command` stays the live check for catalogue edits.

## D-52. Step 7: onboarding polls rather than streams, sits in front of the router, and a chat app is detected, never driven

*Amended by D-74: "Use it" becomes "Start chatting". The model is loaded
first, then the chat surface is opened on a click (`ollama://` for
Ollama's app, or a web chat already running), never driven. LM Studio is
no longer offered for an Ollama model.*

**Decision.**

- **Install and pull progress are polled, not streamed.** D-49 gave
  `GET /api/bench/{id}` an SSE mode because a benchmark run can be watched
  by more than one tab and takes minutes with samples arriving at 1 Hz; an
  install or a pull is a short, one-viewer, foreground action the person
  just clicked a button to start. `internal/server/install.go` and
  `pull.go` keep an in-memory tracker (one entry per backend name for
  installs, one global slot for pulls — Ollama itself only usefully pulls
  one thing at a time) and the UI polls `GET .../install` /
  `GET /api/models/pull` once a second. Neither is written to the store:
  D-13's history tables are evidence the app is built on, and a one-time
  foreground action is not that — if the daemon restarts mid-install the
  person just clicks the button again, and `Detect()` / `Models()` tell the
  truth either way.
- **`InstallSizer` is an optional capability interface**, the same shape as
  `LoadObserver`: a `HEAD` request against the installer's own host before
  `Install` ever runs, so the button can say "about N MB" before the click
  (product rule 5) without widening `Backend` for every future runtime that
  may not have one downloadable file to ask about.
- **`internal/chatapps` is a sibling of `internal/backend`, not a member of
  it** (D-4): a runtime is what the advisor drives (`Detect`, `Install`,
  `Start`, `Pull`) and stores in `backends`/`installed_models`; a chat app
  is only ever detected, for a link at the end of onboarding, and nothing
  about it is stored. It mirrors `internal/backend/ollama`'s own shape — an
  `env` seam (`lookPath`, `statSize`, `userHomeDir`) with per-OS
  `installLocations`/`pathCommand` files — because the problem is the same
  one: is a known program on this machine, checked without starting or
  downloading anything. `$PATH` first (a CLI companion like LM Studio's
  `lms` survives a custom install location), then well-known directories,
  existence only — a macOS `.app` is a directory, so size is not a portable
  signal (D-21: absence is "not seen here," never "not installed").
- **`OnboardingGate` wraps `<App>` in `main.tsx`, not a route inside it.**
  `App.tsx` is `screens/index.ts`'s route tree and nothing else; its own
  tests, and every screen's own test, render `<App>` directly at a chosen
  path and must keep doing that unaffected by whether setup has run. The
  gate asks `GET /api/onboarding` once; if it cannot be answered at all it
  fails open to the working app rather than trapping the person behind a
  door that may never unlock — nothing runs without its own button click
  either way (product rule 5), so that fallback is safe. A `settings` row
  (`onboarding.completed`, step 1's key/value table, no history — a
  setting, not evidence) is what `POST /api/onboarding/complete` sets and
  what every daemon start after that reads back.
- **The glossary is a component, not a table cell.** Recommend's and
  Benchmarks' Advanced panels (steps 5 and 6) already explain a technical
  term next to it in a table row; onboarding's plain-language screens show
  a handful of the same terms inline in a sentence, where a table doesn't
  fit. `copy/glossary.ts` holds the one-line explainer once per term;
  `<Term>` renders it as a button + span, not `<details>`/`<summary>` —
  `Term` sits inside ordinary `<p>` sentences, and only phrasing content is
  legal there. Not every one of CLAUDE.md's seven listed terms had to
  appear in onboarding's own copy to satisfy the rule; the rule is that
  none of the seven is ever shown bare, and the Advanced panels already
  satisfy it for the ones onboarding's plain flow has no natural sentence
  for (quantization, KV cache, offload).

**Open for step 8.** The full Ollama screen (a persistent view of the same
three states, not just a first-run gate) and the full Models screen reuse
`GET /api/backends` and `GET /api/chatapps` as they stand; neither endpoint
needs to change shape for that. `OllamaStep`'s polling interval (1.5 s for
backend status, 1 s while an install or start is in flight) is an
onboarding-only constant, not `estimate.Config`/`bench.Config` material —
worth a shared UI-polling constant if step 8's screens grow the same
pattern rather than each picking their own number.

---

Step 9b (external benchmark ingestion and the public/local split,
2026-09-24) adds D-53.

## D-53. Public data: three approved sources, stored apart, shown apart, scored in the purpose term only

**Decision.** Step 9a's note (`research/EXTERNAL_SOURCES.md`) is the
argument; this is what was built from it.

- **The list.** Three sources are approved, in this order: Hugging Face
  Eval Results (read from each size's *original* repo, `hf_base_repo`, a new
  per-size field in families.yaml), Arena's own CC BY 4.0 leaderboard
  dataset (through Hugging Face's Dataset Viewer API), and Epoch AI's
  benchmark ZIP (Epoch's own runs only — files ending `_external.csv` are
  never opened, and a ZIP that stops marking them is refused whole).
  Artificial Analysis, OpenRouter, ollama.com's library and registry,
  arena.ai's pages and every mirror, the Open LLM Leaderboard, LocalScore,
  the llama.cpp scoreboards at runtime and the Aider leaderboard are
  excluded, for the reasons and clauses the note quotes. No external
  hardware throughput is ingested: speed stays the estimator's, narrowed by
  this machine's own benchmarks (D-48).
- **Data, with a code ceiling.** `data/catalog/external.yaml` is the only
  place a source is switched on; it names each source's hosts, terms URL and
  the date they were read, licence, attribution template and cadence
  (daily; Epoch weekly), the excluded publishers (a Hugging Face result whose
  stated source is one of them is dropped), and the metric → purpose map
  with each metric's plain words and scale. It cannot add a host:
  `external.PermittedHosts` in the code is the ceiling
  (`huggingface.co`, `datasets-server.huggingface.co`, `epoch.ai`), which is
  D-10's allow-list for public data, beside `hf.allowedHost`.
  `data/catalog/aliases.yaml` maps each source's exact model names to one
  catalogue size; nothing is fuzzy-matched, nothing crosses sizes, and a
  miss is a line in the coverage report.
- **Storage.** Migration 0006 gives `catalog_external` the source's own
  date, URL, provenance (`CHECK` in maker, verified, independent, crowd),
  attribution, `detail_json`, `present` and `updated_at`, and adds
  `external_state` (a source's last success and failure in words, and each
  request's validators). Only values that map to a catalogue size are
  stored; a value that leaves its source is marked absent, never deleted; a
  source that fails keeps what it stored. `catalog.External` is the stored
  row and is not served.
- **The type.** `figure.Public` is the third kind of number: a value about a
  model that someone else published. It has no `Source` and refuses to
  marshal without its `Origin` (publisher, the source's own date,
  attribution, provenance). The origin's who-field is `publisher` in JSON,
  not the note's `source`, so no key of a public value matches a local
  figure's. `figure.CheckSeparation` and
  `TestPublicAndLocalNeverShareAStruct` hold that no API struct has a
  `Public` beside a `Bytes` or `Rate`; they sit in sibling objects
  (`public`, `local`). In the UI, `PublicFigure` is the only renderer of a
  public value, and `<Figure>`'s props cannot type-check one (a
  `@ts-expect-error` test).
- **The engine.** Public signals reach the purpose-fit term and nothing
  else: for each purpose asked, the (source, metric) pairs the map assigns
  to it that score at least `ExternalMinCovered` (5) curated sizes give the
  size's average position p, and that purpose's fit is multiplied by
  `1 + ExternalWeight × (2p − 1)` (`ExternalWeight` 0.15, CHOSEN). Only
  verified, independent and crowd values are scored; a maker's own report
  is shown, labelled, and never scored. A size a pair does not score gets
  exactly 1 from it — the absence of a signal. `ExternalWeight = 0`
  reproduces step 5's outcomes byte for byte (a test), and removing public
  data changes no fit category, speed range, memory figure or confidence (a
  test). `Factors.Public` shows the multiplier for Advanced. Percent metrics
  published as fractions are read ×100 before positions are taken.
- **The API.** `GET /api/models/{id}/detail` is a size's detail view:
  sibling `public` (entries with a position in words, the origin beneath,
  Advanced detail as labelled strings, and a "last updated" sentence that
  names any source whose latest check failed) and `local` (exactly
  `/fit`'s answer). The path is `/detail` because `GET /api/models/{id}`
  would conflict with `/api/models/installed` and `/pull` in the mux. Each
  recommendation card carries one `public` line, filled by the server from
  the same view — the engine scores, it does not write the line.
- **The refresh.** `refresh.Run` reads the due sources after the sizes and
  the installed-model mapping (`refresh.Options.External`); the report goes
  into `catalog_refreshes.report_json` beside the model list's. Public data
  never fails a refresh or blocks a recommendation.
  `advisor catalog external [-force] [-report] [-capture DIR]` is the
  curator's tool and prints the coverage report (hits and misses per
  source, aliases never answered with, candidate names, metrics the map
  does not list, results pending in open pull requests, dropped and
  unreadable entries); `advisor catalog check` validates both new files;
  `advisor recommend -detail` prints the top pick's two blocks.
- **Also from the note's build list:** `ollama_quant` per size (optional,
  read by hand from the Ollama library, never fetched) now picks the file
  the engine recommends when it is stated; `released_at` comes from the
  original repo's `createdAt` and is display only; the GGUF repo's own
  `base_model` is checked against `hf_base_repo` and a mismatch is a
  warning in the report.

**Consequences.**

- **The fixtures are shaped, not captured.** The session that built this
  had no route to any of the three hosts. The Hugging Face parser follows
  huggingface_hub 1.32.0's own code (both item shapes it accepts, and the
  request it sends: a repeated `expand=`, not `expand[]` — the note's open
  spelling question), the Arena parser the Dataset Viewer's documented
  shape, the Epoch parser the note; every column and category name is data
  in external.yaml, and a shape the parser does not recognise fails in
  words rather than produce a number. `scripts/verify.command` now saves
  every real answer to `.captures/external/`; replacing the fixtures with
  those, and correcting whatever differs, is part of step 9b's gate.
- **Coverage is the gate's to tell.** The metric ids, Arena categories and
  Epoch file names marked "unconfirmed" in external.yaml, and the aliases,
  were written without a live response; the first coverage report on the
  M1 Pro confirms or corrects them, and decides whether Epoch stays on
  (the note: off, with the reason recorded, if it covers nothing).
- **Not built here:** step 10's notification bullet (the note's P-3, third
  place) is step 10's; the curator flag for an installed model whose quant
  differs from `ollama_quant` waits for a size that states one.

## D-54. The first coverage report, and no screen that needs the model list is a dead end

**Context.** Two things arrived together on 2026-09-24. The first run of
step 9b's gate on the M1 Pro (`scripts/verify.command`) read all three
public sources and stored nothing: 0 of the curated sizes on every source.
And Itay, testing on Windows, found the app in a limbo — Ollama and a model
installed, the model list never fetched, and no screen offering to fetch it
(the fetch button only appeared on an empty recommendation of one kind);
the Benchmarks picker offered only installed models, so "Test it on this
computer" led to a screen that could not test the model it came from; and
two waits (the list fetch, a test between its click and its first event)
showed nothing moving.

**Decision — public data (supersedes D-53's "Consequences" on fixtures and coverage).**

- **Why 0 of 22, per source.** Hugging Face: the task ids huggingface_hub's
  docs gave were wrong (`diamond`, `hle`, `swe_bench_%_resolved`, not
  `gpqa_diamond`, `default`, `default`), and almost every result on the
  curated repos sits in an open pull request, which D-53 does not read
  until merged. Arena: the dataset viewer answered HTTP 500 "the dataset
  index is loading" part-way through each board, and a 500 was not retried.
  Epoch: aliases.yaml had no Epoch entries at all.
- **Fixed from the real answers**: the metric ids in external.yaml (and one
  added: MMMU-Pro vision, merged into the Gemma 4 repos; and Epoch's OTIS
  mock AIME, the Epoch file with the most curated sizes after GPQA); Arena
  and Epoch aliases for every name the answers contained that is a curated
  size, with the near misses written down as deliberately not mapped
  (Epoch's "qwen3-8b" is Qwen3 8B, not Qwen3.8; "_none" runs are thinking
  off; "ministral-3b-2410" is the 2024 Ministral); a 500 is retried, a
  "loading" one after a longer wait (`Fetcher.LoadingWait`); a source read
  only in part (any per-request or per-board state that failed) is due
  again at the next refresh instead of after its cadence
  (`store.ExternalPartsFailed`). Replaying the captured answers through
  the new configuration gives public values for 9 of the 22 sizes the
  catalogue had on 2026-09-24 (Epoch for all nine, the Hub for three of
  them), before any Arena board is read whole.
- **The open-pull-request rule stands.** Most of what the Hub shows for
  these models is in open pull requests (222 results across the curated
  repos in the first report). Reading them would triple Hugging Face's coverage, but they are
  unreviewed by the repo's owner; D-53's rule is unchanged, and the count
  stays in the report. Changing it is a product decision, not a fix.
- **`hf_base_same_as`** (families.yaml, optional): other names a GGUF card
  may give for the same original weights — a renamed repo (Meta's
  `Meta-Llama-3.1-8B-Instruct`), a maker's BF16 copy of an FP8 release
  (Mistral's Ministral 3). It quiets the base-model warning only; scores
  are read from `hf_base_repo` alone.
- **Fixtures are real.** `internal/catalog/external/testdata/` is now cut
  from the captured answers; the two shaped cases left (a verified result,
  an excluded publisher) say so in its README.

**Decision — the model list is a fact every screen can ask for.**

- `GET /api/catalog/status`: whether the list has been fetched (any size
  resolved), the last refresh, whether one is running and how far
  (`refresh.Options.Progress`, a phase — the list, then the public scores —
  with parts done of total and what is being read, in words), and whether
  public scores were ever read. `POST /api/catalog/refresh` is unchanged
  (it answers when the refresh is over); screens poll the status meanwhile.
- One UI component (`components/ModelList.tsx`) shows it on Recommend,
  Benchmarks, Models and onboarding's recommendations: the fetch button
  whenever the list was never fetched — whatever else is empty or missing
  on the machine — the progress while a fetch runs (whoever started it),
  and, on Recommend, one quiet line with a way to fetch again. The detail
  view offers the same fetch when no public source has ever been read.
- `GET /api/bench/models`: what can be tested — installed models (the
  inventory read afresh, curated first) and, apart, the list's sizes that
  are not installed and would run here (fit at Ollama's default context:
  fits, fits with less context, or runs partly on the processor), smallest
  download first, with the download size. The picker groups the two
  ("Models you have installed", "Models you don't have yet"); for one not
  installed the button says "Download 6.4 GB, then run the test" and does
  both — the test starts by itself only if its plan is not refused.
  `?model=` preselects; Recommend cards and the detail view link to it.
- **Every wait moves.** `components/Working.tsx` (an indeterminate bar and
  the seconds so far) replaces the bare "Loading…" lines;
  `components/TestProgress.tsx` shows a test from the click — before its
  first event — to its end, with a bar that moves on its own until the
  first passage is timed. `api.followBench` polls the new
  `GET /api/bench/{id}/progress` when the event stream brings nothing for
  a few seconds or breaks (something between browser and daemon holding it
  back), so a test never looks stalled.

**Consequences.** The gate of step 9b is run again with the fixed
configuration (verify.command); Arena's boards are the part still to be
confirmed whole (their Llama and gpt-oss names lie below row 200, which the
first run did not reach). Backlog item e (progress while fetching the list)
is done by the second decision.

## D-55. A refresh is a person waiting: Arena in two requests a board, every source on a clock

**Context.** The second gate run (2026-09-24, 22:28) read all three sources
— Hugging Face 3 sizes, Epoch 9, Arena 10 (its Llama and gpt-oss aliases
confirmed) — but Arena alone took about fifteen minutes: 43 requests, page
by page through boards of some 400 rows, with the dataset viewer's "index is
loading" answers and waits between them; two boards still failed. On
Windows the same read looked like a hang from the app, and since Epoch was
read after Arena, no public score appeared at all while it ran.

**Decision.**

- **Arena reads two small answers a board**: one row of the whole board
  (it exists, has the columns, how many rows, how recent — the "unchanged"
  shortcut still applies), then only the rows whose model name
  aliases.yaml lists (`"category"='…' AND ("model_name"='…' OR …)`), the
  only rows the advisor can store. The requests remain a function of the
  data files. Should the viewer refuse that filter, the board is read whole,
  as before. The curator's `advisor catalog external -whole-boards` reads
  every row, which is what lists candidate names; the daemon no longer
  does.
- **Every source has a deadline** (`external.DefaultSourceDeadline`, three
  minutes; `Options.SourceDeadline`). Past it the source fails in words,
  keeps what it stored, and — failed — is due again at the next refresh (a
  source whose last read failed is no longer skipped for its cadence).
- **Arena goes last** in external.yaml, so the quick sources' scores are
  stored first.
- **The progress names the part**: "Reading public scores from Arena
  leaderboard dataset (the coding board)", a Hugging Face repo's size,
  Epoch's download. The report prints retries and the seconds waited, and
  the fetcher logs any wait of five seconds or more, so a slow source shows
  why in verify.log.

## D-56. Arena from the files it publishes, with a small Parquet reader; public scores load in the background

**Supersedes** D-53's Arena access path (the Dataset Viewer's `/filter`)
and D-55's two-requests-a-board and `-whole-boards`.

**Context.** Even at two requests a board, the third gate run (2026-09-24,
23:14) spent its three minutes on the dataset viewer's HTTP 500 "the
dataset index is loading" and gave Arena up. The viewer builds that index
on demand; for this dataset it was loading most of the evening. But Arena
publishes the same leaderboard as plain files in the dataset repo:
`text/latest-00000-of-00001.parquet` (589 KB, 10,606 rows: every category
of the text board) and `vision/latest-…` (56 KB), served by the Hub's CDN,
no index involved. They are Parquet, and the advisor had no reader.
Itay chose (asked, 2026-09-24): a small reader of our own rather than a
Parquet library, and the public scores loading in the background so a
person is never kept waiting on them.

**Decision.**

- **`internal/catalog/parquet`**: the standard library only, like the GGUF
  reader — the Thrift compact protocol for the footer and page headers,
  Snappy, the RLE/bit-packed hybrid, flat schemas, BYTE_ARRAY/DOUBLE/FLOAT/
  INT32/INT64, PLAIN and dictionary encodings, data pages v1 and v2.
  Anything else is refused in words. What Arena's file uses was read from
  its footer (Snappy; dictionary pages then RLE_DICTIONARY; every column
  optional; row groups of 1000; parquet-cpp-arrow 19.0.1). Tested against
  files pyarrow writes the same way from real Arena rows, value for value
  against pyarrow's own reading (`testdata/gen.py`).
- **Arena's client** lists a subset's files (`GET /api/datasets/{dataset}/
  tree/main/{subset}`, which gives each file's content hash), downloads the
  split's files (`/resolve/main/…`, redirected to `*.hf.co`), and reads
  every category the metric map names from one file. Unchanged hashes: no
  download. Two requests per subset, whatever the board sizes; the whole
  board is read, so candidate names are listed again. `PermittedHosts` for
  Arena becomes `huggingface.co` and `*.hf.co` (the fetcher admits
  subdomains of a `*.` entry, never the bare domain);
  `datasets-server.huggingface.co` is no longer contacted.
- **Background public scores.** `POST /api/catalog/refresh` answers when
  the model list is in; the public sources are then read in the background
  (`Server.RefreshPublic`, one at a time), and `GET /api/catalog/status`
  carries `public_running` and which source and part is being read. The
  screens show a quiet note — "Public scores are downloading in the
  background" — and ask for their data again when it ends. The CLI and
  `Server.RefreshCatalog` still do both halves in one call.

**Consequences.** The next verify run saves Arena's real file to
`.captures/external/`; a cut of it should replace the pyarrow-written
fixture. The dataset viewer's retry logic (D-54) stays in the fetcher for
any 5xx, harmless for the Hub.

## D-57. The watch scores nothing of its own: a candidate qualifies by appearing in Recommend's own answer

**Context.** Step 10 asks for exactly the rule PRD §12 states: a new model
notifies only if it fits and beats the user's current model on a purpose
they picked. `internal/recommend` already carries that rule — `Engine.
Recommend` drops a candidate that does not fit, and (through `consider`'s
versus-current term) one that changes nothing from the model the user has.
Reimplementing "fits and beats current" as a second set of comparisons in
`internal/watch` would be two places that must agree forever, and the
values the second one needs (this machine's placement, its budget, the
purpose weights) are `internal/recommend`'s own internals, not a public
seam.

**Decision.**

- **Membership is the test.** `internal/watch.Run` asks the same `Engine.
  Recommend` `internal/server/recommend.go` already builds for `GET /api/
  recommend` — same catalogue, same machine, same stored purposes — once
  per run, then for each curated size checks only whether its
  `catalog_models.id` appears in `Result.Recommendations`. Appearing there
  already means it fit and (Preferences.CurrentModel set through recommend
  itself when there is one to compare against) was a real change from what
  the user has; nothing about "fits" or "beats" is re-derived. A size that
  is not in the answer is suppressed, its log line built from `Result.
  Empty` (nothing fit at all) or a fixed sentence naming the current model
  when there is one — never a claim the engine did not make.
- **The notification's words are recommend's own, said once each.**
  `watch.notificationFor` arranges `Recommendation.Reasons[].Text`,
  `.VersusCurrent` and the two speed lines into PRD §12's shape ("Why it
  may matter to you:" and a bullet per reason) — the same "what the
  customer reads is templated" rule (CLAUDE.md, `internal/recommend/
  reasons.go`) extends here rather than getting a second copy.
  `card()` (item 6, "what it changes") already folds the versus-current
  sentence into `Reasons` as a `Kind: "change"` entry whenever there is one
  *and* sets `VersusCurrent` to that same string, so the loop over
  `Reasons` skips `Kind == "change"` and `VersusCurrent` is added once, on
  its own — reading both without the skip said the same sentence twice.
  The one line this function does add itself is `publicBullet`, PRD §12's
  "Strong coding benchmark results": gated on `Recommendation.Public.
  Scored` (never the maker's own numbers — P-4) and on that value's
  `Position` already starting "Among the strongest", the exact wording
  `external.position()` computes for a size's own "Public data" block at
  the same top-third bar — reusing that string, rather than re-deriving
  "top third" from `Rank`/`Rated`, keeps the notification and Details
  saying it from one place. `Recommendation.Public` is nil coming out of
  `Engine.Recommend` (the engine only scores it, via `Engine.Public`); the
  server fills it for the UI's cards post-hoc
  (`handleRecommend`'s `view.Line(...)` loop), and `internal/watch` needed
  the same thing without importing `internal/catalog/external` — see the
  `PublicLine` bullet below.
- **`watch.Options.PublicLine` is a closure, not a new import.**
  `internal/recommend` does not depend on `internal/catalog/external`
  (`Recommendation.Public` is a field the server fills, not something the
  engine computes), and `internal/watch` keeps the same shape: `Options.
  PublicLine func(modelID int64, purposes []catalog.Purpose)
  *catalog.PublicEntry` is `internal/server/watch.go`'s closure over the
  same `*external.View` `RunWatch` already builds for `Engine.Public`
  (`view.Scoring()`); `checkOne` calls it for the one candidate it found in
  `Result.Recommendations`, right before building the notification. A nil
  `PublicLine` (no public data yet) means no public bullet, same as a size
  with nothing public.
- **Once per model, ever.** `watch_state.notified_at` starts empty and,
  once a run sets it, a later `UpsertWatchState` can only leave it as it
  was (`COALESCE(watch_state.notified_at, excluded.notified_at)` in the
  upsert) — enforced in SQL, not only by the Go caller remembering to
  check first. `Run` still checks in Go too (skips re-scoring anything
  already `NotifiedAt`), so a size does not pay for `Engine.Recommend`'s
  work twice in one run for nothing. `NotifyMode` is a second axis from
  `Enabled`: `never` still checks and logs (every size logs "notifications
  are turned off" and nothing is scored, so turning `on` again later gives
  every size a first fair look — no size is silently marked seen while
  notifications were off), `quiet` scores and would notify but shows no
  popup, `on` shows one. Only `Enabled=false` is a full pause: no refresh,
  no check, no log line, not even a recorded run.
- **A new maintainer repo is flagged, never scored.** `hf_base_repo`'s
  owner segment (the maker's own namespace; `hf_repo`'s owner is always the
  GGUF quantizer, `bartowski` and the like — families.yaml's own
  documented distinction) is who "the same maintainers" means. A new
  `hf.Client.ListByAuthor` (`GET /api/models?author=`, the same ETag/
  throttle/retry shape as `ModelInfo`) lists an owner's newest repos;
  anything not already named by the catalogue (as a base repo, an alias,
  or a GGUF repo) is logged `flagged_for_curator` and stops there — there
  is no `recommend.Entry` for a repo families.yaml has never curated, so
  there is nothing to fit or compare it against. Flagged once, like a
  model (the same `watch_state` row, keyed `repo:owner/name` instead of
  `model:id`).
- **`internal/store` keeps its own plain row types** (`WatchStateRow`,
  `WatchRunRow`) rather than importing `internal/watch`'s: `internal/watch`
  already imports `internal/store` (D-11's one-way dependency graph), and
  `internal/watch` converts between the two at its own edge (`toWatchStates`
  in run.go) — the same shape every other domain package's store rows
  already take.
- **Purposes become a durable setting.** They were UI-only state
  (`Recommend.tsx`'s `useState`) until now; a scheduler with nobody
  watching needs to know what to check candidates against, so
  `SettingsResponse.purposes` / `SettingsUpdate.purposes` joins `advanced`
  in the settings table (`settings.purposes`, comma-separated), and
  `Recommend` seeds its checkboxes from it once and writes back on every
  change — the same table row Advanced already lives in, read the same way
  (CLAUDE.md's own settings.go comment already said the next setting would
  land exactly here).
- **Desktop notifications are per-OS `exec.Command`**, the same idiom as
  `cmd/advisor`'s `openBrowser` and `server`'s `openInFileManager`:
  `osascript -e 'display notification'` (macOS), `notify-send` (Linux),
  a short PowerShell script against `System.Windows.Forms.NotifyIcon`
  (Windows, since a real toast needs an installer-registered app identity
  that arrives in step 11). None can hand the OS a click handler back into
  this specific daemon from a bare binary; every notification also states
  its URL as text in the body, so it always works by hand until step 11.
  No new dependency, matching CLAUDE.md's dependency list.
- **The scheduler**: a background goroutine (`Server.WatchScheduler`,
  started from `cmd/advisor/main.go` the same way `RecordHardware` and
  `RecordBackends` are) waits a short jittered pause after start, runs a
  check, then waits `watch.Config.DefaultInterval` (24 h) — jittered by
  `Config.Jitter` each time — before the next. `RunWatch` (the scheduler's
  and a manual `POST /api/watch/run`'s shared entry point) is what one
  check actually does: `RefreshCatalog` first (so a size resolved today is
  already a candidate this run can see), then `watch.Run`. One run at a
  time (`ErrRefreshRunning`, the same shape a concurrent catalogue refresh
  already answers with) — a manual "check now" and the daily tick never
  race each other's `watch_state` writes.

**Consequences.** The watch's own correctness rides on `internal/recommend`
staying correct — a future change there (a new versus-current rule, a
changed purpose weight) changes what gets notified without touching
`internal/watch` at all, which is the point: one rule, one place. A
machine not yet detected (a `POST /api/watch/run` fired moments after
start) is handled the same way a size the engine cannot fit is — suppressed
with a reason, never an error — rather than the scheduler needing its own
"wait for hardware" seam.

## D-58. What speed a purpose needs: two bars per purpose, graded from a data file, settled by a fleet trial

**Context.** Backlog items (b) and (i). Two thresholds decide what the
advisor says about speed today, and neither knows about purpose:
`reasons.go`'s `pace()` (15 and 5 words a second, a number that lives in
code rather than a config) and `recommend.Config.ComfortableTPS` (10 tok/s,
the same for every purpose). Answer speed alone is also the wrong measure
for several purposes. For coding, long documents and agents, the wait
while the model reads the prompt often matters more than answer speed.
Reasoning models write hidden thinking before the answer, and to the person
that is waiting. An agent has no one reading along. The estimator already
produces both rates (`estimate.Speed.Generation` and `.Prompt`), and the
benchmark measures both. Itay scoped the item on 2026-09-25
(`claude/backlog.md`, (i)).

**Decision.**

- **The table is data:** `data/recommend/speed-needs.yaml`. It has one row per
  `catalog.Purpose`. Every value in it is `public` (with a source),
  `trial` (with the date of the fleet trial) or `chosen` (with what would
  settle it). No value has neither. The file's header holds the schema and
  the arithmetic, and `internal/recommend` decodes it strictly.
- **Two bars per purpose.** *Wait* is the seconds before the first visible
  word: `prompt_tokens ÷ prompt tok/s + thinking_tokens ÷ answer tok/s`.
  Model loading is excluded because it happens once, not per answer.
  *Stream* compares answer tok/s with how fast a person takes words in, for
  the purposes someone reads along with. Agentic is `per_step`: its one bar
  is the time for a whole step (new tool output read, plus a tool call
  written). Grades, best first, are `excellent`, `good`, `usable` and
  `too_slow`. A purpose's grade is the worse of its two.
- **Anchors, not inventions.** The stream bars are set by three reading
  rates from Brysbaert (2019): listening 160 wpm (3.56 tok/s), reading 238
  wpm (5.29), skimming 450 wpm (10.0). Andes (2024) agrees within 10%:
  3.3 and 4.8 tok/s. The wait bars come from Nielsen's limits: 1 s keeps
  a person's flow, 10 s is the limit of their attention. Purposes where the
  person expects to wait (reasoning, long documents, agents) start at 10 s.
  Typical prompt sizes come from LMSYS-Chat-1M (70 tokens) and, for
  reasoning, from the overthinking paper's measurements (about 500 to
  1,850 hidden tokens). Every other number, and the mapping from each
  anchor to a grade label, is `chosen` and on the fleet trial's list.
  `words_per_token` (0.75) moves from `reasons.go` into the file.
- **Estimated ranges grade at both ends.** Wait and stream are computed at
  the rate range's low and high ends. When the two grades differ, the words
  say so ("good to excellent"). A grade is never stated more firmly than
  the rates it came from: a grade built from an estimated rate is estimated,
  and one built from measured rates is measured (product rule 4).
  When `Speed.Prompt` is absent, the wait bar is unknown, not zero, and
  the grade says what it rests on. (j) will give the grade its own `figure`
  treatment on screen. In (i) it reaches the customer only inside the
  templated reason, and that reason already names its source.
- **The ranking uses the same bars.** Per purpose asked: `min(1, G ÷
  excellent-stream rate, excellent-wait ÷ wait)`, with rates at the
  geometric middle of the range, as before. It is floored at `SpeedFloor`
  and averaged over the purposes asked. For chat on a graphics card the
  wait term is 1, so the factor equals today's `ComfortableTPS` factor.
  `ComfortableTPS` and `pace()` are removed.
- **Reasons are templated per purpose** in `reasons.go`. There is one
  sentence per purpose asked, naming the grade, the words a second and,
  when it limits the grade, the wait in seconds for that purpose's typical
  prompt ("about half a minute to read a pasted file"). Product rule 6
  holds: a slow grade reads as a tier, not a failure.
- **The glossary shows the table.** `GET /api/speed-needs` serves the
  bars in words and numbers. Its numeric fields are configuration
  (`source:"n/a"`, "curated threshold from speed-needs.yaml"), not figures
  about this machine or a model. The `tokens_per_sec` explainer shows
  them. That closes (b).

**The fleet trial (settles every `chosen` value; required before release).**

1. *Controlled playback.* A page streams the benchmark suite's own text
   (`data/bench/text.txt`) at set speeds after a set wait. Itay rates each
   clip on the four grades for each purpose, framed by a prompt of that
   purpose. Stream grid, with a 1 s wait: 2, 3.5, 5.3, 7, 10, 15 and 25
   tok/s. Wait grid, streaming at 10 tok/s: 0.5, 1, 2, 4, 8, 15, 30, 60
   and 120 s. A bar becomes the slowest speed, or the longest wait, that
   still earns its grade.
2. *Real counts.* On the fleet, Itay sends five of his own prompts per
   purpose to curated models through Ollama. `prompt_eval_count` and
   `eval_count` (thinking included) from each response settle
   `prompt_tokens`, `thinking_tokens`, the vision image tokens and the
   agentic step sizes.

Each settled value becomes `basis: trial` with the trial date and a note
giving the number of raters (one, to start). `version` is bumped.

**Consequences.** The pinned outcomes in `recommend_test.go` can move where
a golden profile's prompt speed is slow, mostly on processor-only machines.
A moved pin is a finding to report, not a test to update. On the processor,
Ollama's prefix cache was seen not to work (ollama/ollama #14780), so the
agentic and multi-turn waits may be longer than the arithmetic says. The
trial's real counts will show it. A new model needs no row here, because
the table is by purpose, not by model.

## D-59. A wait inside the usable bar does not change the ranking (amends D-58)

**Context.** Under D-58's ranking term, each purpose scored `min(1, G ÷
excellent stream rate, excellent wait ÷ wait)`. This penalises a wait
far more than a stream at the same grade. A stream at the `good` bar scores
0.53, but a wait at coding's `good` bar (4 s) scored 0.25, and one at its
`usable` bar (10 s) scored 0.1. With speed weighted at 1.5, the
implementation moved `TestFleetTopPicks`' coding pick on the MacBook Pro M1
Pro from Qwen3.5 9B to 0.8B, over a wait of about 7 s to read a pasted file
(prompt 181–472 tok/s on Metal). On the Mac Pro (Vulkan) it moved from 4B to
2B. The implementation reported this rather than re-pinning (backlog (i)).
A 0.8B model is no coding recommendation, and a 7 s wait is inside the
bars' own `usable`.

**Decision.** Itay chose this option (2026-09-25), over giving full marks at
`good` (which makes the M1 Pro coding pick 4B) and over leaving the tests
failing until the trial.

- The stream term applies to **every** purpose, agentic included: `min(1, G
  ÷ skimming rate)`. This is exactly the `ComfortableTPS` factor D-58
  replaced, and it is what still sends small models to processor-only
  machines, for agents too.
- The wait term is `min(1, usable wait ÷ wait)`. It costs nothing until the
  wait passes the purpose's `usable` bar.
- The grades and the card's words are unchanged. The card still states the
  wait in seconds whenever the wait is what limits the grade. The ranking
  just does not trade a much more capable model for it.
- `TestFleetTopPicks` and every other pinned outcome hold as they were
  before D-58.

**Revisit after the fleet trial.** If the trial finds that waits inside
`usable` feel worse than the bars say, the fix is to move the bars in
`speed-needs.yaml`, not to steepen this term again without a test showing
what it picks.

## D-60. Packaging: GoReleaser OSS for the cross-compiled artifacts, hand-rolled scripts for the rest

**Decision.** `.goreleaser.yaml` builds and archives all four targets,
merges the two macOS binaries into one universal binary, builds the Linux
`.deb` via its bundled `nfpm`, writes checksums, and creates the GitHub
Release — everything GoReleaser OSS can build. The macOS `.app`/`.dmg`
(with notarization), the Windows Inno Setup installer, and the Linux
AppImage are each a hand-rolled script under `packaging/<os>/`, run as
separate steps in three more CI jobs that attach their output to the same
release with `gh release upload`. GoReleaser's own app-bundle/notarize/
MSI/AppImage pipes are Pro-only; buying Pro to avoid roughly 250 lines of
shell was not worth it for three scripts that, once written, need no
further maintenance most releases.

**Consequences.** Two build paths per macOS/Linux binary: the `release`
job's own GoReleaser build (for the raw archive) and the packaging jobs'
downloaded copy of the `build` job's already-smoke-tested binary (for the
`.dmg`/AppImage) — redundant, but each side stays simple to reason about
alone, and RELEASING.md names both. `make dmg`, `make installer` and `make
appimage` wrap the same three scripts locally, each locked to the OS it
packages — only `appimage` can run inside this repo's own Linux CI runner
(and did, for real, while building this: a genuine `appimagetool` build
whose output actually launched under FUSE and answered `/api/health`); the
other two were reviewed carefully but only CI on a real macOS or Windows
runner proves them (`claude/step-11-packaging.md` says exactly which
checks ran where).

## D-61. The tray icon: gogpu/systray, wrapped behind internal/tray's own interface

**Decision.** `internal/tray` wraps `github.com/gogpu/systray` (MIT, pure
Go — Win32 `Shell_NotifyIconW` via the `golang.org/x/sys` this repo already
depends on, AppKit via pure-Go FFI on macOS, D-Bus `StatusNotifierItem` via
`github.com/godbus/dbus/v5` on Linux), rather than break D-26's "nothing
that needs cgo, ever" for one of the older, cgo-based tray libraries. The
library is young (first tagged release v0.3.0), so `internal/tray` keeps a
small, stable interface (`Run`, a menu-item type) between it and the rest
of the app — swapping the backend later, if it proves flaky, is a
`internal/tray/runner.go` change, not a `cmd/advisor` one. A tray init
failure (no D-Bus session, a headless box) is logged and the daemon keeps
running headless; it must never crash the process — the same standard
product rule 6 already set for weak hardware, applied here to a weak
desktop environment instead.

**Consequences.** Menu construction and the "start at login" checkbox's
manual invert-and-confirm logic (the library does not auto-toggle
`Checked` on click — confirmed by reading `platform_linux.go`, not
assumed) are tested through a fake backend (`tray_test.go`); the real
library cannot run in CI (no display, no D-Bus session) or in the sandbox
this step was built in, so its actual on-screen behavior is unverified
until someone runs the packaged app.

## D-62. The update check: one allow-listed host, manual only, no background poller

*Amended by D-64: `api.github.com` is a line in the one allow-list, kept
apart from the model-data hosts by purpose rather than by living in its own
file.*

**Decision.** `internal/update.Check` makes a single unauthenticated GET to
`api.github.com`'s "latest release" endpoint for this repo, only when the
customer clicks Settings' "Check for updates" — never on a timer, never at
startup. `PermittedHost` is its own constant, deliberately not folded into
`internal/catalog/external.PermittedHosts`: that allow-list audits the
model/benchmark data path, and this is a different button answering a
different question. Nothing about the user or the machine rides along in
the request — the same "no usage data leaves the machine" standard product
rule 7 already holds the model-data path to.

**Consequences.** No self-update in the MVP (BUILD_PLAN.md's step 11 says
so explicitly) — a newer version is a link to INSTALL.md's download page,
not an in-place replace. GitHub's unauthenticated rate limit (60
requests/hour per IP) is plenty for a button a person clicks by hand; the
handler surfaces a rate-limit response as a plain "try again later"
rather than an error banner (seen for real while smoke-testing this step).

## D-63. A real Windows identity: AUMID + toast, resolving backlog (k)

**Decision.** `internal/winapp.RegisterIdentity` sets the process's
explicit AppUserModelID (`SetCurrentProcessExplicitAppUserModelID`, via
`syscall.NewLazyDLL` — no new dependency, no cgo) and its display name
under `HKCU\Software\Classes\AppUserModelId`, unconditionally, on every
start. `internal/watch/notify_windows.go` posts a real
`Windows.UI.Notifications` toast under that AUMID (still shelled through
PowerShell, matching the file's existing approach) instead of the legacy
tray "balloon tip" (`NotifyIcon.ShowBalloonTip`), falling back to the
balloon only when the toast call itself fails (no AUMID registered yet, an
old Windows build). This is what backlog (k) asked for: a real OS
notification with its own entry in Windows' notification settings and
history, instead of a balloon that appeared with no permission surface at
all.

**Consequences.** Both HKCU-only, no installer or admin rights required —
consistent with `internal/autostart`'s Windows path. Untestable outside a
real Windows host; `notify_windows_script.go`'s pure string-building (the
XML escaping, the PowerShell escaping, the two scripts' shape) is unit
tested, but the toast actually appearing on screen is not — the same
caveat D-61 makes about the tray.

---

Step 12 (security and privacy review, full code review, 2026-09-27) adds
D-64 to D-70. The findings, with what was checked where, are
`claude/step-12-security.md`; the customer's account is `SECURITY.md`.

## D-64. One allow-list, enforced at the transport: `internal/egress`

*Amended by D-75: a fifth purpose, `ModelSearch`, the one purpose whose
requests carry what the person typed or picked, only on their click.*

**Context.** Before step 12 there were four allow-lists in four packages
(`hf.allowedHost`, `external.PermittedHosts`, `update.PermittedHost`,
`ollama.downloadHost`) and twelve places that built their own
`http.Client` (a thirteenth used `http.DefaultClient`). The lists were
checked by the callers that remembered to: Ollama's installer checked its
first URL and then followed any redirect to any host, over any scheme; the
update check followed any redirect; and nothing stopped a new package from
building a client with no list at all. D-10 asked for "every outbound host
on an allow-list in one file; the list is the audit".

**Decision.**

- `internal/egress/hosts.go` is that file: every host the advisor may
  contact, each with the purposes it serves (`ModelList`, `PublicScores`,
  `OllamaDownload`, `UpdateCheck`), an optional path prefix (GitHub is
  narrowed to `/ollama/ollama/releases/` and to this project's releases),
  and a sentence saying what is fetched.
- `egress.Client(purpose, timeout)` is the only way to reach another
  computer. Its transport refuses, before a byte is sent, any request that
  is not HTTPS on the default port to a host the list gives that purpose,
  with no credentials in the URL. Redirects are requests too, so every hop
  is checked by the same transport, whatever redirect policy a caller sets
  on top. TLS is verified, minimum TLS 1.2. A proxy the computer is set up
  with is honoured, because the TLS still runs end to end to the listed host.
- `egress.Local(timeout)` is the only way to reach the runtime: no proxy,
  and a dial check on the resolved address that refuses anything but
  loopback. The advisor's own CLI talks to its daemon through it too.
- The narrower lists stay as narrower lists (`hf.allowedHost` reads
  egress; `external.PermittedHosts` is tested to be a subset of it).
- `internal/archtest/network_test.go` reads every non-test file under
  `cmd/` and `internal/` and fails the build on: an `http.Client` or
  `Transport` built outside egress, `http.DefaultClient`/`Get`/`Post`, a
  `net.Dial*`, a listener anywhere but `server.Listen`,
  `InsecureSkipVerify`, a replaced `Transport`, a string naming a download
  tool (curl, wget, Invoke-WebRequest, …), a credential header or token
  variable, or a cookie jar.

**Consequences.** Adding a host is a line in `hosts.go` with its reason,
and nothing else can add one. Tests of the Hugging Face client and the
public sources give their fake servers a plain transport (`c.HTTP.Transport
= http.DefaultTransport`), which only test files may do. `scripts/` (probe0,
calibrate) are developer tools, not shipped, and are outside the scan.

## D-65. Nothing typed is ever sent: a model receives only the suite's text

*Amended by D-73 (the model, not only the runtime, must be on this
computer) and D-75 (a download may also be a Hugging Face file the advisor
resolved itself). The benchmark is still the only thing that sends a model
text; D-78's `Load` has no text field.*

**Decision.** D-9 and D-44 said nothing the user typed is ever sent, and
that the code should make that impossible rather than merely true. Step 12
makes it a property of the types and of the one path to a model:

- **The prompt is sealed.** The suite moved to `internal/suite`.
  `suite.Prompt` has no exported field. Its only maker is
  `(*Suite).Request`, which panics on any `Suite` that `suite.Default()`
  (the embedded one) did not return. The suite's data is unexported and
  read through methods, so the embedded suite cannot be edited after
  loading. The loader now refuses a lead line that is more than one word
  around `{n}`.
- **`backend.GenerateRequest` carries no free text:** `Model`, a
  `suite.Prompt`, `suite.Options` (numbers only), `KeepAlive`, `Raw`,
  `NoTruncate`. The `System` field is gone (nothing set it), and so is the
  `map[string]any` of options. `archtest/model_test.go` pins this shape and
  holds D-8: only `internal/bench` calls `Generate` or builds a request.
- **The runtime is this computer.** `OLLAMA_HOST` is honoured only when
  it names this computer (loopback or `localhost`; `0.0.0.0`/`::` read as
  127.0.0.1). One that names another machine is ignored, and `Detect`
  says so in words. An Ollama the advisor starts itself is started with a
  loopback `OLLAMA_HOST`, whatever the environment said.
- **A download is a curated name.** `POST /api/models/pull` accepts only
  an Ollama tag the curated catalogue lists, so the endpoint cannot hand
  free text to the runtime's registry (or, through `hf.co/…` tags, to
  Hugging Face).
- **The page cannot send anywhere either.** The UI's
  Content-Security-Policy has `connect-src 'self'` (D-67).

**Why.** Every outbound request is already a function of the embedded data
files and the version (D-53, D-64), and no API request carries a string that
reaches another computer except the pull tag, now closed. What was left was
the model: a runtime that accepted any string was one careless caller away
from a prompt, and a remote `OLLAMA_HOST` would have sent the suite, and
the benchmark's timing, to another machine.

## D-66. The Ollama installer: a pinned release, the published checksum, and a clear refusal

*Supersedes D-29's "a published checksum where Ollama publishes one — it
does not, today".* It does. Every Ollama release on GitHub carries
`sha256sum.txt`, listing `Ollama.dmg`, `OllamaSetup.exe` and
`ollama-linux-<arch>.tar.zst` (checked 2026-09-27 against v0.34.4).
`ollama.com/download/<file>` answers 307 to
`github.com/ollama/ollama/releases/latest/download/<file>`, which answers
302 to `…/releases/download/<tag>/<file>`, which answers 302 to
`release-assets.githubusercontent.com`.

**Decision.** `internal/backend/ollama/download.go`:

1. follows ollama.com's link only as far as the first hop that names a
   release tag, and stops there. The version is pinned, so the checksum
   and the file always come from the same release, never from two
   "latest"s either side of a publish;
2. reads that release's `sha256sum.txt` and finds the file's line;
3. downloads the pinned file into a new `advisor-ollama-*` folder with mode
   0700 (not a predictable name in a shared temp folder), creating the
   file exclusively with mode 0600, and hashes it on the way;
4. deletes it and fails, in words, unless the hash matches.

It fails with a sentence and installs nothing when there is no release in
the redirect chain (`ErrNoRelease`), no checksum file or no line for the
file (`ErrNoChecksum`), or a mismatch (`ErrChecksumMismatch`). All of this
goes through `egress.Client(OllamaDownload)`. `InstallSize` pins the release
the same way and sends a HEAD request. On Linux, tar also gets
`--no-same-owner`.

**Consequences.** If Ollama stops publishing checksums, or moves its
downloads off GitHub, "Install Ollama" stops working and says why, rather
than running an unchecked file. The fix is then a code change reviewed
like this one. macOS and Windows still check Ollama's own signature when
the person opens the installer. The advisor does not add a second
signature check.

## D-67. The front door: the daemon's own origin, and nothing framed or fetched from elsewhere

**Context.** D-12's Origin check admitted any `http://127.0.0.1:*` or
`http://localhost:*` origin. Any page served from another port of this
computer — a developer's dev server, another local web app, anything a
browser can be pointed at — could `POST` to the API: start a download,
change settings, remove a model. A cross-site `<img>` or `<script>` could
also make a GET reach the API, because such requests carry no Origin. No
response carried anti-framing or content-security headers, so another
site could frame the app and trick a click on "Download 5 GB".

**Decision** (`internal/server/server.go`, `hostCheck`):

- **Host** stays as D-12 has it: `127.0.0.1` or `localhost` (421).
- **Origin**, when a browser sends one, must be exactly `"http://" + Host`:
  the daemon's own origin, the port included. `null` is refused (403).
- **Sec-Fetch-Site** on `/api/` must be `same-origin` or `none`, or absent
  (a program that is not a browser). `same-site` (another port on
  127.0.0.1) and `cross-site` are refused (403). The UI's own pages can be
  opened from anywhere.
- **Headers on every response:** a CSP (`default-src 'none'; script-src
  'self'; style-src 'self'; img-src 'self' data:; connect-src 'self';
  base-uri 'none'; form-action 'self'; frame-ancestors 'none'; …`, checked
  in Chromium against every screen with no violation), `X-Frame-Options:
  DENY`, `nosniff`, `Referrer-Policy: no-referrer`, and same-origin COOP
  and CORP. No CORS header anywhere, a test says so.
- Request bodies are capped at 1 MiB (handlers bound their own reads
  tighter), with `MaxHeaderBytes` and an idle timeout on the server.
- The Vite dev proxy sets `origin` to the daemon's address beside
  `changeOrigin`, so `make dev` passes the same check.
- Links the API passes to the page are someone else's words: the update
  check keeps GitHub's `html_url` only when it is this project's release
  page, a Hugging Face result's own link is kept only when it is https,
  and `PublicFigure` renders a link only for an https URL.

## D-68. Local data: private, deleted on request, and no first contact the person did not make

*Amended by D-72 (the advisor writes no environment variable; the Linux
`ollama/models-location` file is kept; refused while models are being
moved) and D-76 (what the advisor reads outside its data folder).*

**Decision.**

- **Private.** `store.Open` creates the data folder with mode 0700 and the
  database with 0600 (SQLite gives its `-wal` and `-shm` files the same
  mode), and tightens both on an existing install. It only tightens a
  folder it made or the default data folder, never one a developer pointed
  `-data-dir` at. The captured Ollama log is 0600 in a 0700 folder.
  Windows keeps `%LOCALAPPDATA%` to its user already.
- **"Delete everything"** (`POST /api/data/delete {"confirm": true}`, and
  Settings' two-step button that lists what goes and what stays):
  1. it is refused (409) while a test, a download, an install, a refresh or
     a watch run is in flight;
  2. it takes the refresh and watch locks and never gives them back;
  3. it turns off "start at login" without stopping the process
     (`autostart.Forget`; on Linux, no `--now`, since the daemon may be the
     unit's own process);
  4. it removes the Windows AUMID registration (`winapp.UnregisterIdentity`)
     and what the runtime adapter wrote (`backend.DataForgetter`: the
     captured log, any leftover `advisor-ollama-*` download);
  5. it closes the database and deletes exactly `store.DatabaseFiles`
     (db, `-wal`, `-shm`, `-journal`), then the folder if it is empty;
  6. it answers with what was deleted and kept, in words, and the daemon
     quits (`Server.SetShutdown`). The page clears its own localStorage
     copy.

  It deletes the files it knows by name, never "the folder and everything
  in it". Models (Ollama's) and a Linux user-space Ollama (a program) are
  kept, and the answer says so.
- **No first contact.** The daily watch (D-57) used to refresh the model
  list 30 seconds after every start, including the very first one, before
  the person had clicked anything. That downloaded several megabytes and
  contacted Hugging Face, Arena and Epoch on an install nobody had used,
  against product rule 5 and D-54's "the one action that fixes it — fetching
  the model list, with its cost on the button". A scheduled check now does
  nothing at all (`ErrNotFetchedYet`) until the list has been fetched once.
  After that, the watch keeps it current as Settings says.

**What the advisor writes, and where** (the inventory `SECURITY.md` gives
the customer): the database in the data folder; the captured Ollama log
under `ollama/logs` there; on Linux, a user-space Ollama under `ollama/`
there; "start at login" (a LaunchAgent plist, the HKCU Run value, or a
systemd user unit), only on request; the Windows AUMID display name under
HKCU; the Ollama installer in a private temp folder while it installs; the
browser's localStorage copy of the settings. It never stores a model's
answer text: runs keep timings, counts and resource samples.

## D-69. Dependencies: pinned, audited on every push, and so are the tools that build the release

**Decision.**

- **Go.** `go.mod` pins every module, and `go.sum` holds their hashes. CI's
  new `security` job runs `go mod verify` and govulncheck (pinned,
  v1.8.0) for linux, darwin and windows (clean on 2026-09-27).
  `archtest/deps_test.go` pins the set of direct dependencies to the ones
  D-22/D-26/D-61 name, and refuses a `replace`.
- **UI.** `package.json` names exact versions (the lockfile's own). The
  lockfile resolves every package from `https://registry.npmjs.org/` with
  a sha512 integrity hash. `ui/.npmrc` sets `save-exact`, `ignore-scripts`
  (no package runs install-time code) and `audit-level=moderate`. CI runs
  `npm audit`, which was clean on 2026-09-27. A test holds all of it.
- **CI.** Every action is pinned to a commit SHA, with its tag in a
  comment. GoReleaser is pinned (v2.18.2). `appimagetool` is a pinned
  release (1.9.1) checked against its SHA-256, not the moving `continuous`
  build. Each packaged file (`.dmg`, installer, AppImage) is uploaded with
  a `.sha256` beside it, because GoReleaser's `checksums.txt` covers only
  what GoReleaser built. The release waits for the `security` job.

**Open.** Inno Setup comes from Chocolatey unpinned. The version to pin
could not be read from the session that made this change. That is one line
(`choco install innosetup --version …`) for whoever next runs a Windows
release.

## D-70. Architecture rules the code now enforces

Step 12's full review looked for decisions stated in words and enforced
by nothing. Beyond D-64 to D-69, these became tests
(`internal/archtest`, and one UI test):

- **D-11's dependency direction** (`imports_test.go`). A layer number per
  package (leaves, then hardware/suite/update/hf, then catalog/backend, and
  so on up to server and cmd), plus the edges D-35 and D-57 rule out
  (catalog ↛ store/hf, recommend/watch ↛ catalog/external, backend ↛
  store). A new package fails until it is placed.
- **D-8** (`model_test.go`). Only the benchmark harness calls a runtime's
  `Generate`.
- **CLAUDE.md's copy rule, in the UI** (`ui/src/copy/glossaryRule.test.ts`).
  A string in `en.ts` that names VRAM, quantization, GGUF, KV cache,
  context window, tokens/sec or offload must be a label with an `explain`
  beside it, or a key listed as rendered inside `<Term>`, with the screen
  that does it. The one bare case (Settings' Advanced help) was rewritten.
- **Windows notifications.** PowerShell reads ‘ ’ ‚ ‛ as single quotes, and
  the copy uses ’. `psString` escaped only `'`, so a reason containing
  "advisor’s" ended the literal and ran the rest as PowerShell. All four are
  doubled now. `notify-send` gets `--` before its text.

---

Phase 2 step P2-2 (the phase's decisions, 2026-09-29) adds D-71 to D-78.
They answer backlog (n), (q) and (r), and supersede (g). No product code
changed in this step; each entry names the step that builds it and the
tests that will hold it.

## D-71. What phase 2 decides against: Ollama v0.34.2 read from its source, and the other two runtimes' current APIs

**Context.** The entries after this one turn on what Ollama, its desktop
app and Hugging Face actually do, and several of the answers are not what
Ollama's own docs say. So the facts are here once, each with the file it
was read from, and D-72 to D-78 cite them by number. Checked on 2026-09-29
against:

- **Ollama v0.34.2**, tag commit `dfabde45` — the release
  `runtime-support.yaml` pins (D-24) — read from the tag's source, desktop
  app included (`app/`);
- **LM Studio**: its documentation repository (`lmstudio-ai/docs`
  `9b8bc20`, 2026-09-08; the v1 REST API needs LM Studio 0.4.0) and its CLI
  (`lmstudio-ai/lms` `1b7181b`, 2026-09-25);
- **llama-server**: `ggml-org/llama.cpp` `526c43b` (2026-09-29),
  `tools/server/README.md`;
- **Hugging Face**: the search request as `huggingface_hub` `f151320`
  (`hf_api.py`, `list_models`) sends it, and `huggingface/hub-docs`
  `761bb84` (`docs/hub/ollama.md`);
- **Open WebUI**: `open-webui/open-webui` `8bd8b4f` (2026-09-21), for
  D-74's hand-off to a web chat.

**Findings about Ollama v0.34.2.**

1. **The desktop app owns its server's models folder.** The app starts
   `ollama serve` itself, with `OLLAMA_MODELS` set to its own setting
   `Models` when that folder exists, and to the default with a warning when
   it does not (`app/server/server.go`, `cmd`). An unsaved setting defaults
   to the app's own `OLLAMA_MODELS`, else `~/.ollama/models`
   (`app/store/store.go`, `Settings`). But the app's screens save the whole
   settings object on every change, the defaulted folder included
   (`app/ui/app/src/hooks/useSettings.ts`, `components/Settings.tsx`), and
   the app makes such a change by itself during its own onboarding
   (`routes/onboarding.tsx`) and the first time its chat picks a model
   (`hooks/useSelectedModel.ts`). From then on the app ignores
   `OLLAMA_MODELS`. The setting is **Model location** in Ollama's Settings,
   a folder picker, and changing it restarts the server
   (`app/ui/ui.go`, `settings`). Ollama's docs still say to set the
   variable (`docs/windows.mdx`, "Changing Model Location";
   `docs/faq.mdx`). As in D-24, what the code does is what counts.
2. **The app stops any other `ollama serve`.** When its own server exits
   with code 1 (the port is taken), it terminates every `ollama serve`
   process on the machine and starts its own (`reapServers`,
   `app/server/server_unix.go`, `server_windows.go`). An `ollama serve` the
   advisor started is cut off the moment the person opens Ollama's app,
   along with the model it had loaded and any download it was running, and
   the replacement uses the app's folder.
3. **The app exists on macOS and Windows only.** Everything under `app/`
   builds with `//go:build windows || darwin`. On Linux, Ollama is the CLI
   and, from the install script, a systemd service.
4. **The app can be opened, not told what to do.** It registers the
   `ollama://` scheme (`app/darwin/Ollama.app/Contents/Info.plist`;
   `app/ollama.iss`, `HKCU\Software\Classes\ollama`) and accepts only a bare
   `ollama://`, `ollama://apps` and `ollama://connect`
   (`app/cmd/app/app.go`, `parseURLScheme`). Its chat uses its own stored
   `SelectedModel`. With none stored it picks gemma3 by graphics memory if
   that is installed, else the first entry of its featured list
   (`hooks/useSelectedModel.ts`, `hooks/useModels.ts`), and the built-in
   featured list begins with four cloud models
   (`server/model_recommendations.go`). Its own UI server listens on a
   random port with a per-run token (`app/cmd/app/app.go`). That is
   private to the app, not an API.
5. **Some models run on Ollama's computers, through the local Ollama.** A
   cloud model is listed by `/api/tags` and `/api/show` with `remote_host`
   set (`api/types.go`), and a generate or chat request for it is forwarded
   there (`server/routes.go`, the `RemoteHost` branches of both handlers;
   `server/cloud_proxy.go`). A name ending in `:local` makes Ollama refuse
   a remote model with a 404 instead of forwarding
   (`internal/modelref/modelref.go`; the `modelSourceLocal` checks in both
   handlers). LM Studio's LM Link does the same with another computer's
   model behind `localhost` (LM Studio docs,
   `1_developer/0_core/lmlink.md`). A runtime on loopback (D-65) does not
   by itself mean the model runs on this computer.
6. **A load needs no text.** `/api/generate` with no prompt schedules the
   model with the request's options and `keep_alive` and answers
   `done_reason: "load"`. `/api/chat` with no messages does the same
   (`server/routes.go`, `GenerateHandler`, `ChatHandler`).
7. **A pull goes wherever the name says.** The host part of a model name
   is the registry Ollama contacts, over HTTPS (`types/model/name.go`,
   `BaseURL`). Nothing in the tree special-cases `hf.co`: Hugging Face
   serves the registry protocol itself. `hf.co/{owner}/{repo}:{tag}` takes
   a quant label, case-insensitively, or the exact file name as the tag.
   A repository may carry `template`, `system` and `params` files, and
   Ollama applies them (hub-docs, `docs/hub/ollama.md`). Each part of a
   name is `[A-Za-z0-9_][A-Za-z0-9_.-]*` and at most 80 characters, and the
   owner part may not contain a dot (`types/model/name.go`, `isValidPart`).
8. **A local file goes into Ollama as a copy.** `POST
   /api/blobs/sha256:<digest>` streams a file into Ollama's blobs folder
   and refuses a body whose hash differs. `POST /api/create` then names a
   model from the digests (`server/routes.go`, `CreateBlobHandler`;
   `api/types.go`, `CreateRequest`). `CreateRequest` also has `System`,
   `Template`, `Parameters` and `RemoteHost`, which are free text and a
   remote host, and the advisor must never set them. A model created from
   split GGUF files is marked as requiring Ollama 0.35.0
   (`server/create_split_gguf.go`).
9. **Ollama says where its models are, in two places.** `/api/show`'s
   Modelfile for every local model contains `FROM <models
   folder>/blobs/sha256-<digest>` (`server/routes.go`, `ShowHandler`;
   `server/images.go`, `Model.String`), which gives the folder and the
   weights' own SHA-256. And the server logs `server config
   env="map[… OLLAMA_MODELS:<folder> …]"` when it starts
   (`server/routes.go`, `Serve`). No API endpoint returns the folder.
10. **A server prunes the folder it starts on.** At startup it deletes
    partial downloads, and blobs that no manifest references once they are
    an hour old (`server/images.go`, `PruneLayers`,
    `layerPruneGracePeriod`).
11. **The Windows uninstaller offers to delete the default models folder**
    (`%USERPROFILE%\.ollama\models`, with its checkbox ticked by default)
    and looks nowhere else, so a folder chosen in Model location survives an
    uninstall (`app/ollama.iss`, `[Code]`, `DelTree`; `docs/windows.mdx`,
    "Uninstall"). The installer shows no folder page
    (`DisableDirPage=yes`), and its `/DIR=` moves only the program.

LM Studio's and llama-server's answers to the same questions are D-78's
table.

**Consequences.** These facts belong to v0.34.2. When the pin in
`runtime-support.yaml` moves, this list is re-checked the way that file is
(D-24), and an entry that rests on a fact that changed is superseded.
P2-4 and P2-7 check facts 1, 2, 4, 5 and 9 on a real macOS and Windows
install before they ship, because those are the facts read from source
and not yet seen on the fleet.

## D-72. The models folder: Ollama's own setting, read back from Ollama, never an environment variable

**Amends** D-16 (the models folder is `OLLAMA_MODELS` or the OS default),
D-29 (Start on macOS and Windows) and D-68 ("delete everything", and what
the advisor writes where). Answers backlog (n)'s question about the folder;
P2-3 builds the reading, P2-7 the choice and the move.

**Decision.**

- **The advisor writes no environment variable, `OLLAMA_MODELS` or any
  other, on any operating system.** Backlog (n) assumed it would set
  `OLLAMA_MODELS` as a user variable on Windows. On macOS and Windows the
  app decides. It passes its own saved Model location to the server it
  starts, and reads the variable only while that setting has never been
  saved, which lasts until the app's own onboarding or its first chat saves
  it (D-71, 1). The variable would still reach an `ollama serve` the
  advisor started, and the app stops those as soon as it runs (D-71, 2).
  So the variable would split one person's models across two
  folders depending on who started Ollama last: a model downloaded to D:
  while the advisor's server ran would be gone from the list once the app
  took over. The build plan asked which is worse, a variable left behind or
  models the person can't reach. Here the variable is what would make them
  unreachable, so the advisor sets none, and nothing is left behind. On
  macOS the variable would also need `launchctl setenv`, which a restart
  forgets.
- **The folder in use is the one Ollama reports.** `ModelsFolder` (D-78)
  reads it from the `FROM` line that `/api/show` gives for any installed
  local model, and otherwise from the `server config` line of the running
  server's log (D-71, 9). Before Ollama has ever run, the OS default is
  shown as where Ollama *will* put models, and labelled that way. Every
  free-space check (P2-3), Settings' "your models" path (D-16) and the move
  below read this answer, never the variable. It also corrects today's
  reading (`hardware.Storage.ModelsDir`, from the variable), which is wrong
  for anyone who has changed Ollama's Model location. That reading stays
  as the fallback for "before Ollama has run".
- **Who changes the folder depends on who runs Ollama**
  (`ModelsFolder.Control`, D-78):
  - **Ollama's app (macOS, Windows): the folder is the app's Model
    location.** The advisor recommends a drive and creates the folder on a
    button that says so ("Create the folder D:\Ollama models"). The app
    falls back to its default when the folder does not exist (D-71, 1). The
    advisor then shows the steps in words, with the path and a copy button:
    open Ollama, then Settings, then Model location, then Browse, and pick
    that folder. The app restarts its own server. The advisor watches for
    Ollama to report the new folder and says "Ollama now saves models to
    D:\Ollama models" only once it has. It never writes the app's settings
    database and never calls the app's private UI server (D-71, 4). This
    takes a few clicks in Ollama's window and no terminal (rule 1), and
    each change is the person's own click on a control that says what it
    does (rule 5).
  - **An Ollama the advisor starts itself, with no app:** the Linux
    user-space install (D-29), and `ollama serve` on a Mac that has only
    the CLI. Here the advisor sets the folder. The folder is written to
    `ollama/models-location` beside the program in the data folder, and
    passed as `OLLAMA_MODELS` only to the process the advisor starts
    (`serveEnv`, which already sets `OLLAMA_HOST` the same way). This is
    one button: "Keep models on /mnt/data (812 GB free)".
  - **A system service** (the Linux install script's systemd unit):
    changing it needs an administrator. The advisor says where the models
    are and what would move them, in words. It never asks for a password
    (D-29).
- **Start follows the same rule** (amends D-29). Where Ollama's app is
  installed, "Start Ollama" opens the app hidden: `open -j -a Ollama` on
  macOS, `ollama app.exe hidden` on Windows (the app's own "hidden"
  argument, `app/cmd/app/app.go`). The server is then always the app's,
  using the app's folder, and the app never cuts off a model the advisor
  loaded or a download it started (D-71, 2). The advisor starts `ollama
  serve` directly only where there is no app. The runtime-path log (D-31,
  D-46) is then Ollama's own `server.log`, already one of the documented
  locations the adapter reads.
- **When the choice is offered.** Before the first download (onboarding's
  recommendations and Settings), not at install time, because the
  installer's `/DIR` moves only the program (D-71, 11). It is offered again
  whenever a download would not fit (P2-3). Only fixed local drives are
  offered (P2-7): the app goes back to its default folder when the chosen
  one is missing (D-71, 1), so a removable drive would silently send
  downloads back to C:. When the folder Ollama reports is not the one the
  person chose, the advisor says so in words ("Ollama can't reach D:\Ollama
  models and is saving to C:\Users\…\.ollama\models instead. Is the drive
  connected?").
- **Moving models that already exist** is one flow on every OS (P2-7):
  1. It is refused while a download, a test, a model load or an import is
     running, and unless the new drive has room for every blob plus
     P2-3's margin.
  2. The old folder is the one Ollama reports. Ollama keeps running on it
     during the copy: blobs never change once written, and each is named
     by its own hash.
  3. The advisor copies every `blobs/sha256-<hex>`, never a `-partial`
     file or anything else, then checks each copy's SHA-256 against its
     name. Progress is shown in bytes. Cancel deletes what has been copied
     into the new folder so far, and nothing else.
  4. It copies `manifests/` last, so a server that starts on the new folder
     finds every blob referenced and prunes nothing (D-71, 10). If the
     manifests changed during the copy, it copies the difference and
     checks again.
  5. The switch happens as above: the person changes Ollama's setting, or
     the advisor changes it for the Ollama it runs itself.
  6. Confirm: Ollama reports the new folder, and `/api/tags` lists the same
     names with the same digests as before the move. Otherwise the advisor
     says which models are missing, says that everything is still in the
     old folder, and offers the way back (the same steps, or automatic for
     its own Ollama).
  7. Only then does it offer "Delete the old copy (frees 23 GB)", as its
     own click. That deletes exactly the files the advisor copied and
     checked, by name, and then the folders it leaves empty. It never
     deletes a folder recursively and never deletes a file it did not copy
     (the rule D-68 set for the data folder).

  Links (a symbolic link on macOS or Linux, a junction on Windows) from the
  old path to the new one are not used. Ollama's own setting would then
  show a folder whose contents live somewhere else. And Ollama's Windows
  uninstaller offers to delete `%USERPROFILE%\.ollama\models`, with the box
  ticked by default (D-71, 11). Through a junction, that would delete the
  models on the new drive.
- **"Delete everything" (D-68)** has no variable to undo. The Linux
  `ollama/models-location` file is kept with the program it configures
  (D-68 already keeps `ollama/`), so the models stay reachable, and the
  answer says so. Models in a folder the person moved them to are theirs
  and are kept. An old copy not yet deleted is listed with its path and
  size, so the person can remove it. The button is refused while a move is
  running (added to D-68's list of refusals).

**Why.** Ollama's app, not the environment, decides where the app's server
keeps models, and it stops any server it did not start. A setting the
advisor wrote anywhere but in the app would work until the app next ran.
Reading the folder back from Ollama is the same principle as D-31 (the
runtime path is what the runtime did, not what the environment suggests).

**Costs.** On macOS and Windows, choosing the drive is not one button. It
takes several clicks in Ollama's own window, which the advisor cannot see:
it knows the change happened only once Ollama reports it. If Ollama
redesigns its Settings screen, the steps (copy in `en.ts`, checked against
the pinned release) need rewriting. One fallback, the `server config` log
line, depends on Ollama's log format. The primary reading, Show's `FROM`
line, depends on its Modelfile format. Both are pinned by fixtures.

**Held by** (tests P2-3 and P2-7 write):

- `internal/backend/ollama`: `ModelsFolder` from a `/api/show` fixture's
  `FROM` line, from a `server.log` fixture's `server config` line (both in
  v0.34.2's format), and unknown when neither exists. `Start` opens the app
  when one is found (per-OS `env` fixtures) and runs `ollama serve` only
  when none is. `serveEnv` sets `OLLAMA_MODELS` only from
  `ollama/models-location`.
- `internal/archtest`: no string in the daemon's code names `setx`,
  `launchctl setenv`, `HKCU\Environment` or `SetEnvironmentVariable`, and
  `OLLAMA_MODELS` appears only in `internal/hardware` (reading) and
  `internal/backend/ollama` (reading the log, and `serveEnv`).
- The move, on `t.TempDir()` folders holding real small blobs: a corrupted
  copy is caught. A cancel leaves the old folder byte-identical and removes
  every file it copied. Manifests are written after every blob has been
  checked (the order is asserted). "Delete the old copy" removes exactly
  the copied names and does nothing without its own confirmation.
- `internal/server/deletedata.go`: `ollama/models-location` is kept, and
  the button is refused (409) while a move is running.

## D-73. A model on this computer means its weights run here: cloud and linked models get no text

**Amends** D-65 ("The runtime is this computer"). P2-4 builds it, because
it touches the adapter's `Generate` and adds `Load`.

**Decision.** The benchmark, the only path that sends a model text (D-74
keeps it that way), accepts only a model whose weights are on this
computer. So does `Load` (D-78): loading a cloud model means nothing on
this computer, and "Start chatting" never offers one.

- `backend.Installed` gains `Remote` and `RemoteKnown`, and
  `backend.ModelInfo` gains `RemoteHost`. The Ollama adapter fills them
  from `remote_host` in `/api/tags` and `/api/show` (D-71, 5).
- The Ollama adapter's `Generate` and `Load` send the model's name with
  Ollama's `:local` suffix (D-71, 5), so Ollama itself refuses a remote
  model instead of forwarding it. Before sending, they also refuse
  a model that `Show` reports as remote, with `ErrRemoteModel`, and never
  send anything for it.
- A backend that cannot tell whether a model is remote reports
  `RemoteKnown: false`, and such a model gets no test and no "Start
  chatting". LM
  Studio's LM Link is the case phase 3 has to settle (D-78).
- On the Models screen a remote model says "Runs on Ollama's computers,
  not this one", with no test and no "Start chatting" button. The recommendation
  engine and the benchmark picker treat it as not installed, because its
  fit on this machine means nothing.

**Why.** At v0.34.2 a cloud model's name reaches ollama.com through the
local Ollama (D-71, 5). A benchmark of an installed cloud model would send
the suite's text there today and time someone else's computer. D-65
checked that the runtime was on loopback. That was never the whole of
product rule 7.

**Costs.** A person who uses Ollama's cloud models cannot test them, or
start them from the advisor. That is intended: the advisor is about this
computer.

**Held by.** Adapter tests against a fake Ollama that lists a model with
`remote_host`: `Generate` and `Load` refuse it before any request reaches
`/api/generate` (the fake records every request),
and every request the adapter does send names the model with `:local`.
`POST /api/bench` answers 422 `remote_model`. An `internal/archtest` shape
test holds `Installed.Remote` and `RemoteKnown`.

## D-74. The chat stays outside the advisor: "Start chatting" loads the model and opens the chat this computer already has

**Amends** D-4 and D-52. A chat app is still detected and never driven.
What changes is that it is now opened on a click, after the model has been
loaded. D-8 and D-65 are unchanged: the benchmark is still the only thing
that sends a model any text. Answers backlog (r) and supersedes (g). P2-4
builds it.

**The three options, against D-71's facts.** The test is product rule 1 on
all three operating systems, for a person who has never heard of Open
WebUI.

- **(a) Hand-off.** The advisor starts the runtime, loads the model with no
  text (D-78, `Load`) and opens the chat the computer already has. On
  macOS and Windows that is Ollama's app, which cannot be told which model
  to use (D-71, 4), so the model can only be named for the person to pick.
  On Linux, Ollama has no app (D-71, 3), so there is nothing to open unless
  the person runs a web chat themselves.
- **(b) A chat page inside the advisor.** This works the same on all three
  operating systems and can guarantee the model, but it makes the advisor
  a chat interface and adds a second path by which text reaches a model.
- **(c) Both**, with the built-in page where no chat app exists.

**Decision: (a), hand-off. The advisor has no chat of its own.** The first
draft of this entry proposed (b). Itay chose (a) on 2026-09-29: "the chat
shouldn't be in this app, it should only open what's relevant — whether
it's Ollama itself or a web page with the chat if that's how it works on
the device, and if not available, we can add later, when we have other
programs that do support it (like llama.cpp)." D-4's "the product never
becomes a chat interface" and PRD §22 stand as written.

**"Start chatting with <name>"** (P2-4). One button per stage, each saying
what it will do:

1. "Download 5.2 GB", with P2-3's space check, when the model is not
   installed.
2. "Start Ollama" when it is stopped. Where the app is installed, this
   opens the app hidden (D-72).
3. **Load.** The model is loaded with no text (D-78) at the runtime's
   default context, which is the context a chat app's first request will
   ask for, so the load is not repeated. The screen says "Loading Qwen3.5
   9B into memory" and, when a test has measured it, how long the load took
   last time. A load that fails is reported in words here, before anything
   is opened: not enough memory, or a file the runtime cannot run.
4. **Open the chat surface for this runtime on this computer**, the first
   of these that exists. The others, if any, are offered as links below it.
   - **A chat page the runtime serves itself** (`Capabilities.ChatPage`,
     D-78), opened in the browser at its loopback address, with the model
     chosen when the page allows it. There is none in phase 2: Ollama
     serves no page, because its chat is its app. llama-server serves one:
     its built-in web UI, on by default (`--ui`, `tools/server/README.md`).
     That is how phase 3 gives Linux a chat that opens in one click.
   - **A web chat already running on this computer that talks to this
     runtime.** Today that means Open WebUI, recognised by one GET to its
     public `/api/config` at its default address (`127.0.0.1:8080`)
     through `egress.Local`. It is opened at `/?model=<name>`. Open WebUI
     selects the model named in its URL, and when it doesn't know that
     name it opens its model picker with the name filled in (open-webui
     `8bd8b4f`, `src/lib/components/chat/Chat.svelte`). The person chose to
     run it, and it can be told the model, so it comes before the next
     option.
   - **The runtime's own app**, detected by `internal/chatapps`: Ollama's
     app on macOS and Windows, opened with `ollama://` (D-71, 4). It cannot
     be told which model to use. So beside the link the screen shows the
     model's exact name with the copy button (backlog a), and the sentence
     "In Ollama's window, pick <name> from the model menu. Models whose
     names end in 'cloud' run on Ollama's computers, not yours."
   - **None.** This is Ollama on Linux with no web chat running. The screen
     says so plainly: "Ollama on Linux has no chat window of its own, so
     there's nothing to open yet." It shows the model's exact name with the
     copy button, and lists the other chat apps with a note that each has
     to be connected to Ollama in its own settings, and that LM Studio
     cannot use Ollama's models at all.
5. Home's "most useful thing" card becomes "Continue chatting with <name>".
   The last model started this way is a setting in the settings table, not
   history.

**Which chat apps are offered.** An app that cannot reach the runtime's
models as it stands is never offered as the place to chat with them. LM
Studio keeps its own copies of models, so it is never offered for an
Ollama model. The copy on the "Use it" screen changes to match.

**What the advisor does not do.** It has no chat page and sends no text
(`LoadRequest` has no text field, and archtest pins its shape). It installs
no chat app (D-4). It writes no chat app's settings and never calls a chat
app's private API (D-71, 4). It never pretends to choose a model in an app
that cannot be told which model to use. The daemon opens a chat surface
only by an id it issued: `POST /api/chat/open {"target": "<id>"}` names a
surface the server detected, and the server builds the URL itself
(`ollama://`, or the loopback page with the model's installed name). A
request can never hand the daemon a URL or a program to open.

**Why.** The product's job ends at "this model, loaded and working, in the
chat you have" (PRD §22). The gap this leaves, no one-click chat for
Ollama on Linux, is accepted and scheduled for phase 3's llama-server page.
The alternative was a chat surface the product would then have to keep
narrow indefinitely.

**Costs.**

- **Linux with Ollama: no chat to open in phase 2.** Everything up to a
  loaded, tested model works without a terminal. Talking to the model
  waits for phase 3, or for a web chat the person runs themselves.
- **macOS and Windows: the person picks the model in Ollama's window.** The
  advisor cannot guarantee which model is used, or stop the app's cloud
  default (D-71, 4). It can only say so, which the sentence above does.
- **The hand-off rests on others' interfaces**: Ollama's `ollama://` and
  Open WebUI's `/api/config` and `?model=`, pinned to the versions read
  (D-71; open-webui `8bd8b4f`). P2-4 checks each on a real install.
- **Recognising Open WebUI** costs one GET to one loopback address, reading
  one public endpoint and nothing else.

**P2-4's model is Sonnet.** No guarantee that archtest enforces is loosened:
`Load` carries no text, `Generate` is still called only by the benchmark,
and the rules that hold D-8 and D-65 are unchanged.

**Held by** (tests P2-4 writes):

- `internal/archtest`: `LoadRequest` is pinned to `Model`, `NumCtx` and
  `KeepAlive` (D-78). `TestOnlyTheBenchmarkAsksAModelAnything` and
  `TestAModelIsSentOnlyTheSuitesText` are unchanged.
- `internal/chatapps`: Open WebUI is recognised from an `httptest` server on
  loopback that answers `/api/config` like Open WebUI, and not recognised
  from one that answers differently. Nothing but that one request is made.
- `internal/server`: the choice of surface is tested for every combination
  (a runtime page, a web chat, the runtime's app, none), with fake
  capabilities and fake detection, including the words shown for each.
  `POST /api/chat/open` refuses anything but an id it issued, and a body
  carrying a URL is refused.
- `internal/backend/ollama`: `Load` sends `/api/generate` with no prompt,
  and a failed load comes back as words.

## D-75. Free text to Hugging Face: search on a click, under its own purpose; a download is a curated tag or a file the advisor resolved itself

**Amends** D-64 ("the requests are a function of the data files and the
version alone"), D-65 ("a download is a curated name") and CLAUDE.md's
Network convention. Answers the search half of backlog (q). P2-8 builds the
search and P2-9 the download.

**Decision.**

- **Allowed, narrowly.** A new egress purpose, `ModelSearch`, on
  `huggingface.co`, and on `hf.co` with its subdomains (the download
  network that header reads are redirected to). It covers three requests:
  the Hub's model search, the file listing of a repository the person
  opened, and range reads of that repository's GGUF headers. The header
  reads follow D-33 and D-34's rules: ranges from byte 0, no whole-file
  answer, the same limits, no token. It is the one purpose whose requests
  carry what the person typed or chose. Every other purpose keeps D-64's
  rule, and the daily watch never uses this one.
- **The search** is `GET
  https://huggingface.co/api/models?search=<words>&filter=gguf&gated=false&sort=downloads&limit=<N>`,
  the parameters `huggingface_hub`'s `list_models` sends (D-71). It returns
  GGUF repositories only, ungated only (the advisor never sends a token,
  D-34), most downloaded first, one page, and the advisor never follows
  the `Link` header to another page. The words are trimmed, at most 100
  characters, and sent only as the URL-encoded `search` parameter, only
  when the person presses **"Search Hugging Face"**. Never on typing,
  never on a timer.
- **A pasted link** (or `owner/repo`) is parsed on the server. The host
  must be `huggingface.co` or `hf.co`, with or without `www.`, and the path
  `/{owner}/{repo}`, optionally followed by `/blob/…`, `/resolve/…` or
  `/tree/…`. Owner and repo are checked against a strict repository-id
  grammar before either becomes part of a URL. Anything else is refused
  with a sentence.
- **The screen says it, above the box:** "Your search words go to Hugging
  Face." Nothing about a search is stored. Results and opened repositories
  live in the daemon's memory for the screen, with a size limit, never in
  SQLite, and are gone at restart. There is no search history.
- **Not behind the Advanced toggle.** Search is its own screen, **"Find a
  model"**, reached from the Models screen and the navigation. It never
  appears on Recommend or in onboarding, where the curated list stays the
  front door (D-7). The person who needs it already knows a model's name,
  not the Advanced toggle. The privacy exception is made visible by the
  button's words, not by hiding the box. The technical columns in its
  results are behind Advanced, as everywhere else (rule 2).
- **How a pull accepts a model that is not in the catalogue** (amends
  D-65). `POST /api/models/pull` accepts exactly one of two fields:
  - `ollama_tag`: a tag the curated catalogue lists, as now;
  - `resolved`: an id the server issued when, during this run of the
    daemon, it resolved a file itself. That means it listed the repository
    (pinned to a commit), chose one GGUF file from the listing, and read
    that file's header. The server keeps `{repo, commit, file, size,
    sha256}` (the SHA-256 being the listing's LFS hash) and the header, in
    memory, under a random 128-bit id, with a size limit, forgotten at
    restart.

  The request cannot name a repository, a file or a host. The server
  builds `ModelSource{Kind: huggingface_gguf, HFRepo, HFFile, HFQuant,
  HFSHA256}` (D-78) from its own record, and the Ollama adapter turns that
  into `hf.co/{owner}/{repo}:{file}`. The exact file name as the tag
  (D-71, 7) means Ollama fetches the file the estimate was made for.
  Where the file name is not a valid Ollama tag (over 80 characters, or a
  character outside the grammar), the adapter uses the quant label when
  exactly one file in the repository has it, and otherwise the model is
  refused with a sentence. After the pull, the adapter compares the
  weights' SHA-256 from Show's `FROM` line (D-71, 9) with the recorded LFS
  hash. A mismatch is said in words, and removing the model is offered.
- **Models split across several files** (`-0000N-of-0000M`) are refused in
  phase 2, with a sentence (D-71, 8).
- **Ollama, not the advisor, contacts Hugging Face for the download**, as
  it contacts its own library for a curated tag (D-65). The download is
  therefore not a line in `hosts.go`, and SECURITY.md's table gets its own
  row for it: what goes to Hugging Face is the address of the one file
  the person was shown. The repository's own `template`, `system` and
  `params` files come with it (D-71, 7). They are the model's
  configuration, applied by Ollama as for any model. The advisor's
  benchmark is raw (D-44) and ignores them.

**Why product rule 7 still holds.** Rule 7 says no prompt, no file and no
usage data leaves the machine. A search the person runs, sent where the
screen says, carries no prompt and no file, and nothing about the person or
the computer beyond what any web request carries (D-64: the User-Agent and
the connection's address). It is the person using Hugging Face through the
advisor, on their own click. **This reading of rule 7 is a product
decision, and it needs Itay's explicit agreement** (`claude/p2-2-decisions.md`
asks for it).

**Costs.** These are the first requests whose content the advisor does not
control. The audit statement in `hosts.go` changes from "a function of the
data files and the version" to "a function of the data files and the
version, and, for `ModelSearch` only, of what the person typed or picked,
on their click". Search results are repositories no curator has reviewed;
D-77 decides what they get.

**Held by** (tests P2-8 and P2-9 write):

- `internal/egress`: the `ModelSearch` purpose and its lines in
  `hosts.go`. A `ModelSearch` client cannot reach `epoch.ai`, GitHub or
  `ollama.com`.
- `internal/archtest`: `egress.ModelSearch` is referenced only in
  `internal/catalog/hf`'s search file and in the server handler that calls
  it. `internal/watch` and `internal/catalog/refresh` never reference it.
- `internal/catalog/hf`: the repository-id grammar against `..`, extra
  slashes, percent-encoding, Unicode look-alikes and over-long names. A
  thousand-item answer is cut to N. A `Link` header is not followed. A
  redirect off Hugging Face is refused (egress already refuses it).
- `internal/server`: the pull refuses a body with any field other than
  `ollama_tag` or `resolved`, refuses an unknown id, and never turns
  request text into a tag. The tag builder is tested against Ollama's name
  grammar, and the digest check after a pull is tested. The search refuses
  more than 100 characters (400). A test store that fails every write
  shows the search writes nothing.

## D-76. A model file already on this computer: a scan on a click, a pasted path under Advanced, and only the header until the person adds it

**Amends** D-16 (what the advisor reads outside its data folder) and D-68's
inventory. Answers the local-file half of backlog (q). P2-10 builds it.

**Decision: both a scan and a pasted path.**

- **"Look for model files on this computer"** (not behind Advanced) runs
  only when that button is pressed. It lists `.gguf` files in three known
  places:
  - LM Studio's models folder, found the way LM Studio's own CLI finds it:
    `downloadsFolder` in `<LM Studio home>/settings.json`, else
    `~/.lmstudio/models` (`lms` `1b7181b`, `src/subcommands/importCmd.ts`,
    `src/lmstudioPaths.ts`). The advisor reads that one key and nothing
    else of LM Studio's;
  - the Hugging Face cache (`HF_HUB_CACHE`, else `$HF_HOME/hub`, else
    `~/.cache/huggingface/hub`);
  - the Downloads folder.

  The walk has a depth limit and considers only names ending `.gguf`. It
  follows a symbolic link only when the link resolves inside the same root
  (the Hugging Face cache's snapshots point into its own blob folder).
  Nothing else is opened.
- **A pasted path** (behind Advanced, because typing a path is technical):
  an absolute path to a regular file ending `.gguf`.
- **The browser's file picker is not used.** It hands a page a file's
  contents, not its path, and uploading gigabytes through the page would be
  wrong.
- **What is read, and when.** For each file found or pasted, the advisor
  reads the GGUF header: the metadata and the tensor table, within D-33's
  limits. Reading past the tokenizer costs nothing on the network locally,
  and the tensor table gives the parameter count that the Hub supplies for
  a remote file. The whole file is read only after the person presses "Add
  to Ollama (copies 5.2 GB)": once to hash it (Ollama's blob upload needs
  the digest first) and once to stream it to Ollama (D-71, 8). The advisor
  writes nothing outside its data folder; Ollama writes its own copy into
  its own folder.
- **Ids, not paths, in the add request.** The server keeps what it found
  in memory under ids, as in D-75. `POST /api/models/import {"found":
  "<id>"}` is the only way to add a file. The advisor builds the model's
  name from the file name, cleaned to Ollama's name grammar (D-71, 7), and
  never takes a name from the request. The create request carries only the
  name and the file digests: never a system prompt, a template, parameters
  or a remote host (D-71, 8).
- **The costs are said on the button and before the click.** The copy
  doubles the disk space used until the person deletes the original, and
  P2-3's check applies to it. A file in LM Studio's folder stays LM
  Studio's, and deleting it removes it from LM Studio. The advisor never
  deletes the original.
- **Odd files.** An image reader (`mmproj*.gguf`) beside the weights is
  offered with them. A split model, a partial download, or a file that is
  not GGUF gets a sentence each, and nothing is added.

**Why both.** The scan covers the common cases, a model LM Studio or a
browser downloaded, without asking a beginner to know a path. The pasted
path covers everything else, for people who know where their file is.

**Costs.** The advisor now reads places it did not read before: the names
in three folders and the headers of `.gguf` files, only on the click and
only under the roots named here. SECURITY.md lists them (P2-10). Hashing
and copying a multi-gigabyte file takes minutes, so progress is shown in
bytes.

**Held by** (tests P2-10 writes, on `t.TempDir()` trees built from the GGUF
header fixtures): a symbolic link out of a root is not followed. A file
not ending `.gguf` is never opened (the file-system seam records every
open). A relative path or a directory is refused when pasted. The import
handler refuses any field but `found`. A fake Ollama asserts that the
create request's JSON has exactly `model` and `files`. A digest mismatch
reported by Ollama is shown in words. The scan's roots are defined in one
file.

## D-77. What a model that is not on the advisor's list gets, and what it does not

**Amends** D-7 (the catalogue is the candidate set) by saying what an
uncurated model gets outside it. P2-8 builds the estimate screen, P2-9 and
P2-10 the paths that install such a model.

**Decision.**

- **It gets a memory fit and a speed range from its file** (D-38 to D-40),
  with confidence at most medium (D-42), and the sentence "This model
  isn't on the advisor's list, so this is based only on its file." A
  benchmark of it can make that configuration's confidence high, as for
  any model.
- **An architecture the parser or the estimator does not know** gets
  "can't estimate this one" and the reason, never a number (D-21).
- **A mixture-of-experts model whose file does not say how much of it runs
  per word** gets the memory fit (every expert is resident) and no speed
  range, with a sentence saying why. Active parameters are not guessed.
- **Facts from the file are shown as facts:** whether it has an image
  reader, the context it was trained for, and the licence the repository
  states.
- **Tests and "Start chatting": yes,** once it is installed and runs
  locally (D-73).
  Its test calibrates this machine, as D-48 already allows for a model the
  catalogue does not know (the header comes from `/api/show`).
- **It does not get a purpose fit.** A family's purposes are the curator's
  ordered judgement (D-41). A name or a tag containing "coder" is not that
  judgement, and inferring one would be a guess or a model deciding (D-8).
- **It does not get public scores.** D-53 stores only values that an alias
  maps to a curated size, and nothing is matched fuzzily. A GGUF
  repository's `base_model` is only its uploader's claim.
- **It does not get a place among the recommendation cards**, on
  Recommend, in onboarding or in the watch's notifications (D-57). Those
  come only from the curated list, and the watch never tracks an
  uncurated model. It can still be "the model you have" in the
  comparison with the current model (D-41), as any unknown installed model
  can today.
- **On the Models screen** it says where it came from ("From Hugging Face,
  not on the advisor's list", or "From a file on this computer") and shows
  its speed, estimated or measured, like any other model.

**Why.** A recommendation card is the advisor saying "this is good for
what you asked", and it can only say that where a curator has looked. Fit
and speed are arithmetic, and they apply to any file.

**Costs.** A strong model that no curator has reviewed never appears as a
recommendation, however well it would suit the machine. The curator's
signal for it is the one D-7 and D-57 already give: installed models the
catalogue does not know are listed for the curator.

**Held by.** An engine test that an uncurated installed model never
appears in `Result.Recommendations`. Estimator tests that a header with an
unknown architecture gives unknown, not a number, and that confidence
never exceeds medium without a measurement. A watch test that a model with
no catalogue id is never checked or notified.

## D-78. The Backend methods phase 2 adds, checked against LM Studio and llama-server, and the capabilities that say what each runtime can do

**Amends** D-27 (the interface frozen against three runtimes) and D-3.
Like D-27, every method was checked against all three runtimes before it
was settled, so that phase 3's llama.cpp and LM Studio backends are one
more file each, not a rewrite.

**What each runtime offers** (D-71's sources; LM Studio's REST v1 needs
0.4.0, and llama-server's column is its router mode, the one that loads
and unloads models on request):

| | Ollama v0.34.2 | LM Studio | llama-server (router mode) |
|---|---|---|---|
| **Load with no text** | `POST /api/generate` with no prompt, `options.num_ctx`, `keep_alive`, answering `done_reason: "load"` (D-71, 6) | `POST /api/v1/models/load {model, context_length}`. The REST call has no idle timeout; `lms load --ttl` does | `POST /models/load {model}`. The context comes from the router's start-up arguments or a preset, not from the request |
| **Where the person chats** (D-74) | No page of its own. Its desktop app on macOS and Windows, opened with `ollama://`, cannot be told the model (D-71, 3 and 4) | Its own app, which chats with LM Studio's own models. Phase 3 checks whether it can be opened on a model | A built-in web page at its own address, on by default (`--ui`). Phase 3 checks whether the page can be opened on a model |
| **Pull from Hugging Face** | `hf.co/{owner}/{repo}:{file}` through its registry client (D-71, 7): the exact file | `POST /api/v1/models/download {model: <the repository's URL>, quantization}`, status at `/api/v1/models/download/status/{job_id}`: chosen by quant label | `POST /models {model: "owner/repo:quant"}`, progress on `/models/sse`: chosen by quant label (`--hf-file` exists only at start-up) |
| **Add a file already on disk** | `POST /api/blobs/sha256:<digest>`, then `POST /api/create {model, files}`: a copy (D-71, 8) | `lms import <path>` with `--hard-link` on the same drive, else `--copy`. Its default moves the file, and the adapter never uses the default. CLI only | A preset entry (`--models-preset`) that points at the file where it is: no copy |
| **Report the models folder** | Show's `FROM` line, else the `server config` log line (D-71, 9) | `downloadsFolder` in `settings.json`, as `lms` reads it. Not in the REST API | `GET /models` lists each model's `path`. The folders are the router's `LLAMA_CACHE` and `--models-dir` |
| **Set the models folder** | The app's Model location, set by the person with the advisor's steps; `OLLAMA_MODELS` for a server the advisor starts (D-72) | LM Studio's My Models tab, set by the person. No API or CLI | `LLAMA_CACHE` / `--models-dir` of a router the advisor starts (phase 3 supervises it, as D-29 does Ollama on Linux) |
| **Say which models are remote** | `remote_host`; the `:local` suffix refuses them (D-71, 5) | LM Link serves another computer's model behind `localhost`. Phase 3 must find how the API marks one, or report `RemoteKnown: false` | Not applicable: it serves only files it loaded |

**Decision.** `backend.Backend` gains six methods, written once for every
runtime. Where runtimes differ, `Capabilities` says so, so that no caller
branches on an Ollama assumption.

```go
// Capabilities is static for an adapter and install: what this runtime
// can do, so that a button can say before the click what it will cost.
Capabilities() Capabilities

// Load puts a model in memory without sending it any text (D-74's
// "Start chatting", D-71 6). LoadRequest has no text field; archtest pins
// its shape.
Load(ctx context.Context, req LoadRequest) error

// ChatPage is the address of a chat page the runtime serves itself, on
// loopback, opened on the model where the page allows (D-74). Valid only
// when Capabilities().ChatPage; the advisor never sends it any text.
ChatPage(ctx context.Context, model string) (url string, err error)

// Import adds a GGUF file already on this computer (D-76) and returns the
// name the runtime now knows it by, which the adapter builds from
// f.BaseName under its own naming rules.
Import(ctx context.Context, f LocalFile, progress func(PullProgress)) (name string, err error)

// ModelsFolder reports where the runtime keeps models, as the runtime
// itself says (D-72), and who can change that.
ModelsFolder(ctx context.Context) (ModelsFolder, error)

// SetModelsFolder is valid only when ModelsFolder reports Control ==
// FolderAdvisor. Otherwise it returns ErrFolderNotAdvisors, and callers
// check Control first.
SetModelsFolder(ctx context.Context, path string) error
```

```go
type Capabilities struct {
	PullFrom      []SourceKind // the ModelSource kinds Pull accepts
	PullExactFile bool         // Pull fetches the named file; false: the runtime picks by quant label
	LoadContext   bool         // Load applies LoadRequest.NumCtx
	LoadKeepAlive bool         // Load applies LoadRequest.KeepAlive
	ChatPage      bool         // the runtime serves a chat page of its own (D-74)
	Import        ImportMode   // "copy" | "link_or_copy" | "in_place"; "" = cannot import
}

type LoadRequest struct {
	Model     string
	NumCtx    int           // 0 = the runtime's default
	KeepAlive time.Duration // 0 = the runtime's default
}

type ModelsFolder struct {
	Path    string
	Known   bool          // false: Path is where the runtime will put models by default
	How     string        // how it was read, in words ("from a model Ollama has installed")
	Control FolderControl // "advisor" | "runtime_app" | "administrator" | "unknown"
}
```

The capability values per runtime:

| | Ollama | LM Studio | llama-server |
|---|---|---|---|
| `PullFrom` | `ollama_tag`, `huggingface_gguf` | `huggingface_gguf` | `huggingface_gguf` |
| `PullExactFile` | yes | no | no |
| `LoadContext` | yes | yes | no |
| `LoadKeepAlive` | yes | yes (through `lms load --ttl`) | no (its idle sleep is a start-up option) |
| `ChatPage` | no | no | yes |
| `Import` | `copy` | `link_or_copy` | `in_place` |
| `Control` | `runtime_app` with the app; `advisor` for a server it starts; `administrator` for a system service | `runtime_app` | `advisor` |

Existing types change too:

- `ModelSource` for `huggingface_gguf` gains `HFQuant` and `HFSHA256`,
  filled by the server from its own record (D-75). A runtime that fetches
  by file uses `HFFile`. A runtime that picks by quant label uses
  `HFQuant`, and the server refuses such a source when more than one file
  in the repository carries that label. Each adapter compares `HFSHA256`
  with what it downloaded, where it can read the weights' hash.
- `Installed` gains `Remote` and `RemoteKnown`, and `ModelInfo` gains
  `RemoteHost` and `WeightsSHA256` (D-73; for Ollama, the `FROM` line,
  D-71, 9).

**Why methods and capabilities, not errors.** Every runtime can do each of
these in some form. What differs is what it costs and who acts: a copy
against a link, the exact file against a quant label, the advisor against
the person in another app's settings. The screen has to say that before
the click ("copies 5.2 GB" or "uses the file where it is"; "Ollama fetches
this exact file" or "LM Studio picks the Q4_K_M file"), so it has to be
known without trying. The optional interfaces (`LoadObserver`,
`InstallSizer`, `DataForgetter`) stay optional, because a runtime can
genuinely have nothing for them: no log, no installer, no files of its
own.

**Where each is built.** `ModelsFolder`, reading only, in P2-3, which needs
it for the free-space check. `Capabilities`, `Load`, `ChatPage` (which
Ollama answers with none) and the `Remote` fields in P2-4. `SetModelsFolder` in P2-7. The Hugging Face
fields of `ModelSource` in P2-9. `Import` in P2-10. Until each is built,
the Ollama adapter does not implement it and the interface does not have
it yet; each step adds its method to the interface and to the adapter
together.

**Costs.** The interface grows from eleven methods to seventeen, and every
future runtime implements all of them. LM Studio's import and idle timeout
go through its CLI (`lms`), so its adapter runs a program, as the Ollama
adapter runs `tar` on Linux. The LM Studio and llama-server columns are
read from documentation, not from a running copy; phase 3 checks them the
way step 3 checked Ollama's.

**Held by.**

- `internal/backend`: a fake backend in the tests implements the whole
  interface (a compile-time check), and the Ollama adapter's
  `Capabilities` values are asserted.
- `internal/archtest`: `LoadRequest` is pinned to `Model`, `NumCtx` and
  `KeepAlive`, and has no other string field. No package above `internal/backend` (server, bench, recommend,
  watch, estimate) contains a string literal equal to a runtime's
  registered name, so no caller can branch on "ollama".
- `internal/server`: tests with a fake backend for every `ImportMode` and
  every `FolderControl`, showing that the words on the button and the
  screen follow the capability, not the runtime's name.
