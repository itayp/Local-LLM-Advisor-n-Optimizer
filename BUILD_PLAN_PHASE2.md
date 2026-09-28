# Local LLM Advisor & Optimizer — phase 2 build plan

**What this adds.** The MVP (`BUILD_PLAN.md`, steps 0–13) takes a person from
"what can my computer run?" to a measured number and a model name. Phase 2
closes the three gaps that testing found: where models are stored and whether
there is room for them; using any model, not only the curated ones; and getting
from "this is the model I want" to a chat with it. The remarks behind each step
are in `claude/backlog.md`: items (n)–(r), dated 2026-09-29, and the open
earlier items (a), (f), (i), (j), (l) and (m). The backlog's status table says
where every item went.

**Read before any step:** `CLAUDE.md` (the product rules are still the spec),
`ARCHITECTURE.md` (in particular D-52, D-58, D-64, D-65 and D-68, which this phase
amends), and the backlog items the step names.

**Decisions already taken, do not relitigate them mid-step:**

- **The curated list is still the front door.** A beginner sees the
  recommendations. Searching all of Hugging Face is a second door, for people
  who already know what they want.
- **The advisor still calls no model to do its own job.** Anything P2-2 allows
  a model to receive is the person's own chat, started by the person, sent to
  this computer's runtime only.
- **Ollama is still the only runtime.** llama.cpp, LM Studio and multiple
  GPUs (backlog c, d, h) are phase 3 (`BUILD_PLAN_PHASE3.md`). Phase 2
  prepares for them in two ways: P2-2 checks every new backend method
  against their APIs, and P2-10 lets LM Studio's already-downloaded models
  be used through Ollama.
- **Nothing changes on the machine without a button that says what it will
  do.** That now includes setting where models are stored, moving them, and
  copying a file into Ollama.

## How to run this phase

As before: **one step per session, one model per step.** Paste the step's
prompt as the whole first message. Each step ends with a commit that names it
(`phase 2 step P2-3: free-space check before every download`) and a step doc
in `claude/` (`claude/p2-3-disk-space.md`).

| Step | What | Backlog | Model | Gate |
|---|---|---|---|---|
| P2-1 | Copy fixes: the explainer that breaks sentences, the "images" purpose, a copy button | a, o, p | **Sonnet** | — |
| P2-2 | Phase 2 decisions: models folder, chat, free text to Hugging Face, local files, backend methods checked against LM Studio / llama.cpp | n, q, r | **Fable** | Every later step builds on these |
| P2-3 | Free-space check before every download | n | **Sonnet** | — |
| P2-4 | From "this one" to a running model and an open chat | g, r | **Sonnet** (or **Opus**, see step) | The product's missing last mile |
| P2-5 | Models and Benchmarks screens: speed verdict everywhere, sizes, chat from the Models screen | j, l | **Sonnet** | — |
| P2-6 | Recommendation quality: the public-score bias and the 9–18 GB gap | f, m | **Opus** | Changes the picks on the golden profiles |
| P2-7 | Choosing and moving the models drive (Windows first) | n | **Opus** | Changes the machine; must not lose a model |
| P2-8 | Hugging Face search and links: estimate any model first | q | **Opus** | New input from outside the curated list |
| P2-9 | Download and test any Hugging Face model | q | **Sonnet** | — |
| P2-10 | A model file already on this computer, including LM Studio's models | q | **Sonnet** | — |
| P2-11 | The speed-needs fleet trial | i | **Itay** + **Sonnet** to record | Settles D-58's `chosen` bars |
| P2-12 | Testers again, on the new paths | all | **Itay + testers** | The only step that can say it works |

**Order.** P2-1 needs no decision and can go first, today. Then P2-2 to
P2-6, **before step 13's testers**. Without P2-4, a tester who gets a
recommendation has no way to talk to the model and will stop there, and that
finding is already known. P2-6 changes the picks testers will see. The MVP's
own open items (below) also come before step 13. P2-7 to P2-10 can follow
step 13, informed by what the testers did. P2-11 can run any time after P2-4.

**Before step 13, outside this plan.** These are MVP leftovers, listed in
`claude/step-11-packaging.md` and `claude/step-12-security.md`. They are
Itay's to do, not a session's: push a test tag so the release and packaging
jobs run on real macOS and Windows runners, and try the installers on real
machines (the tray, the Windows toast and the .dmg have never run for real);
decide on a LICENSE; turn on private vulnerability reporting; pin Inno Setup;
decide whether the watch stays on by default. P2-7 also fills
`backends.installed_version`, since it touches the install code.

**Why the models are where they are.** Fable once, for P2-2: it amends the
privacy promise (D-65) and the "does not chat" line (D-52) that the rest of
the product has been built around. A wrong call there costs every later step,
the same reason the skeleton and the estimator got Fable in the MVP. Opus
where correctness is the risk: P2-6 changes what the engine recommends and
needs research behind it; P2-7 moves gigabytes of someone's models and edits
their environment; P2-8 lets data from outside the curated list reach the
estimator and the API, so the security and "unknown is unknown" rules are
tested there. Sonnet for bounded work that follows a decision already written:
copy, the disk check, the UI flows.

---

---

## Step P2-1 — Copy fixes and the copy button (Sonnet)

```
Read CLAUDE.md, ui/src/components/Term.tsx, ui/src/copy/en.ts,
ui/src/copy/glossary.ts, ui/src/onboarding/Recommendations.tsx,
ui/src/screens/Recommend.tsx, ui/src/onboarding/UseIt.tsx, and
claude/backlog.md items (a), (o) and (p).

1. The explainer breaks the sentence it sits in (backlog p). <Term> renders
   its explanation inline, in the middle of the sentence, so onboarding's card
   reads "Keeps about N words of context window ? How much text — yours and
   its replies together — a model can keep in mind at once. in mind at once."
   Fix the component, not only this string. The explainer must not become part
   of the sentence around it: open it as a small popover anchored to the "?",
   or as a note below the sentence. Keep it phrasing-content-safe (Term sits
   inside <p>, see the comment in Term.tsx), keyboard reachable, closed with
   Escape, and readable at phone width. Keep tokens_per_sec's speed table
   working inside it. Then look at every <Term> use (grep "Term id=") with its
   explainer open and fix any sentence that still reads wrong. Add a test that
   renders an open <Term> inside a sentence and checks the sentence's own text
   stays whole.

2. The "Looking at images" purpose (backlog o). It is a standard category:
   models that take a picture as input alongside text. Rename it in both lists
   in en.ts (screens.recommend.purposes and onboarding.purposes.labels) to
   something like "Understanding pictures and screenshots", and give every
   purpose a one-line description shown under its checkbox (the step 7 prompt
   asked for one; check whether each purpose already has one and add the
   missing ones). For vision the line gives examples and one limit: "Describe
   a photo, read the text in a screenshot or scanned page, explain a chart.
   You attach the picture in your chat app. It does not create images." Add a
   glossary entry if the word "vision" appears anywhere the person can see it.

3. A copy button wherever a model's exact name is shown (backlog a). Pull
   UseIt.tsx's clipboard code into ui/src/components/CopyButton.tsx
   (value, a two-second "Copied" state, strings from en.ts, a quiet
   fallback when the clipboard is refused), use it in UseIt.tsx, and add it
   to Recommend's cards (pull_name), the Benchmarks picker and history
   rows, and the Models screen. P2-4 reuses it.

Strings only in en.ts. No API change. Run make check and the UI tests.
```

**Done when** every open explainer on Recommend, onboarding and Benchmarks
leaves its sentence readable, and a person can say what the vision purpose is
for from its label and its one line. Every exact model name on screen has
a copy button.

---

## Step P2-2 — Phase 2 decisions (Fable)

**Why this is a decision step.** Remarks (n), (q) and (r) each run into a rule
the code enforces today. Deciding them one step at a time would re-open the
privacy promise three times. This step writes the decisions once, as
superseding ARCHITECTURE.md entries, with the tests and archtest rules each one
implies. It writes no product code.

```
Read CLAUDE.md, ARCHITECTURE.md in full (especially D-4, D-8, D-9, D-44,
D-52, D-53, D-64, D-65, D-67, D-68, D-70), PRD §6–§8, §12, §17, §18, and
claude/backlog.md items (g), (n), (q), (r). Look at the code each rule lives
in: internal/egress/hosts.go, internal/server/pull.go, internal/suite,
internal/archtest, internal/chatapps, internal/hardware/storage.go.

Write new, superseding ARCHITECTURE.md entries (D-71 onward). Each states
the decision, what it amends, why, what it costs, and the tests or archtest
rules that will hold it. Check the facts against the Ollama release
runtime-support.yaml pins, not against memory. Decide:

1. The models folder (backlog n). May the advisor set where Ollama stores
   models (OLLAMA_MODELS as a user environment variable on Windows; the
   equivalent, if any, on macOS and on the Linux user-space install; Ollama's
   own app setting if the pinned release has one)? If yes: which button says
   it, when it is offered (before the first download, not at install; the
   installer's /DIR moves only the program), whether moving existing models
   is offered and how it avoids losing one (copy, verify, switch, and delete
   the old copy only on a second click), how Ollama is restarted, and what
   "delete everything" (D-68) does with the variable. Leaving a person's
   models unreachable is worse than leaving a variable behind; say which one
   you chose and why. List the variable in SECURITY.md's list of what the
   advisor writes outside its data folder.

2. The chat (backlog r, supersedes g). Compare, with facts:
   a. Hand-off: a "Start chatting" button that starts Ollama, loads the model
      (a new Backend method that loads with no text, which keeps D-8 intact),
      and opens Ollama's own app, or another detected chat app. Can the
      model be chosen for the person, or only named for them to pick? What
      happens on Linux, where Ollama has no desktop app?
   b. A minimal chat page inside the advisor, sent only to this computer's
      runtime through egress.Local. This amends D-52 and D-65. What stays
      sealed (the benchmark still only ever sends the suite), what the new
      path may carry, whether any conversation is stored (the default should
      be no), and which archtest rule replaces "only bench calls Generate".
   c. Both: hand-off first, the built-in page where no chat app exists.
   Pick one. The test: product rule 1 on all three operating systems, for a
   person who has never heard of Open WebUI. State whether P2-4 then needs
   Sonnet or Opus (Opus if it loosens a guarantee archtest enforces).

3. Free text to Hugging Face (backlog q). A search box sends the person's
   words to huggingface.co; so does a pasted link. Decide whether this is
   allowed and, if so, how narrowly: a new egress purpose (model_search) for
   the Hub's search API only; sent only on a button click; the screen says in
   words that the search goes to Hugging Face; nothing stored beyond what the
   screen needs. Decide how POST /api/models/pull accepts a model that is not
   in the catalogue without accepting free text. For example: the server
   builds the hf.co/{owner}/{repo}:{quant} tag itself from a repo and file it
   resolved and read a header from in this session, and refuses anything
   else. Say whether search lives behind the Advanced toggle.

4. A model file already on this computer (backlog q). A browser file picker
   cannot hand the daemon a path, and uploading GBs through the page is
   wrong. Decide between: a path the person types or pastes (Advanced); a
   scan of known folders (LM Studio's models folder, the Hugging Face cache,
   Downloads), done only on a button click; or both. Decide what the
   advisor may read outside its data folder (the GGUF header only, then the
   whole file when copying into Ollama) and say that the copy doubles the
   disk use until the person deletes the original.

5. What an uncurated model gets and does not get. Fit and speed estimates
   from its header: yes, with lower confidence and a sentence saying it is
   not on the advisor's list. Purpose fit, public scores, a place among the
   recommendation cards: decide, and say why.

6. Every new Backend method this phase needs (load with no text for the
   chat, pull from a Hugging Face source, create from a local file, set or
   report the models folder) is written once, for every runtime. Before
   you settle one, check it against LM Studio's local API and lms CLI and
   against llama-server's API, as step 3 did, so the phase 3 backends are
   a second file each and not a rewrite. Where a runtime cannot do
   something, the method says so through a capability, not an Ollama
   assumption in the caller. Name these in the entry.

Update CLAUDE.md where a rule changed (Network, "A model is sent the
suite's text and nothing else", the pull line) so it matches the new
entries, and adjust the P2-3 to P2-10 prompts in BUILD_PLAN_PHASE2.md if a
decision changed their scope. No product code in this step.
```

**Done when** Itay has read D-71 onward and agreed to each one. Every later
step's prompt matches them.

---

## Step P2-3 — Free-space check before every download (Sonnet)

```
Read CLAUDE.md, ARCHITECTURE.md D-71 onward (P2-2), internal/hardware/
storage.go, internal/server/pull.go, internal/server/install.go,
internal/estimate/config.go, and claude/backlog.md item (n).

1. Before a pull starts, compare its download size (the catalogue's file
   size; for an uncurated model, the size P2-8 resolves) with the free space
   on the models folder's volume, read fresh, not from the cached profile.
   - Not enough room: refuse with a sentence that gives both numbers and what
     to do ("This needs 9.1 GB and the drive Ollama saves models to has
     6.4 GB free. Remove a model you no longer use, or free up space").
     Link to the Models screen's Remove button. On Windows, once P2-7
     exists, also link to "Use another drive".
   - Room, but little left after: warn before the click, on the button's
     own screen: "After this download, about 7 GB will be left on C:." The
     threshold is a new constant in a config struct, marked CHOSEN, with
     the reason (10 GB: enough for Windows updates and a second small model)
     and what would settle it.
   Free space is a value read from the OS: source:"n/a" in the API types.
   A download size is a figure.Bytes with its source. When free space is
   unknown, say so and let the person continue. Never block on a value the
   advisor could not read.

2. The same check for the Ollama installer's download into the temp folder
   (install.go), using InstallSizer's size.

3. Every download button shows the check's result before it is clicked:
   Recommend cards, onboarding's "Download X GB", Benchmarks, the watch
   notifications' links. One shared UI component, strings in en.ts.

Tests: fixtures for enough / just enough / not enough / unknown on each OS
path; the pull handler refuses before it calls the backend.
```

**Done when** a download that would fill the disk is refused before it
starts, with both numbers in the message, and one that leaves less than the
threshold says how much will be left.

---

## Step P2-4 — From "this one" to a running model and an open chat (Sonnet, or Opus)

**Model.** Sonnet if D-71 chose hand-off only. Opus if it chose a built-in
chat page, because that step adds a second path to a model next to the
benchmark's sealed one and changes the archtest rule that holds D-8/D-65.

```
Read CLAUDE.md, ARCHITECTURE.md D-52, D-65 and D-71 onward (the chat
decision), internal/backend (the interface and the Ollama adapter),
internal/chatapps, internal/server/{backend,pull,chatapps,onboarding}.go,
ui/src/onboarding/UseIt.tsx, ui/src/screens/{Models,Recommend,Home}.tsx,
and claude/backlog.md items (g) and (r).

The goal, in the person's words: "I pick a model and I end up talking to
it." One flow, reachable from every place a model is shown (a Recommend
card, the Models screen, the end of onboarding, and later a Hugging Face
result), and one button at each stage that says what it will do:

  Not installed    → "Download 5.2 GB" (with P2-3's check)
  Ollama stopped   → "Start Ollama"
  Ready            → "Start chatting with <name>"

"Start chatting" does what D-71 decided, for example: load the model
(a Backend method that loads with no text, keep_alive long enough for a
conversation), then open the chat, with the model already chosen where
that is possible. Where it is not possible, show the exact name to pick,
with the copy button (backlog a), next to the opened app. Show what is
happening at each stage ("Loading Qwen3.5 9B into memory — usually about
20 seconds"), and say in words when the load fails and what to try.

Home's "one most useful thing" card becomes "Continue chatting with <name>"
once a model has been used this way. Record the last model started in the
settings table, not in history (it is a setting, not evidence).

If D-71 chose a built-in chat page: one screen, the model's name at the top,
streaming replies, a stop button, nothing stored unless D-71 said otherwise.
The text goes through the new sealed path D-71 defined, to egress.Local
only, with the archtest rule that holds it. It never offers a model to
switch to on its own (product rule 5, PRD §12).

Test on all three operating systems, including Linux, where Ollama has no
desktop app.
```

**Done when** a person who has never used the app goes from a recommendation
to a reply from that model without a terminal and without typing the model's
name, on macOS, Windows and Linux.

---

## Step P2-5 — Models and Benchmarks screens: the speed verdict everywhere, sizes, and where you start a chat (Sonnet)

**Why now.** After P2-4 the Models screen becomes the place a person goes
back to in order to use a model, so it has to show what the other screens
already show. Backlog (j) left the Models screen and a few views without the
speed verdict, and (l) asks for sizes on Benchmarks.

```
Read CLAUDE.md, ARCHITECTURE.md D-48, D-58, D-59, claude/backlog.md items
(j) (the "Left for (j)" list) and (l), internal/recommend/{grade,verdict}.go,
internal/server/{verdicts,modeldetail,benchmodels}.go, internal/bench
(WithMeasurement and the evidence it writes), ui/src/screens/{Models,
Benchmarks}.tsx, and ui/src/components/SpeedVerdict.tsx.

1. Models screen (j): each installed model the catalogue knows shows its fit
   and its speed with the verdict, measured once tested and estimated
   otherwise, through <SpeedWithVerdict>. That needs the fit of each
   installed catalogue file on /api/models/installed, or a sibling endpoint.
   Add the smallest one, list its type in server.APITypes(), and mirror it
   in ui/src/api/types.ts. A model the catalogue does not know says so in
   words (P2-9 and P2-10 will add models from outside the list). Each row
   gets P2-4's "Start chatting" button and P2-1's copy button.

2. One prompt rate per prompt size (j): WithMeasurement stores one prompt
   rate per configuration, so a tested model's coding verdict on Recommend
   reads better than on Benchmarks. Store the rate for each suite prompt in
   the evidence (a migration if it lands in a column; *_json if it grows)
   and grade each purpose from the same prompt Benchmarks uses
   (recommend.MeasuredVerdicts). Check that every pin in recommend_test.go
   and TestFleetTopPicks still holds. If one moves, report it and do not
   re-pin, as (i) did.

3. The rest of (j)'s list: the verdict on Benchmarks' Compare view, in
   `advisor recommend`'s text, and on Recommend's "the model you have" line.

4. Sizes on Benchmarks (l): every installed model in the picker and in the
   history shows its size on disk (a value read from the runtime,
   source:"n/a") and the memory it needs (a figure.Bytes, estimated or
   measured), labelled so they cannot be confused.
```

**Done when** every speed a person sees for a model, on every screen, has its
verdict with the same grade, and the Benchmarks picker shows each model's size.

---

## Step P2-6 — Recommendation quality: the public-score bias and the 9–18 GB gap (Opus)

**Why Opus.** It changes what the engine recommends on the golden profiles,
and part of it is research: which families to add, checked against three
sources. It goes before step 13 because testers will see these picks.

```
Read CLAUDE.md, ARCHITECTURE.md D-41, D-53, D-57, D-58, D-59,
claude/backlog.md items (f) and (m) in full, internal/recommend
(publicAdjustment, Config, recommend_test.go), data/catalog/{families,
aliases,external}.yaml, and the captures in .captures/external/ (their
dates are in the file names; re-capture with `advisor catalog external
-capture` if they are older than a month).

1. The bias (f, cause 2). publicAdjustment places a size among every
   curated size a signal scored, 70B included, so a scored small model lands
   near the bottom and loses up to 15%, while an unscored one gets 1.0.
   Having a score counts against a small model. Compare like with like
   instead. Backlog (f) option 1 favours size bands (effective parameters)
   over "the candidates on this machine"; test both on the golden profiles
   and pick with the numbers in front of you. Whatever you pick: an
   unscored size must not beat a scored one of the same band just for
   being unscored. Constants go in recommend.Config, marked. Write it up as
   a superseding ARCHITECTURE.md entry. Report every pin that moves, with
   old and new scores, and ask Itay before re-pinning any of them.

2. The wording (f, option 3). "No public data for this size" says why:
   public leaderboards rarely rate models this small. It also points to
   the two-minute test as the evidence that counts on this computer. The
   sentence is templated in reasons.go.

3. The catalogue (f option 2, m). Research candidates for the 9–18 GB
   download band and for scored small models (backlog f names Granite 4.2
   3B/8B, Qwen3 4B-Instruct-2507, Phi-4 14B, Mistral Small 3.2 24B as
   starting points; check what is current now, not only that list). A
   family goes in only if it has a GGUF repo, an Ollama tag, and passes
   `advisor catalog refresh` and `external -report` with its aliases.
   Add, don't replace. Each entry carries its review date and source.
   Never count the catalogue in prose.

4. Re-run `advisor recommend` for each golden profile and each purpose, and
   put the before/after table in the step doc.
```

**Done when** on the M1 Pro and the 12–16 GB card profiles, a scored model
competes on the same footing as an unscored one, the mid-size band has a
"just right" option, and Itay has agreed to every pin that moved.

---

## Step P2-7 — Choosing and moving the models drive (Opus)

**Why Opus.** This is the first step that edits a person's environment and
moves their files. Losing a model someone downloaded over hours, or leaving
Ollama pointed at an empty folder, is the failure to design against.

```
Read CLAUDE.md, ARCHITECTURE.md D-71 onward (the models-folder decision),
D-66, D-68, internal/hardware (storage.go, winprobe.go, sys_windows.go,
macprobe.go, linuxprobe.go), internal/backend/ollama (install_windows.go,
env.go, the Start/Detect code), internal/autostart (how it already writes
HKCU on Windows), internal/server/deletedata.go, and backlog (n).

1. Every drive, not only the models folder's. Windows: the fixed local drives
   (GetLogicalDrives + GetDriveType + GetDiskFreeSpaceEx; skip removable,
   network and optical drives; say why each skipped drive was skipped),
   with label, total and free. macOS and Linux: mounted local volumes where
   the person can write, external drives included and labelled as such.
   Through the hardware env seam, with txtar fixtures in each tool's real
   format, including a machine with C: nearly full and a large D:.

2. Before the first download (onboarding's Ollama screen and its
   Recommendations screen, and Settings): a short, plain warning that models
   are large, a few GB to tens of GB each. When another drive has much more
   room than the models folder's (the rule is a CHOSEN constant), recommend
   it: "Keep models on D: (412 GB free) instead of C: (38 GB free)", with a
   button that says what it will do. The button sets the models folder as
   D-71 decided, restarts Ollama if it is running, and confirms by asking
   Ollama (not by reading the variable back) where it now stores models.

3. Moving models that already exist (Settings → "Move my models to D:"),
   exactly as D-71 decided: check the space first; stop Ollama; copy blobs
   and manifests; verify every blob's sha256 against its name; switch the
   folder; start Ollama; check the same models list comes back. Only then
   offer "Delete the old copy (frees 23 GB)" as its own click. A failure at
   any point leaves the old folder in use and says what happened. Show
   progress in bytes, and let the person cancel, which rolls back.

4. "Delete everything" and SECURITY.md follow D-71's decision about the
   variable.

5. While in the install code: write `backends.installed_version` from the
   release tag the verified install already knows (`download` returns it;
   step 12 found it is never written).

Windows is the gate. macOS and Linux get the drive list and the
recommendation if D-71 found a supported way to set the folder there, and
otherwise a sentence saying how.
```

**Done when**, on a Windows machine with two drives, a person picks the other
drive before their first download, the model lands there, and a second machine
with models already on C: moves them to D: and still lists and runs every one.

---

## Step P2-8 — Hugging Face search and links: estimate any model first (Opus)

**Why Opus.** This is the first time a model the curators did not review
reaches the estimator, the API and the screen. Headers can be odd or hostile,
architectures can be unknown, and the "unknown is unknown" rule must hold.

```
Read CLAUDE.md, ARCHITECTURE.md D-34, D-53, D-64, D-65 and D-71 onward
(free text to Hugging Face; what an uncurated model gets),
internal/catalog/hf, internal/catalog/gguf, internal/catalog/refresh,
internal/estimate, internal/egress/hosts.go, internal/archtest, and backlog (q).

1. Search, as D-71 allowed: a new egress purpose for the Hub's model search
   only, GGUF repos only, sorted by downloads, a small page of results. Sent
   only on the Search button. The screen says in words that the search words
   go to Hugging Face. A pasted link or "owner/repo" is parsed on the server,
   the host checked against Hugging Face's own names, and anything else
   refused with a sentence.

2. For a result: list the repo's GGUF files grouped by size and quant (the
   catalogue's grouping code), pick the default the way the catalogue does,
   read the header by range request (never the whole file), and run
   estimate.Fit and the speed range for this machine. Show the answer the
   way a recommendation shows it (fits / tight / does not fit, the reasons,
   the download size, the estimate treatment) and say plainly that this
   model is not on the advisor's list: no purpose fit, no public scores
   unless D-71 said otherwise, lower confidence. An architecture the parser
   or the estimator does not know gets "can't estimate this one" and why,
   never a guessed number. Show the licence and whether the file includes an
   image reader.

3. "What to look for" (backlog q): a collapsible guide on the search screen,
   strings in en.ts, glossary terms through <Term>. It covers: GGUF files (the
   format Ollama runs), instruct/chat versions rather than base models, Q4_K_M
   as the usual balance, uploaders people trust (the lab's own organisation,
   well-known quantisers), the licence, and that "fits" here is an estimate.
   Better still, show as badges on each result what the advisor can check
   itself (has GGUF files, instruct, from the lab's own organisation, has an
   image reader), so the guide explains the badges rather than asking the
   person to check by hand.

4. Where it lives: the screen and toggle D-71 chose. Register it in
   ui/src/screens/index.ts.

Security tests: a header that lies about its size, a redirect off Hugging
Face, a link to another host, a repo with no GGUF, a whole-file answer to a
range request, a search that returns a thousand results. The archtest rules
still pass with the new egress purpose, and nothing else builds a client.
```

**Done when** a person can search for a model, open a result, and see whether
it fits this machine and roughly how fast it will run, in the estimate
treatment, before any download, and a pasted link does the same.

---

## Step P2-9 — Download and test any Hugging Face model (Sonnet)

```
Read CLAUDE.md, ARCHITECTURE.md D-71 onward (how pull accepts an uncurated
model), P2-8's code, internal/server/pull.go, internal/bench, and P2-4's flow.

From a P2-8 result: "Download 5.2 GB and test it". POST /api/models/pull
accepts an uncurated model exactly as D-71 specified, for example a
reference to a repo and file the server resolved in P2-8, from which it
builds the hf.co/{owner}/{repo}:{quant} tag itself, and never a tag string
from the request. P2-3's space check applies. When the pull finishes, offer
the two-minute test (the benchmark suite, unchanged) and show the
measurement next to the estimate it replaces, as onboarding does. Then
P2-4's "Start chatting" flow.

On the Models screen, such a model shows where it came from ("from Hugging
Face, not on the advisor's list") and its measured or estimated speed like
any other. The watch (step 10) does not track it. If Ollama cannot run the
file (no chat template, an unsupported architecture), say so in words and
offer Remove.
```

**Done when** a model found by search is downloaded, tested and opened in chat
from inside the app, and it appears on the Models screen labelled as coming
from Hugging Face.

---

## Step P2-10 — A model file already on this computer, including LM Studio's models (Sonnet)

```
Read CLAUDE.md, ARCHITECTURE.md D-68 and D-71 onward (local files),
internal/catalog/gguf, internal/estimate, internal/backend/ollama, and
P2-8's result screen.

As D-71 decided (a path the person pastes, a scan of known folders on a
button click, or both): find the .gguf file, read its header locally, and
show the same estimate screen as P2-8, with no network request. "Add to
Ollama (copies 5.2 GB)" pushes the file with /api/blobs and creates the
model with /api/create, with P2-3's space check (the copy doubles disk use
until the original is deleted; say so). The model name is built by the
advisor from the file name, cleaned, never taken raw. A split or partial
file, a file that is not GGUF, an image reader (mmproj) next to the
weights: each gets a sentence. Then the test and P2-4's chat.

Nothing is read outside the data folder except the file the person chose,
or the folders D-71 lists for the scan. Tests use t.TempDir() files built
from the existing GGUF header fixtures.
```

**Done when** a GGUF downloaded earlier with a browser or LM Studio is
estimated, added to Ollama, tested and opened in chat, without a terminal. A
person with LM Studio installed sees its downloaded models offered, if D-71
allowed the scan.

---

## Step P2-11 — The speed-needs fleet trial (Itay, with a Sonnet session to record it)

**Why here.** D-58's speed bars are marked `chosen` until real use settles
them (backlog i). The trial needs Itay to use models at different speeds for
real tasks. That is much easier once P2-4 opens a chat in one click, which is
why it waits until then.

Itay: for each purpose you actually use (chat, coding, writing, long
documents; vision if P2-1's relabel makes it one you'd try), use two or three
installed models of clearly different speeds on the fleet machines, for a real
task, and note for each: the words a second and the wait before the first word
(both on the Benchmarks screen), and one word, "great / fine / annoying / too
slow". A dozen sessions spread across purposes is enough to move a bar.

```
Read CLAUDE.md, ARCHITECTURE.md D-58 and D-59, data/recommend/
speed-needs.yaml, and Itay's notes (pasted below or in claude/).

Record the sessions in claude/p2-11-fleet-trial.md as a table. For each
`chosen` value in speed-needs.yaml, say whether the notes support it, move
it, or say nothing about it. Change only the values the notes settle, mark
each changed value as measured with a pointer to the table, and leave the
rest `chosen`. Run the recommend tests; report any pin that moves and do not
re-pin without Itay. Add a superseding ARCHITECTURE.md entry if a bar moved.
```

**Done when** every bar the notes cover is marked as settled by them, and the
explainer's "provisional" line says which bars still are.

---

## Step P2-12 — Testers again, on the new paths (Itay + testers)

If step 13 has not run yet, run it with P2-1 to P2-6 in place, and
add these questions. If it has, find two new people, one on a Windows PC
with a small C: drive:

1. After the recommendation, did they reach a reply from the model without
   asking anything?
2. Did the drive and free-space messages make sense, and did anyone pick
   the other drive?
3. Shown the Hugging Face search, did they use it, and did the badges and
   the guide help or scare them off?
4. Where did they stop, and what did they say?

Record each finding as its own file under `backlog/`, as step 13 says.

---

---


## Not in this phase

Phase 3 (`BUILD_PLAN_PHASE3.md`): the llama.cpp and LM Studio backends
(backlog h), pooling multiple GPUs (d), and multi-GPU setup guidance (c).
Then PRD §18's later items. The backlog's status table lists everything.
