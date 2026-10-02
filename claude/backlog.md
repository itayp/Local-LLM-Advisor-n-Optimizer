# Backlog

Small, concrete asks that surfaced from actually using the app, not tied to
one build-plan step. Not a replacement for BUILD_PLAN.md/PRD's own phase
lists — this is where a specific, dated ask lives until a step picks it up.

## Status (2026-09-29)

Where every item stands and which plan picks it up. The item's own section has
the detail.

| Item | Status | Goes to |
|---|---|---|
| a. Copy button on every model name | Done (P2-1); Compare's column heads and a Benchmarks result card's title still show a bare name | — |
| b. What a speed is good for | Done, closed by (i) | — |
| c. Multiple GPUs: setup help, and checking Ollama uses the expected one | Open, needs a multi-GPU machine | Phase 3, P3-3 |
| d. llama.cpp pools GPUs on the Mac Pro | Open, waiting for Itay's notes | Phase 3, P3-2 / P3-3 |
| e. Progress while fetching the model list | Done (D-54) | — |
| f. Top picks with no public scores | Open, parked until the catalogue grows | Phase 2, P2-6 (the engine bias doesn't depend on the catalogue) |
| g. How to run a model after picking it | Superseded by (r) | Phase 2, P2-4 |
| h. llama.cpp / LM Studio backends | Open | Phase 3 (P2-2 prepares the interface; P2-10 reads LM Studio's models) |
| i. Speed a purpose needs | Built (D-58, D-59); the fleet trial is still to do | Phase 2, P2-11 (the trial) |
| j. Speed verdict next to every speed | Part 1 done; Models screen, per-prompt rates, Compare, CLI left | Phase 2, P2-5 |
| k. Windows notifications permission | Resolved in code (D-63); a real toast not yet seen on Windows | Step 11's gate (a real installer run), not a phase |
| l. Sizes on the Benchmarks picker | Open | Phase 2, P2-5 |
| m. The 9–18 GB gap in the catalogue | Open | Phase 2, P2-6 |
| n. Pick the drive, check space before downloads | Decided (D-72): the folder is Ollama's own setting, read back from Ollama; the advisor sets no environment variable | Phase 2, P2-3 and P2-7 |
| o. "Looking at images" is unclear | Done (P2-1); the daemon's own "vision" wording is left, see the section | Phase 2, P2-6 (Go copy) |
| p. Explainer breaks its sentence | Done (P2-1) | — |
| q. Any model from Hugging Face, or a local file | Decided (D-75 to D-77): search on a click under its own egress purpose, not behind Advanced; a folder scan on a click and a pasted path under Advanced | Phase 2, P2-8 to P2-10 |
| r. From a chosen model to an open chat | Decided (D-74): hand-off only; load the model, then open the chat the computer already has. No chat in the advisor; Linux waits for phase 3 | Phase 2, P2-4 |

## a. A copy button anywhere a model name is shown

**From:** Itay, testing step 7 (2026-09-20), on the Benchmarks screen
especially.

Onboarding's "Use it" screen (`ui/src/onboarding/UseIt.tsx`) already has
one: `navigator.clipboard.writeText(modelName)`, a small state flip to
"Copied" for two seconds, `en.onboarding.useit.copy`/`.copied` for the
strings. Worth pulling into a small shared `<CopyButton value={...} />` in
`ui/src/components/` so it isn't retyped, then wiring it onto every place a
model's *exact* name is shown and might be pasted somewhere — the
Benchmarks model picker and its history rows, the Models screen (both step
8), and Recommend's cards (`<code>{r.pull_name}</code>`, step 5, currently
just text). Step 8's own scope; no API change needed.

**Done (2026-09-29, P2-1).** `ui/src/components/CopyButton.tsx` (strings in
`en.copyButton`; "Copied" for two seconds; the old selection-based copy when
the clipboard is refused, then a quiet muted note, never an alert). On
Recommend's cards, the model's own page, the Benchmarks picker (beside the
select, for the chosen model) and history rows, the Models table, and
onboarding's "Use it". Not covered: the Compare table's column heads and the
title of a Benchmarks result card, which also print the bare name.

## b. An explainer next to tok/s for what a speed is actually good for — done, closed by (i)

**From:** Itay, same session.

`copy/glossary.ts`'s `tokens_per_sec` entry (step 7) explains what a
token and tok/s *are*; it says nothing about what a given number is good
*for*. `internal/recommend/reasons.go` already does a coarse version of
this in one templated sentence ("Estimated to answer at roughly 59 to 85
words a second — much faster than you can read") — the ask is to make that
comparison a persistent, reusable thing attached to every generation-speed
`<Figure>`, not just one reason on one card.

Needs product judgement before it's built, not just UI work: real
thresholds for "comfortable for back-and-forth chat" vs. "fine for a
document you'll read once in the background" vs. "too slow to be useful."
Reading speed (~200–250 wpm, so roughly 3–4 tok/s as "keeps up with
reading") is a reasonable anchor for the bottom end; the top end (when
more speed stops mattering) is less obvious and probably wants real user
reaction, not a guess. Flag for whoever picks this up: bring real numbers,
don't invent thresholds in the UI layer.

## c. Help setting up multiple GPUs (e.g. iGPU + dedicated), and checking Ollama sees the same

**From:** Itay, same session.

Already known and already surfaced, just not acted on:
`internal/hardware/derive.go:202` detects more than one discrete GPU and
adds a plain-language note — *"This computer has N graphics cards. The
advisor plans for the first one, the X; using several cards together is
not optimised in this version"* — but the profile only ever plans for
`p.GPUs[0]` (`derive.go:226-289`). The ask has two parts:

1. **Setup guidance** when multiple GPUs are found (especially the common
   laptop/desktop case of an integrated GPU alongside a dedicated one):
   which one Ollama will actually use, and how to steer that (Ollama env
   vars, disabling iGPU offload) rather than leaving the person with only
   the "not optimised yet" note.
2. **Checking the advisor's plan against what Ollama actually did.** The
   per-GPU `RuntimePaths` a benchmark run establishes (D-39, step 3 — read
   from the runtime's own log after a load) is the existing "what actually
   happened" signal; the gap is comparing it against which GPU the advisor
   *expected* and saying so explicitly when they disagree, rather than
   only recording both separately.

Both need real multi-GPU machines to build against responsibly — check
whether one exists in the fleet before scoping this into a step.

## d. llama.cpp can pool multiple GPUs "as one" on the Mac Pro

**From:** Itay, same session (2026-09-20). *Itay: you said you'd share the
documentation for this — once you do, attach it here or point me at it and
I'll fold in the details.*

Directly relevant to (c)'s limitation: Ollama (the only backend built so
far — step 3) plans for one GPU at a time, but Itay can already get both
cards on the Mac Pro working as a single pool through llama.cpp directly.
PRD §15 already lists llama.cpp as a future alternative backend, and §18
(Phase 2) lists it as a bucket; this is the specific capability that makes
it worth doing sooner rather than later — it's not just "another runtime,"
it's the fix for (c)'s stated gap on multi-GPU machines. Worth becoming
its own line in ARCHITECTURE.md once the llama.cpp backend is scoped,
citing whatever Itay shares.

## e. Progress visualization when downloading the models list — done (2026-09-24, ARCHITECTURE.md D-54)

**From:** Itay, step 8 closure (2026-09-20).

The model list fetch (`POST /api/catalog/refresh`) takes a minute or two
and showed only a button, then "Fetching…", then a result — nothing that
moved while it ran.

**Done** in the session after step 9b's first gate run: `GET
/api/catalog/status` carries the running fetch's phase (the list, then the
public scores), parts done of total and what is being read, and
`ui/src/components/ModelList.tsx` shows them — a bar and the seconds so far
— on every screen that needs the list. Itay's Windows findings of the same
day (the list never fetched and no way to fetch it; Benchmarks offering
only installed models; waits with nothing moving) are in
`claude/step-9b-external.md`, "Also fixed this session".

## f. Top recommendations with no public scores

**From:** Itay, reading the third step 9b gate run (2026-09-24, the M1 Pro).
Parked on purpose: the catalogue has only a few sizes per size class today,
so fixing this before the catalogue grows could mean fixing it twice. When
the catalogue next grows, check whether it still happens.

**The symptom.** Most top picks on a 16 GB Mac have no public data: Ministral 3 8B
(chat #1), Gemma 4 E4B, Qwen3.5 9B for coding, Qwen3.5 4B, Ministral 3 3B.
The only scored card, Llama 3.1 8B, is labelled "among the weaker" and moved
down (×0.917).

**What it is not.** It is not about timing: that run had already read all
three sources and Ministral 3 8B still had nothing. "Don't recommend until
the public data is in" would not change the picks.

**Cause 1: the sources barely cover small, new models.** From the
captures in `.captures/external/` (Arena text board 2026-09-13; Epoch ZIP
2026-09-24):

- Arena rates no size of Ministral 3, Gemma 4 below 26B, or Qwen3.5
  below 27B. The current-generation open models it does rate below ~15B:
  `granite-4.2-3b` (1297), `granite-4.2-8b` (1317), `gemma-3-4b-it` (1291),
  `gemma-3-12b-it` (1334), `gemma-3n-e4b-it` (1305), `molmo-2-8b` (1294). For
  comparison, `llama-3.1-8b-instruct` is 1186 and `llama-3.2-3b-instruct`
  is 1109.
- Epoch (mapped metrics: GPQA Diamond, MATH L5, OTIS AIME, SWE-bench
  Verified) has `qwen3.5-9B` (already aliased), `qwen3-4b-instruct-2507`,
  `qwen3-8b`, `qwen3-14b`, `phi-4`, `gemma-3-{1b,4b,12b}-it`,
  `granite-4.0-{350m,1b,micro}`, `mistral-small-3.2-2506`. It has only
  thinking-off runs of Qwen3.5 2B and 4B (`_none`, not mapped, see
  aliases.yaml) and only the 2410 Ministral, not Ministral 3.
- Scores sit in open pull requests on Hugging Face (skipped under D-53) for
  Gemma 4 E4B (MMMU-Pro), Gemma 4 12B (MMLU-Pro, HLE, MMMU-Pro, AIME 2026) and
  Qwen3.5 4B (MMMU-Pro). They come from the model cards, so even if read they
  would be shown but never used for ranking. This is gate item 3 in
  step-9b-external.md.
- Ministral 3 has no eval results on Hugging Face at all.

**Cause 2: a bias in the engine (`recommend.publicAdjustment`).** A size's position is
computed among *all* curated sizes a signal scored, 70B included, so
any scored small model lands near the bottom and loses up to 15%. An
unscored size gets exactly 1.0 (as if in the middle). So having a score
counts against a small model, and unscored sizes rise to the top. This happens
whatever the catalogue holds.

**Options discussed (none chosen):**

1. Compare positions within a size class (by effective parameters, or
   among the sizes that are candidates on this machine), so a score
   compares like with like. A catch with the "candidates on this machine"
   version: on the M1 Pro the only scored chat candidates are Llama
   3.2/3.1, so Llama 3.1 8B would come *first* for beating even older
   models. Size bands look safer; it needs a test on the golden profiles
   either way.
2. Add scored small families alongside the current ones (not instead):
   Granite 4.2 3B/8B, Qwen3 4B-Instruct-2507, Phi-4 14B, Mistral Small 3.2
   24B. Gemma 3 4B/12B are covered too, but Gemma 4 has replaced them. Each needs
   its GGUF repo, Ollama tag and aliases checked by `advisor catalog
   refresh` / `external -report` before it goes in.
3. Better wording for "public data: none for this size and purpose": say
   why (public leaderboards rarely rate models this small) and point to the
   two-minute test as the evidence that counts here.
4. Only recommend scored sizes. Rejected for now: on small and medium
   machines it swaps in older, weaker models (Llama 3.1 8B over Ministral 3 /
   Gemma 4 E4B), which goes against product rule 6.
5. Read pull-request eval results labelled "unreviewed" (gate item 3). This
   adds shown values for Gemma 4 E4B/12B and Qwen3.5 4B, but nothing scored.

## g. How to run: UI guidance after model selection, or button to start Ollama — superseded by (r), phase 2 step P2-4

**From:** Itay, in-app onboarding feedback (2026-09-25).

When the user selects a model on the Recommend or Models screen, they need
to know the next step: is Ollama running? If not, how do they start it?
Once it runs, what command or button launches the model?

Product rule 1: the user never needs a terminal. Today, there's no in-app
path — they get a recommendation and are left to figure out Ollama on their
own. Options:

1. A button on Recommend/Models that says what it does: "Start Ollama and
   load [Model Name]" — pre-fills the URL in Ollama's CLI or (better) calls
   the Ollama HTTP API to load the model directly if Ollama is already
   running. A check on page load (`GET /api/backend/status`) sees whether
   Ollama is listening.
2. A "How to run" explainer on the Benchmarks and Models screens showing
   the exact incantation the user needs, in plain language (e.g. "Run
   `ollama pull mistral` and then `ollama run mistral`") tailored to the
   selected model name — pull from the model's card (step 4's catalogue) or
   generate it dynamically.
3. Both: a button as primary (faster, one click), the explainer as fallback
   or "next step" guidance.

Needs product scope before building: how much automation is safe (Ollama CLI
call vs. HTTP request vs. just instructions), and what happens when Ollama
is not installed or the load fails. Flag for step assignment: this connects
recommendation to actually running a model — early UX gain and a blocker on
the "I got a recommendation, now what?" question.

## h. Support for other inference backends (llama.cpp, LM Studio, etc.)

**From:** Itay, in-app onboarding feedback (2026-09-25).

PRD §15 already lists llama.cpp and other backends as future phases, but
the question is coming up from testing: "I don't want to use Ollama, I have
llama.cpp / LM Studio / [other tool] set up already — can the advisor work
with that?"

Today only Ollama is supported (`internal/backend/ollama`); the backend
registry lives in `internal/backend/registry.go` (step 3) and a new backend
takes `Load()`, `Unload()` and `PS()` methods.

Related backlog items that affect scope:

- **(c)**: Multiple GPUs — llama.cpp can pool them on the Mac Pro, making
  it a priority backend for that use case sooner than a general "other
  backends" phase.
- **(d)**: Documents this capability for Mac Pro multi-GPU.

Outline for step assignment (not a full scope, just what it unlocks):

1. **llama.cpp backend**: matches the priority of (c)/(d). It reads a Unix
   socket or HTTP endpoint, runs on macOS, Linux, Windows. Exists at
   `~/.local/share/llama.cpp/llama-server` or a configured path; check for
   it on startup, or let the user configure the endpoint. Port it uses is
   typically 8000 (user-configurable). The load and PS calls parse llama.cpp's
   HTTP API (different from Ollama's). Real machine for testing: Itay's Mac
   Pro with both GPUs via llama.cpp.
2. **LM Studio backend**: reads the HTTP API at `localhost:1234`
   (default), similar to Ollama. Simpler to add second, since both expose
   HTTP. Tested against LM Studio on Windows/Mac, not Itay's current fleet.
3. **Detection**: on startup, Ollama responds at `localhost:11434` by default
   (also configurable); llama.cpp at 8000, LM Studio at 1234. If more than
   one is listening, the UI needs to let the user pick which one (step 8, the
   Settings screen already has a backend picker drafted in backlog but not
   wired up). If none is listening, a clear message saying so, with links to
   install guides.

Worth a dedicated build-plan step once product prioritizes multi-GPU support
or the number of "can I use [tool] instead?" questions hits a threshold.
For now, scoped as a backlog idea and a feeder to (c) and (d)'s scope.

## i. A matrix of the speed a purpose actually needs, by quality bar

**From:** Itay, in-app testing feedback (2026-09-25). Needs research before it's built.

Backlog item (b) already flagged that `tokens_per_sec`'s glossary entry says
what a speed *is* but not what it is *good for*, and that
`internal/recommend/reasons.go` only does one coarse, hard-coded comparison
("much faster than you can read") on one sentence of one card. This item is
the concrete shape (b) was missing: a real table/ladder of purpose × quality
bar, e.g.

|            | Coding | Visual (vision) | Text (chat) | Translation |
|---|---|---|---|---|
| Excellent  | ? tok/s | ? tok/s | ? tok/s | ? tok/s |
| Good       | ? tok/s | ? tok/s | ? tok/s | ? tok/s |
| Usable     | ? tok/s | ? tok/s | ? tok/s | ? tok/s |
| Too slow   | below ? | below ? | below ? | below ? |

Itay: coding and translation plausibly tolerate a slower rate than chat
before it stops being "excellent" (you read the output more slowly, or
paste it rather than watch it stream); a vision/visual purpose may be
capped less by tok/s and more by whether the model is any good at the task
at all. None of that is more than a guess yet.

**Explicitly needs research, not a UI-layer invention** (same flag (b)
raised): real thresholds per purpose, not numbers picked to make the table
look plausible. Candidate anchors: reading speed (~200–250 wpm, ~3–4 tok/s)
as a "keeps up with reading" floor for chat, already noted in (b); beyond
that, real user reaction per purpose is probably needed rather than a
desk-derived guess, especially for where "excellent" tips into "good enough
but not exciting."

Once it exists, it both answers (b) properly and is the data table (g) asks
for.

**Scoped (2026-09-25, Itay):**

- **Columns:** the app's own purposes (`catalog.Purpose`), one row each.
  No translation purpose.
- **A cell has two bars:** answer (generation) speed, and seconds before
  the first word for a typical prompt of that purpose (prompt processing,
  already modelled by `estimate.Config.PromptRatio`). Reasoning also gets a
  thinking-token factor. Agentic is not tied to reading speed.
- **Evidence:** research first, with cited anchors and the rest marked
  CHOSEN along with what would settle it. Then a fleet trial: Itay rates real
  sessions at a few speeds, and that settles the CHOSEN cells before it
  ships.
- **This item covers:** an ARCHITECTURE.md decision (D-58), the data file
  plus its strict Go type and tests, replacing `reasons.go`'s `pace()` (15
  and 5 words a second) and `recommend.Config.ComfortableTPS` with the
  per-purpose bars, a templated reason per purpose, and a glossary entry
  that closes (b). The per-figure score and the UI are (j).
- **Model split:** the research and D-58 need judgement (Opus). The
  implementation after D-58 follows the `gpus.yaml` and
  `runtime-support.yaml` patterns and suits Sonnet.

**Research done (2026-09-25):** the decision is ARCHITECTURE.md D-58, and the
table is `data/recommend/speed-needs.yaml`. Every value in it is either cited
or marked `chosen` along with what would settle it. Next steps: the
implementation (Sonnet, from D-58), and separately the fleet trial in D-58,
which settles the `chosen` values before this ships.

**Done (2026-09-25, Sonnet, from D-58):** `internal/recommend/speedneeds.go`
loads and strictly validates `speed-needs.yaml` (unchanged — every number in
it is still `chosen` or cited exactly as the research left it; nothing was
tuned to make a test pass); `grade.go` grades a purpose per D-58's arithmetic
(wait vs. the reading anchors' stream bars, worse of the two, graded at both
ends of the estimated range); `speedFactor` now uses the per-purpose formula
in `recommend.Config`'s doc comment in place of `ComfortableTPS`/`pace()`/the
hard-coded `wordsPerToken`; `reasons.go` names the grade, the words a second
and — only when it is what limits the grade — the wait, per purpose asked;
`GET /api/speed-needs` (figure-tagged `source:"n/a"`, since every value is
curated configuration, not a measurement or a publisher's figure) feeds the
`tokens_per_sec` glossary explainer's new "what a speed is good for" table
(`ui/src/components/Term.tsx`), which closes (b). The chosen values still
await the fleet trial D-58 calls for; nothing here settles them early.

**Pinned-outcome finding, reported rather than fixed (2026-09-25):**
`TestFleetTopPicks`'s coding pick moved on the two non-CUDA fleet machines,
exactly the kind of move the task that implemented this flagged in advance
and asked to be reported, not silently re-pinned or tuned away:

- **MacBook Pro M1 Pro 16 GB (Metal), coding:** was `qwen3.5:9b`, now
  `qwen3.5:0.8b`. 9B's own generation speed is fine (19–28 tok/s; the old,
  purpose-blind `ComfortableTPS` factor gave it a full 1.0), but its prompt
  speed on Metal (181–472 tok/s) is what D-58 newly grades: coding's typical
  2,000-token prompt then takes 2000⁄181 ≈ 11 s at the cautious end of the
  range — past coding's own `usable` bar (10 s, `wait_s.usable`) — against
  0.98 s for 0.8B. Old score 0.513 vs. 0.131 (9B ahead); new score 0.029 vs.
  0.131 (0.8B ahead). Old speed factor 1.000 → new 0.146 for 9B; 0.8B's stays
  1.000 either way.
- **Mac Pro 2× D700 6 GB (Vulkan), coding:** was `qwen3.5:4b`, now
  `qwen3.5:2b`. 4B's prompt speed (384–3,892 tok/s) puts its wait at the
  cautious end at 2000⁄384 ≈ 5.2 s — `usable`, not `good` — against 2.6 s
  (`good`) for 2B. Old score 0.359 vs. 0.245 (4B ahead); new score 0.171 vs.
  0.245 (2B ahead). Old speed factor 1.000 → new 0.611 for 4B; 2B's stays
  1.000 either way.

Both are the same shape: not a slow processor-only machine (the usual case
this kind of move comes from), but a non-CUDA GPU path (Metal, Vulkan) whose
prompt processing is comparatively weak next to its own generation speed —
an asymmetry `ComfortableTPS` could never see, because it only ever looked
at generation. Per the task's instruction, the pin in `recommend_test.go`
was left exactly as written and no weight was retuned to chase it; whether
`qwen3.5:9b`/`qwen3.5:4b` should still win coding on these machines is a
product question for the fleet trial (do their prompts really run this slow
on Metal/Vulkan; does an 11 s or 5.2 s wait actually feel too slow for
"chat-style coding help" the way the `usable` bar assumes), not a bug in
this implementation.

**Resolved (2026-09-25, ARCHITECTURE.md D-59):** the cause was D-58's
ranking term, not the implementation. Scoring the wait against the
*excellent* bar penalised a wait inside `usable` far more than a stream at
the same grade. Itay chose this: the stream term applies to every purpose
(the old `ComfortableTPS` factor, unchanged), and the wait counts in the
ranking only past the purpose's `usable` bar. The cards still state the
wait. Every pin holds again (M1 Pro coding: `qwen3.5:9b`; Mac Pro coding:
`qwen3.5:4b`). Revisit after the fleet trial by moving the bars, not the
term.

## j. Show a generic "good for" score next to the speed figure — part 1 done (2026-09-25)

**From:** Itay, in-app testing feedback (2026-09-25). Depends on (i).

Once (i)'s matrix exists, surface it next to every generation-speed
`<Figure>` a person sees for a purpose — not just the one templated
sentence `reasons.go` builds for a card's top line today. Two parts to this,
matching how the app already keeps local and public numbers apart
(figure/public.go, `TestPublicAndLocalNeverShareAStruct`):

1. **A score or colour**, computed from (i)'s table against the model's own
   measured-or-estimated speed for the purposes it's being shown for
   ("excellent for coding," "usable for translation but not more"). This is
   a local, sourced figure (estimated or measured, like every other speed on
   this screen) scored against public/curated thresholds — needs its own
   `figure` treatment, not a bare adjective, so product rule 4's estimated/
   measured split still holds for it.
2. **"What is this good for"**, a tap-away explainer (product rule 2) in the
   same spirit as the glossary: which purposes this model's speed clears,
   and at what bar, from the same matrix — possibly the matrix itself,
   rendered for this one model's numbers rather than the abstract table.

Not scoped further than this until (i) has real numbers — the visual
treatment is the easy part; the data it displays is the open question.

**Part 1 done (2026-09-25):** the verdict beside every speed, measured on
Benchmarks; plus the Ollama menu entry.

- **One grading function.** `(*SpeedNeeds).GradeSpeed(gen, prompt, purpose)`
  (`internal/recommend/grade.go`) takes the answer and prompt-reading speeds
  as `figure.Rate` ranges (a measurement is a range of one point) and returns
  the grade at both ends, the wait, and which bar limits it
  (`Limit`: `answer_speed` | `wait`). D-58's arithmetic unchanged; the
  engine's `gradePurpose` is now a thin wrapper, and every pin in
  `recommend_test.go` and `TestFleetTopPicks` passed unchanged.
- **`recommend.SpeedVerdict`** (`verdict.go`, a sibling of `reasons.go`, same
  wording pieces): purpose, grades, limit, the wait as a `figure.Rate` (unit
  `s`), `source` inherited from the rates used, and the words (`text`,
  `wait_text` only when the wait holds the grade below excellent, `note`
  when it rests on less than both bars). In `server.APITypes()`, mirrored in
  `ui/src/api/types.ts`; never beside a `figure.Public`.
- **Where it is served**, for the purposes saved in settings (chat when
  none): `Recommendation.verdicts` (every card), `FileFit.verdicts`
  (`/api/models/{id}/fit` and `/detail` — measured once tested), and
  `bench.Run.verdicts` on every run the server serves (start, get, stream,
  cancel, history; filled by `internal/server/verdicts.go`, not stored).
  A benchmark grades each purpose at the suite prompt whose nominal size is
  the shortest at least the purpose's typical prompt
  (`recommend.MeasuredVerdicts` says it in code: chat/reasoning/writing →
  500, coding/vision/agent step → 2000, long documents → none); an unmeasured
  or absent prompt grades on answer speed alone and the note says so.
- **`advisor bench`** prints `Measured: … — …` under each run, and each
  note beneath.
- **UI:** `<SpeedVerdict>` / `<SpeedWithVerdict>` (`ui/src/components/`)
  on Benchmarks (the run with notes, each history row compact, and the plan's
  speed), Recommend and onboarding's cards (compact chip), the model detail
  page's "Your machine" block, and TryIt. Chips take `<Figure>`'s two
  treatments; every speed has the `tokens_per_sec` explainer one tap away
  (through the verdict line, or a `<Term>` beside the number when there is
  no verdict — Home's "your model" line too). The explainer now says the
  bars are provisional until tested on real use. No value in
  `speed-needs.yaml` was changed.
- **Ollama screen:** the placeholder is replaced by the backend status Home
  reads (state, version, detail) with Home's install and start cards,
  extracted to `ui/src/components/OllamaActions.tsx` (a small extraction, so
  the entry stays in the navigation). `Placeholder.tsx` is gone.

**Left for (j):**

- **Models screen:** not done. It lists installed models from
  `/api/models/installed`, which carries no estimate or measurement, so a
  verdict there needs new data on that endpoint (the fit of each installed
  catalogue file) — out of part 1's scope.
- **Measured verdicts outside Benchmarks grade every purpose at the
  headline prompt's speed** (the 500 prompt: `WithMeasurement` stores one
  prompt rate per configuration), so a tested model's coding verdict on
  Recommend or its detail page can read better than on Benchmarks, which
  uses the 2000 prompt. Storing the per-prompt rates in the evidence would
  make them agree.
- Not yet shown: the Compare view on Benchmarks, `advisor recommend`'s text,
  the "model you have" (`Current`) on Recommend.
- "What is this good for" as the matrix rendered for one model's numbers
  (part 2 of the original item) — the verdict line covers the purposes the
  user saved; the full per-purpose table for one model is not built.
- The fleet trial (D-58) still settles every `chosen` bar.

## k. Windows notifications never ask permission — the balloon API doesn't have one

**From:** Itay, in-app testing feedback (2026-09-25).

Itay noticed the Windows build's desktop notification popped up with no OS
permission prompt beforehand, and asked whether that's expected. It is, given
today's implementation: `internal/watch/notify_windows.go` shows the
notification via `System.Windows.Forms.NotifyIcon.ShowBalloonTip` — the
legacy system-tray "balloon tip" API. That API predates Windows' app
permission model and was never gated by it; any process can call it with no
prompt, then or ever. It is not a bug or an oversight in this code, just the
API's own behaviour.

The modern replacement — `Windows.UI.Notifications` "toast" notifications,
the ones that do show a permission-style entry in Windows' notification
settings — needs the app to have a registered identity (an AUMID), which in
turn needs an installer (`notify.go`'s own comment already flags this,
tied to build-plan step 11 packaging). macOS's `osascript display
notification` (notify_darwin.go) already goes through the real
`NSUserNotificationCenter`-style permission system and macOS does prompt for
it; Linux's `notify-send` (notify_linux.go) is closer to Windows — desktop
environments vary, most don't prompt either.

So: nothing to fix in the balloon-tip code itself, but when step 11
packages a real Windows installer, switching to the toast API (and getting
the permission prompt Itay expected) should go with it rather than being
left as a footnote. Until then, defaulting the watch's notification mode to
quiet (see `watch.DefaultSettings`, changed 2026-09-25) is the mitigation:
no popup, on any OS, until the user turns it on in Settings.

**Resolved in step 11 (2026-09-26).** `internal/winapp.RegisterIdentity`
registers a real AUMID (`ItayPollak.LocalLLMAdvisor`) and its display name
unconditionally on every daemon start — no installer-time step needed,
HKCU-only. `internal/watch/notify_windows.go` now posts a real
`Windows.UI.Notifications` toast under that AUMID instead of the balloon
tip, falling back to the balloon only if the toast call itself fails (no
AUMID registered yet, an old Windows build). ARCHITECTURE.md D-63 has the
decision; `claude/step-11-packaging.md` has what could and couldn't be
verified without a real Windows machine — the toast actually prompting
for permission and appearing on screen is one of the parts that still
needs one.

## l. Already downloaded models in benchmark screen should have their sizes as well

**From:** Itay, in-app testing feedback (2026-09-25).

The Benchmarks screen shows a list of installed models for the user to select
and run. Each model card in the list shows the model name and potentially other
metadata, but currently omits the model's file size. Since the user has already
downloaded these models and the size information is known (available in the
catalogue), displaying it would be helpful context: running a benchmark, knowing
how much VRAM the model will use (proportional to its file size for a given
quantization level), matters for setting expectations about what else can run
on the machine during the test, and whether the machine will need to offload
to system RAM.

UI location: `ui/src/screens/Benchmarks.tsx`, on the installed-models list or picker.
API: the model size is already available in the catalogue response
(`GET /api/models`); no new API needed.

Product rule 4: the size shown must carry its source (estimated, measured, or
from a public catalogue — currently all catalogue sizes come from Hugging Face
and are thus measured/public).

## m. Size gap between 9GB and 18GB model models in catalogue

**From:** Itay, in-app testing feedback (2026-09-25).

The curated catalogue has a noticeably sparse coverage of model sizes in the 9–18 GB
range. There are models at ~9 GB and models at ~18+ GB, but very few in between.
This gap makes recommendations harder: on machines with VRAM budgets in that
range (e.g., 12–16 GB cards), the advisor sometimes recommends either a model
that is smaller and less capable, or one that is larger and will barely fit,
with no "just right" option in between.

This is a data issue, not code: it means the curated families (`data/catalog/families.yaml`)
either are missing some sizes of existing families or are missing families altogether
in that size band. The decision on which families/sizes to add should come from:

1. **Popularity and recency** of models in the 9–18 GB range on Hugging Face.
2. **Coverage by public benchmarks** (Arena, Epoch): models in this range that
   are scored tend to recommend better than unscored ones (backlog item f).
3. **Ollama availability**: the model must have a published Ollama tag to be
   runnable by the customer.

Scoped as: a research task to identify 2–3 candidate models in the 9–18 GB range
that meet the above criteria, then a catalogue PR to add them. Assign once product
prioritizes recommendation quality on mid-size machines.

---

The five items below (n–r) are Itay's remarks of 2026-09-29. They are
scheduled in `BUILD_PLAN_PHASE2.md`, which says which step picks up each one.

## n. Models are big: pick the drive before the first download, and check space before every one

**From:** Itay, 2026-09-29. **Scheduled:** phase 2, steps P2-3 and P2-7.

When the app offers to install Ollama, it should warn that models are large
(a few GB to tens of GB each). On Windows especially, it should look at the
drives and recommend the one with the most free space. Bonus: before any
download, check there is room, and warn when the download would leave less
than about 10 GB free ("after this download you will have about X GB left").

What the code already has: `internal/hardware/storage.go` reads the free
space on the volume that holds (or will hold) Ollama's models folder.
`OLLAMA_MODELS` is read on all three OSes (`winprobe.go`, `macprobe.go`,
`linuxprobe.go`). `POST /api/models/pull` does not check free space today.

**Itay's open question: can the models live somewhere other than Ollama
itself?** Yes. Ollama's program and its models are separate. The installer's
`/DIR=` flag moves only the program. The models folder is set with
`OLLAMA_MODELS` (on Windows a user environment variable, default
`%USERPROFILE%\.ollama\models`, which is on C:). Recent Ollama desktop apps
may also offer a "model location" setting; P2-7 checks that against the
pinned release. So the drive choice belongs to the first download, not the
install. The program itself is only a few hundred MB to a couple of GB.
Setting the variable changes the machine, so it needs its own button
(product rule 5) and a decision about whether "delete everything" undoes
it (D-68). That decision is P2-2.

**Decided (P2-2, ARCHITECTURE.md D-72).** The pinned release's desktop app
does have a Model location setting, and once the app has saved its settings
(its own onboarding does) it ignores `OLLAMA_MODELS`. It also stops any
`ollama serve` it did not start. So the advisor sets no variable: on macOS
and Windows it recommends the drive and walks the person through Ollama's
own setting, then confirms from what Ollama reports. On the Linux user-space
install it sets the folder itself. Moving existing models is copy, verify,
switch, and delete the old copy only on a second click.

**Built (P2-3, `claude/p2-3-disk-space.md`).** `Backend.ModelsFolder`, the fresh
free-space check (`internal/diskroom`, 10 GB warn threshold, CHOSEN) and the note
beside every download button. A pull or an installer download that does not fit is
refused with both numbers before the runtime is called. "Keep models on another
drive" is still P2-7.

## o. "Looking at images": what is it, and is it a normal category?

**From:** Itay, 2026-09-29. **Scheduled:** phase 2, step P2-1.

It is a normal category. It means models that take a picture as input
alongside text: describing a photo, reading the text in a screenshot or a
scanned page (the OCR-like use), explaining a chart or a diagram. Hugging Face
calls it `image-text-to-text`, Ollama marks these models with a "vision"
capability, and Arena runs a separate Vision leaderboard. In the catalogue it
is `catalog.PurposeVision`. The estimator already counts the image reader's
memory (the `mmproj` / projector file).

The problem is the label. "Looking at images" does not say what the person
gets, and it could read as image *generation*, which these models do not do.
The fix: a clearer label ("Understanding pictures and screenshots"), a
one-line description with examples, and a note that it only helps if the
chat app lets you attach an image (Ollama's app does). Also be honest that
small models misread dense documents. Applies to both lists in
`ui/src/copy/en.ts` (Recommend and onboarding).

**Done (2026-09-29, P2-1).** Both labels now read "Understanding pictures and
screenshots"; every purpose has a one-line description under its checkbox
(written once, `purposeDescriptions` in `en.ts`, linked to the checkbox with
`aria-describedby`); the picture one gives the examples and the two limits
(attach the picture in the chat app; it does not create images). The UI copy
never says "vision" (`glossaryRule.test.ts` holds it), so no glossary entry.
**Left:** two strings the daemon sends still say it — the watch notification's
"Strong vision benchmark results" (`internal/watch/reasons.go`,
`publicPurposeWords`) and the Advanced note "includes the model's vision
encoder" (`internal/estimate/fit.go`). Go copy, outside a UI step; it wants
"pictures" / "image reader" ("its image reader" is already what
`weights.explain` says). The honest note about small models misreading dense
documents is not in the line yet.
Seen in `verify.command`'s output (2026-09-29): the cards' reasons still say
"looking at images" (`internal/recommend/reasons.go` `purposeWords`; also
`external/public.go`'s "at reading pictures"), so the label and the reasons
now name the purpose differently. Same fix, same Go step.

## p. Garbled context explainer: "…keep in mind at once. in mind at once"

**From:** Itay, 2026-09-29. **Scheduled:** phase 2, step P2-1.

Cause (not a typo): onboarding's card builds the sentence
`keepsInMind(words)` + `<Term id="context_window">` + `keepsInMindSuffix`
(`ui/src/onboarding/Recommendations.tsx:95-97`). `<Term>`
(`ui/src/components/Term.tsx`) opens its explainer *inline*, in the middle of
the sentence. With the `?` open, the reader sees: "Keeps about N words of
context window ? How much text — yours and its replies together — a model can
keep in mind at once. in mind at once." Any `<Term>` used mid-sentence
(about 16 uses) can do the same thing. Fix the component, not only this one
string: show the explainer beside or below the sentence, and add a test that
renders an open Term inside a sentence.

## q. Any model from Hugging Face: search, paste a link, or use a file you already have

**From:** Itay, 2026-09-29. **Scheduled:** phase 2, steps P2-2 (the decision),
P2-8, P2-9 and P2-10.

In the app: search Hugging Face, get an estimate first (does it fit, how
fast), then "Download and test". Also: paste a Hugging Face link, and point at
a model file already on this computer. Open question: explain what to look
for on Hugging Face, or keep all of this for advanced users?

This goes against three current rules, so it needs a decision first:
- D-65: `POST /api/models/pull` accepts only catalogue tags, and "nothing the
  user typed is ever sent anywhere". A search box sends the person's words to
  Hugging Face.
- D-64 / CLAUDE.md "Network": "the requests are a function of the data files
  and the version alone".
- The catalogue is curated so that a beginner is never shown fifty thousand
  repos (BUILD_PLAN.md decisions: "open discovery across all of HF is
  Phase 2", which is now).

The estimate path mostly exists already: `internal/catalog/hf` reads GGUF
headers with range requests, `internal/catalog/gguf` parses them, and
`internal/estimate.Fit` needs only header facts. Ollama can pull straight from
Hugging Face (`hf.co/{owner}/{repo}:{quant}`). A local file can go into
Ollama with `/api/blobs` + `/api/create`, which copies it into Ollama's own
folder, so it needs the disk check from (n).

**Decided (P2-2, ARCHITECTURE.md D-75 to D-77).** Search is allowed under its
own egress purpose, sent only on a click, with the screen saying the words go
to Hugging Face. It is its own screen, not behind Advanced, and never on
Recommend or in onboarding. A download is a curated tag or an id the server
issued for a file it resolved itself. Local files: a scan of LM Studio's
folder, the Hugging Face cache and Downloads on a click, plus a pasted path
under Advanced. An uncurated model gets a fit and a speed estimate, and no
purpose fit, public scores or recommendation card.

## r. From "this is the model I want" to a running model and an open chat

**From:** Itay, 2026-09-29. **Scheduled:** phase 2, steps P2-2 (the decision)
and P2-4. Supersedes backlog (g), which asked the same thing.

The biggest gap. A person can pick a model and download it, but the app stops
at "here is its name, go find a chat app" (step 7's "Use it" screen, D-52: a
chat app is detected, never driven). The goal is one button: start Ollama if
needed, load the model, and open a chat with that model already chosen.

Constraints to decide against (P2-2): D-52 (the advisor does not drive a chat
app), D-8 / D-65 (only the benchmark talks to a model, and only with the
suite's text), and product rule 1 (no terminal). Ollama's own desktop app
exists on macOS and Windows but not on Linux, so hand-off alone leaves Linux
users without a chat.

**Decided (P2-2, ARCHITECTURE.md D-74; Itay, 2026-09-29).** Hand-off only;
the advisor has no chat of its own. "Start chatting" loads the model with no
text, then opens the first chat surface this computer has: the runtime's own
page (none for Ollama; llama-server has one, phase 3), Open WebUI if it is
already running (opened on the model), or Ollama's app on macOS and Windows.
Ollama's app can't be told which model to use, so the advisor shows the exact
name to pick. On Linux with Ollama and no web chat running there is nothing
to open yet, and the screen says so; phase 3's llama-server page fills that
gap. P2-4 is Sonnet.
