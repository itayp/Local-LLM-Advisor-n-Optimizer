# Phase 2 step P2-2 — the phase's decisions

**Date:** 2026-09-29. **Model:** Fable. **Code changed:** none (docs only).
**Gate:** Itay reads ARCHITECTURE.md D-71 to D-78 and agrees to each one.
Every later step's prompt in `BUILD_PLAN_PHASE2.md` has been rewritten to
match them.

## What was checked, and against what

Everything was read from source or documentation in this session. None of
it has been run on the fleet yet.

| What | Version read | Where |
|---|---|---|
| Ollama, desktop app included | v0.34.2, tag commit `dfabde45` (the release `runtime-support.yaml` pins) | `github.com/ollama/ollama`, `app/`, `server/`, `types/model/`, `envconfig/`, `internal/modelref/`, `docs/` |
| LM Studio | docs `9b8bc20` (2026-09-08); `lms` `1b7181b` (2026-09-25) | `github.com/lmstudio-ai/docs`, `github.com/lmstudio-ai/lms` |
| llama-server | `ggml-org/llama.cpp` `526c43b` (2026-09-29) | `tools/server/README.md` |
| Hugging Face | `huggingface_hub` `f151320`; `hub-docs` `761bb84` | `hf_api.py` (`list_models`), `docs/hub/ollama.md` |
| Open WebUI | `open-webui/open-webui` `8bd8b4f` (2026-09-21) | `backend/open_webui/main.py` (`/api/config`), `src/lib/components/chat/Chat.svelte` (`?model=`) |

D-71 lists the findings with a file for each.

## The decisions, one line each

- **D-71.** These are the facts the phase decides against. Several differ
  from what Ollama's own docs say.
- **D-72. Models folder.** The advisor sets no environment variable. The
  folder is the one Ollama reports. With Ollama's app (macOS, Windows) the
  person changes it in Ollama's own Settings, guided by the advisor. For the
  Ollama the advisor runs itself (Linux user-space), the advisor sets it.
  Moving models is copy, verify, switch, then delete the old copy on its own
  click. "Start Ollama" opens the app where the app is installed.
- **D-73.** Cloud (and linked) models are neither tested nor loaded.
- **D-74. Chat.** Hand-off only; the advisor has no chat of its own (Itay's
  review, below). "Start chatting" loads the model with no text, then opens
  the first chat surface the computer has: the runtime's own page (none for
  Ollama), Open WebUI if running (opened on the model), or Ollama's app on
  macOS and Windows (with the name to pick). On Linux with Ollama there is
  nothing to open yet, and the screen says so; phase 3's llama-server page
  fills that gap. P2-4 is **Sonnet**.
- **D-75. Hugging Face search.** Allowed under a new egress purpose
  (`ModelSearch`), sent only on a click, and not behind Advanced. A pull
  takes a curated tag or a server-issued id, never a tag string.
- **D-76. Local files.** A folder scan on a click (LM Studio's folder, the
  Hugging Face cache, Downloads) and a pasted path under Advanced. Only the
  header is read until the person adds the file. The copy doubles disk use.
- **D-77. Uncurated models.** They get fit and speed. They get no purpose
  fit, no public scores and no recommendation card.
- **D-78. Backend.** Six new methods (`Capabilities`, `Load`, `ChatPage`,
  `Import`, `ModelsFolder`, `SetModelsFolder`), written once for all three
  runtimes, with `Capabilities` for where they differ.

## Itay's review (2026-09-29)

- **D-74 changed.** The first draft proposed a minimal chat page inside the
  advisor, option (b). Itay: "the chat shouldn't be in this app, it should
  only open what's relevant — whether it's Ollama itself or a web page with
  the chat if that's how it works on the device, and if not available, we
  can add later, when we have other programs that do support it (like
  llama.cpp)." D-74 now records (a), the hand-off, with (b) and the reason
  it was not chosen. D-73, D-78, CLAUDE.md, the P2-4 prompt and the backlog
  follow. P2-4 goes back to Sonnet. D-8, D-65 and the archtest rules that
  hold them stay exactly as they were.

## Needs Itay's explicit agreement

1. **Product rule 7's reading (D-75).** A search the person runs, sent to
   Hugging Face with the screen saying so, is treated as not being the
   "usage data" the rule forbids. Rule 7's text is verbatim from
   `BUILD_PLAN.md` and was not changed. If you read it more strictly,
   search moves behind Advanced or is dropped, and P2-8's prompt changes.
2. **Linux with Ollama gets no chat to open in phase 2 (D-74).** Everything
   up to a loaded, tested model works without a terminal; the last step
   waits for phase 3's llama-server page, or for a web chat the person runs.
3. **Guided steps instead of one button on macOS and Windows (D-72).** The
   drive choice takes a few clicks in Ollama's own Settings. The advisor
   cannot make it one button without writing another app's private
   settings, which D-72 rules out.
4. **Search not behind Advanced (D-75).** P2-12's testers are the check on
   this.

## Found while checking: things that are wrong today

These are fixed by the steps named, not in this step.

- **A benchmark of an Ollama cloud model would send the suite's text to
  ollama.com** and time Ollama's servers. The advisor has no idea of cloud
  models (`grep -ri remote_host` finds nothing). Fixed in P2-4 (D-73).
- **"Start Ollama" on macOS and Windows runs `ollama serve` directly.** That
  server ignores the app's Model location, so anyone who changed it sees
  their models missing. It is also stopped by the app as soon as the person
  opens the app (D-71, facts 1 and 2). Fixed in P2-4 (D-72).
- **The models folder the advisor shows** (`hardware.Storage.ModelsDir`) is
  read from `OLLAMA_MODELS`. It is wrong for anyone who set Ollama's Model
  location in the app, and the free-space figure is read on that wrong
  folder. Fixed in P2-3 (D-72).
- **"Use it" lists LM Studio as a place to chat with the model**, but LM
  Studio cannot use Ollama's models. Fixed in P2-4 (D-74).
- **Ollama's app, opened fresh, picks a model of its own**, often a cloud
  one (D-71, fact 4). The hand-off tells the person which model to pick and
  what "cloud" means. Built in P2-4 (D-74).

## Not verified, and who verifies it

- D-71's facts 1, 2, 4 and 5 are read from source, not seen on a machine.
  P2-4 checks them on a real Mac and a real Windows PC before it builds on
  them, and stops if one is wrong.
- The `:local` suffix (D-71, fact 5; D-73) is taken from source; P2-4's
  adapter tests and a fleet run confirm it.
- Open WebUI's `/api/config` and `?model=` (D-74) are taken from source;
  P2-4 checks them against a real Open WebUI before relying on them.
- D-78's LM Studio and llama-server columns are from documentation. Phase 3
  checks them against running copies, as step 3 did for Ollama.

## Files changed

- `ARCHITECTURE.md`: D-71 to D-78 added. Pointers added at the top of D-4,
  D-8, D-9, D-16, D-27, D-29, D-52, D-64, D-65 and D-68 to the entries that
  amend them. Status line and "Read with" updated.
- `CLAUDE.md`: the rules the code enforces (only the suite reaches a model,
  and `Load` has no text; no chat in the advisor; a model on this computer;
  downloads by curated tag or resolved id; `ModelSearch`), the `internal/suite` and `internal/archtest` layout
  lines, Local data (no environment variable), and Network.
- `SECURITY.md`: states that the app never changes the account's
  environment variables (`OLLAMA_MODELS` included) or Ollama's settings.
  That is true today and stays true. The rows for search, Hugging Face
  downloads, the move and the scan are added by P2-7 to P2-10 when those
  ship, so that SECURITY.md never describes behaviour that doesn't exist
  yet.
- `BUILD_PLAN_PHASE2.md`: P2-3, P2-4, P2-7, P2-8, P2-9 and P2-10 rewritten
  to match; P2-4 stays Sonnet; P2-5's Models screen handles remote models;
  the phase's "decisions already taken" and "read before any step" point to
  D-71 to D-78.
- `BUILD_PLAN_PHASE3.md`: names D-78's methods.
- `claude/backlog.md`: (n), (q) and (r) marked decided, with a paragraph
  each.
