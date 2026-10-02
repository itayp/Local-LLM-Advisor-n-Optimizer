# Phase 2 step P2-3 — free-space check before every download

**Date:** 2026-10-01. **Model:** Sonnet. **Backlog:** (n), the "check space before every
download" half. "Keep models on another drive" is P2-7 and is not built here.

## What it does

- **`Backend.ModelsFolder`** (D-78, read only). The Ollama adapter answers, in this order:
  1. the `FROM <folder>/blobs/sha256-…` line of `/api/show` for an installed local model
     (up to five tried, cloud models skipped);
  2. the `server config` line of the server's log: the advisor's captured log, Ollama's
     own `server.log` and `server-supervised.log` where the app runs the server, and
     `journalctl` for a systemd install. The last start in a log wins;
  3. `Known=false` with the OS default (`hardware.DefaultModelsDir`, which also reads the
     saved `OLLAMA_MODELS` on Windows), as where Ollama *will* put models.
  `Control` is `runtime_app`, `advisor`, `administrator` or `unknown` (unknown when Ollama
  is not installed). Nothing here sets the folder. The string `OLLAMA_MODELS` appears only in
  `internal/hardware` and `internal/backend/ollama` (`archtest`).
- **`internal/diskroom`**: the one function, `Checker.Check`, that P2-10 also calls. It reads
  the free space fresh (`hardware.FreeSpaceOn`) and returns `enough`, `low`, `not_enough` or
  `unknown`, with a templated sentence that has both numbers.
  `Config.LowAfter` is 10 GB, marked CHOSEN, with what would settle it.
- **Server.** `POST /api/models/pull` and `POST /api/backends/{name}/install` refuse with
  HTTP 507 `not_enough_room` *before* the runtime is called. New read-only routes:
  `GET /api/models/pull/check?ollama_tag=`, `GET /api/backends/{name}/install/check`,
  `GET /api/models/folder`. Settings' "open models folder" uses the runtime's folder.
- **UI.** One component, `RoomNote`, beside every download button: Recommend cards,
  onboarding's Download (disabled on `not_enough`), the Ollama installer, Benchmarks'
  download-and-test, and Settings' models folder. Strings are in `en.ts` (`room`); the
  sentences are the daemon's. Watch notifications link to Recommend or Benchmarks, so
  they get the check there.

## Judgement calls

- **Estimated vs measured (rule 4).** A catalogue file size for an Ollama tag is
  *estimated* ("about 9.1 GB", Ollama's own layers can differ). The installer's HEAD size is
  *measured*. `Left` takes the need's source. Free space is read from the OS: `source:"n/a"`.
- **507, not 409.** Benchmarks already treats 409 as "a pull is running".
- **Unknown never blocks.** Unknown free space, or an unknown size, says so and lets the
  person continue.
- **Control is `unknown` when Ollama is not installed.** No guess about who would run it.
- **The Linux installer's extraction size is not counted**, only the download.
- **"Keep models on another drive"** is not linked yet (P2-7); the action list carries only
  `remove_models`, and `actionLinks` in `RoomNote.tsx` is where P2-7 adds it.

## Not verified on a real machine

- Fixtures are shaped from Ollama v0.34.2 source (D-71 fact 9), not captured from a
  running Ollama. The Windows case (doubled backslashes in the log, a space in the path)
  is covered by a fixture; a real Windows run is the first check to make.
- The Windows registry read for the saved `OLLAMA_MODELS` is only vetted
  (`GOOS=windows go vet`), not run.

## Tests

`internal/diskroom` (each verdict on Windows, macOS and Linux paths; the exact margin; fresh
read each call), `internal/hardware/storage_test.go`, `internal/backend/ollama/folder_test.go`
(Show, log, journal, neither; fixtures in `testdata/folder/`), `internal/server/room_test.go`
(refused before the backend is called, folder checked is the runtime's, unknown never
blocks, installer), `internal/archtest` (layer table, `OLLAMA_MODELS`), and the UI
(`RoomNote.test.tsx`, disabled Download, the refusal shown in Getting).
