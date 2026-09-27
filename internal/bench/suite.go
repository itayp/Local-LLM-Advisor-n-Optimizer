package bench

import "advisor/internal/suite"

// The benchmark suite — data/bench/suite.yaml and its text — lives in
// internal/suite (ARCHITECTURE.md D-65): it is the only text the advisor
// ever sends a model, and that package makes that a property of the types
// (a suite.Prompt can only be made from the embedded suite) rather than of
// this harness's good behaviour. These names keep the harness's own code
// reading as it did.

// Suite is the benchmark suite.
type Suite = suite.Suite

// PromptSpec is one prompt of the suite.
type PromptSpec = suite.PromptSpec

// DefaultSuite is the suite embedded in the binary.
func DefaultSuite() (*Suite, error) { return suite.Default() }
