// Package archtest holds no code the daemon runs. Its tests read the
// source of every other package and fail the build where the code has
// drifted from a rule ARCHITECTURE.md or CLAUDE.md states about its shape —
// the rules that are about where things may happen, which no unit test of
// a single package can see:
//
//   - network_test.go: only internal/egress opens connections to other
//     computers or builds HTTP clients; only internal/server listens, and
//     only through server.Listen (product rule 7, D-10, D-12, D-64); no
//     string the code runs names a download tool; nothing turns off TLS
//     verification.
//   - imports_test.go: the dependency direction of D-11 — nothing imports
//     internal/server or cmd, and the lower packages never import the
//     higher ones.
//   - model_test.go: the benchmark harness is the only caller of a
//     runtime's Generate (D-8), and a model is sent nothing but the
//     benchmark suite's text (product rule 7, D-44, D-65).
//   - deps_test.go: the direct dependencies are the ones ARCHITECTURE.md
//     names, every version is pinned (go.mod, ui/package.json and its
//     lockfile, ui/.npmrc), and every CI action is pinned to a commit
//     (D-69).
//
// A test here that fails is the review comment. Fix the code, or — when
// the rule itself should change — change ARCHITECTURE.md first and the
// test with it.
package archtest
