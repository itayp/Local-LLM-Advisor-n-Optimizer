# Local LLM Advisor & Optimizer — phase 3 outline

**What this adds.** A second and third runtime (llama.cpp and LM Studio), and
the one capability that makes llama.cpp worth doing first: using more than one
graphics card as a single pool. Backlog (c), (d) and (h), and the start of
PRD §18.

**This is an outline, not yet a plan.** The prompts get written when phase 3
starts, not now. They depend on the Backend methods phase 2 adds
(ARCHITECTURE.md D-78: `Capabilities`, `Load`, `ChatPage`, `Import`,
`ModelsFolder`, `SetModelsFolder`). P2-2 checked each against LM Studio's and
llama-server's documentation; D-78's table is what phase 3 re-checks against
running copies, as step 3 did for Ollama.
Prompts written today would describe an interface that is about to change.

## Why phase 3 and not phase 2

- **Phase 2 finishes the beginner's path; this phase widens who the app
  serves.** The customer (CLAUDE.md, one line) has never installed a runtime.
  A person who already runs LM Studio has a "will it fit" badge and a chat
  of their own. llama.cpp is run from a terminal. Both are real users, but
  not the one the product is built for.
- **The interface should settle before it gets a second implementation.**
  Phase 2 adds six Backend methods (D-78). Building llama.cpp in the same phase
  means designing each of them against three moving runtimes at once.
- **Multiple GPUs need hardware and documents that aren't in yet.** Itay's
  llama.cpp setup notes (backlog d) haven't been shared, and the Mac Pro is
  the only multi-GPU box in the fleet. BUILD_PLAN.md calls it "an edge case
  the product should survive, not design for".
- **Phase 2 already builds a bridge.** P2-10 lets models downloaded in LM
  Studio be used through Ollama, which covers the most common "I already
  have LM Studio" case without a second backend.

**Pull it forward if** step 13 or P2-12 shows testers already using LM Studio
and stopping because of it, or if AMD/Intel machines turn out to run much
better on llama.cpp's Vulkan than on Ollama's.

## Steps (draft)

| Step | What | Backlog | Model |
|---|---|---|---|
| P3-1 | Second runtime decision: runtime picker, where each runtime's models live, which one a recommendation targets, how "Start chatting" works per runtime | h | **Opus** |
| P3-2 | llama.cpp backend: find or install llama-server, supervise it as a child process on loopback, load / unload / running models, runtime path from its log; its built-in web page becomes the chat surface "Start chatting" opens (D-74), which gives Linux a one-click chat | h | **Opus** |
| P3-3 | Multiple GPUs: plan across cards (estimate.Fit per device and a split), llama.cpp's split options, setup guidance for integrated + dedicated, and comparing the expected GPU with the one the runtime used | c, d | **Opus** (Fable if the estimator's memory formula, D-20, has to change) |
| P3-4 | LM Studio backend: detect it, drive its local API and lms CLI, MLX models on Apple Silicon as a separate file type in the catalogue | h | **Sonnet** |
| P3-5 | Screens: runtime picker in Settings, per-runtime status on the Ollama screen (renamed "Runtimes"), benchmarks comparable across runtimes | h | **Sonnet** |
| P3-6 | Testers: one Mac with LM Studio, the Mac Pro on llama.cpp with both cards | — | **Itay + testers** |

Why the models: llama.cpp is the first time the backend interface earns its
keep, and it adds process supervision and a different load report. Multiple
GPUs change the estimator. Both are correctness work (Opus). LM Studio
follows once the second backend has proved the interface, and the screens
are bounded UI (Sonnet).

## After phase 3

PRD §18's remaining items, each needing its own decision first: the community
hardware database (a cloud endpoint and a privacy note before any code) and
workload quality evaluations (the first time the advisor itself runs a model,
as a local judge). Watch items, not steps: the Open WebUI desktop app's own
engine, MLX-native runtimes, and Ollama's Vulkan support leaving experimental.
