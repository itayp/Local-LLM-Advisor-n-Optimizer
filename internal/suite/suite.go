// Package suite is the benchmark suite — data/bench/suite.yaml and the text
// its prompts are cut from, embedded in the binary — and the only text the
// advisor ever sends a model.
//
// Product rule 7 says no prompt leaves the machine, and D-9/D-44 add that
// nothing the user typed is ever sent anywhere, and that the code should
// make that impossible rather than merely true (ARCHITECTURE.md D-65). This
// package is how:
//
//   - A Prompt has no exported field and exactly one maker, Request, on the
//     embedded suite. backend.GenerateRequest carries a Prompt, not a
//     string, so there is no way to hand a runtime text from anywhere else:
//     not from an API request, not from a setting, not from a file.
//   - The suite's data is unexported and read through methods, so the
//     embedded suite cannot be edited after it is loaded (its lead line
//     included), and a Suite built any other way than Default makes no
//     prompts.
//   - The runtime options that ride along (Options) are numbers.
//
// The suite's shape and the reasons for it are D-44 and D-50; the file's
// header comment is the schema of record.
package suite

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"

	"advisor/data"
)

// Suite is a loaded, validated suite. Its fields are read through methods.
type Suite struct {
	f          file
	paragraphs [][]string // the text: paragraphs, each its words
	words      int        // in the whole text
	digest     string
	// embedded is set by Default only: the one suite whose prompts may be
	// sent to a model.
	embedded bool
}

// file mirrors data/bench/suite.yaml, decoded strictly.
type file struct {
	Version          string       `yaml:"version"`
	ReviewedAt       string       `yaml:"reviewed_at"`
	Text             string       `yaml:"text"`
	Lead             string       `yaml:"lead"`
	CompletionTokens int          `yaml:"completion_tokens"`
	Temperature      float64      `yaml:"temperature"`
	Seed             int          `yaml:"seed"`
	Warmups          int          `yaml:"warmups"`
	Repeats          int          `yaml:"repeats"`
	Prompts          []PromptSpec `yaml:"prompts"`
}

// PromptSpec is one prompt of the suite.
type PromptSpec struct {
	ID string `yaml:"id"`
	// Words is the prompt's length: the text's first N words, its paragraph
	// breaks kept. The N-th word must end mid-sentence (a word with no
	// punctuation after it), so the model has a sentence to finish before it
	// can decide the text is over and stop (suite version 2; version 1 cut
	// at paragraph ends, and its longest prompt was the whole essay, which a
	// model answered with an end-of-text after one token).
	Words int `yaml:"words"`
	// Tokens is the prompt's length with the reference tokenizer (Llama 3's):
	// what decides, before a run, whether it fits a context. Each model's own
	// count comes back from the runtime.
	Tokens int `yaml:"tokens"`
}

// Prompt is text a benchmark sends a model: the embedded suite's own words
// and a numbered lead line, nothing else. It has no exported field; the
// only way to make one with text in it is (*Suite).Request on the suite
// Default returns. The zero Prompt is empty, and a runtime adapter refuses
// to send an empty prompt.
type Prompt struct{ text string }

// Text is what is sent.
func (p Prompt) Text() string { return p.text }

// Options are the runtime options every request of a run carries:
// deterministic sampling, the answer's budget, and the context under test.
// Numbers only — nothing that could carry text to a model.
type Options struct {
	Temperature float64
	Seed        int
	NumPredict  int
	NumCtx      int
}

// ErrSuite wraps every problem with a suite file.
var ErrSuite = errors.New("suite: invalid suite")

// Default is the suite embedded in the binary — the one whose prompts are
// sent to models.
func Default() (*Suite, error) {
	s, err := load(data.Files, data.BenchSuitePath)
	if err != nil {
		return nil, err
	}
	s.embedded = true
	return s, nil
}

// Validate reads and checks a suite from fsys: the curator's check of an
// edited suite, and the tests of what a bad one looks like. It returns no
// Suite: a suite read from anywhere but the binary makes no prompts.
func Validate(fsys fs.FS, suitePath string) error {
	_, err := load(fsys, suitePath)
	return err
}

func load(fsys fs.FS, suitePath string) (*Suite, error) {
	y, err := fs.ReadFile(fsys, suitePath)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSuite, err)
	}
	var s Suite
	dec := yaml.NewDecoder(bytes.NewReader(y), yaml.DisallowUnknownField())
	if err := dec.Decode(&s.f); err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrSuite, suitePath, err)
	}
	if s.f.Text == "" || strings.ContainsAny(s.f.Text, `/\`) {
		return nil, fmt.Errorf("%w: text must name a file beside the suite", ErrSuite)
	}
	text, err := fs.ReadFile(fsys, path.Join(path.Dir(suitePath), s.f.Text))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSuite, err)
	}
	if err := s.init(y, text); err != nil {
		return nil, err
	}
	return &s, nil
}

func (s *Suite) init(yamlBytes, text []byte) error {
	norm := func(b []byte) []byte { return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")) }
	for _, p := range strings.Split(strings.TrimSpace(string(norm(text))), "\n\n") {
		if ws := strings.Fields(p); len(ws) > 0 {
			s.paragraphs = append(s.paragraphs, ws)
			s.words += len(ws)
		}
	}
	h := sha256.New()
	h.Write(norm(yamlBytes))
	h.Write([]byte{0})
	h.Write(norm(text))
	s.digest = hex.EncodeToString(h.Sum(nil))[:16]

	f := s.f
	var problems []string
	bad := func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) }
	if f.Version == "" {
		bad("version is required")
	}
	if !strings.Contains(f.Lead, "{n}") {
		bad("lead must contain {n}, so no two requests share a prefix")
	}
	if strings.Count(f.Lead, "{n}") != 1 || len(strings.Fields(strings.ReplaceAll(f.Lead, "{n}", "0"))) != 1 {
		bad("lead must be one short word around {n}, not a sentence: it is sent to the model")
	}
	if f.CompletionTokens <= 0 {
		bad("completion_tokens must be positive")
	}
	if f.Warmups < 0 {
		bad("warmups cannot be negative")
	}
	if f.Repeats < 1 {
		bad("repeats must be at least 1")
	}
	if len(f.Prompts) == 0 {
		bad("at least one prompt")
	}
	seen := map[string]bool{}
	prev := 0
	for _, p := range f.Prompts {
		switch {
		case p.ID == "" || seen[p.ID]:
			bad("prompt %q: ids must be present and unique", p.ID)
		case p.Words < 1 || p.Words >= s.words:
			bad("prompt %q: words must be between 1 and %d, short of the whole text", p.ID, s.words-1)
		case !midSentence(s.lastWord(p)):
			bad("prompt %q: its last word %q ends a clause or a sentence; cut the prompt mid-sentence, so the model has one to finish", p.ID, s.lastWord(p))
		case p.Tokens <= prev:
			bad("prompt %q: prompts are listed shortest first, and tokens must be counted", p.ID)
		}
		seen[p.ID] = true
		prev = p.Tokens
	}
	if len(problems) > 0 {
		return fmt.Errorf("%w: %s", ErrSuite, strings.Join(problems, "; "))
	}
	return nil
}

// Version is the suite's version: part of every run's comparability key.
func (s *Suite) Version() string { return s.f.Version }

// Digest identifies the suite's exact content: the file and the text.
func (s *Suite) Digest() string { return s.digest }

// CompletionTokens is the answer's budget (num_predict).
func (s *Suite) CompletionTokens() int { return s.f.CompletionTokens }

// Temperature is the sampling temperature (0: the same answer every time).
func (s *Suite) Temperature() float64 { return s.f.Temperature }

// Seed is the sampling seed.
func (s *Suite) Seed() int { return s.f.Seed }

// Warmups is the untimed requests at the start of a run.
func (s *Suite) Warmups() int { return s.f.Warmups }

// Repeats is the timed requests per prompt.
func (s *Suite) Repeats() int { return s.f.Repeats }

// Prompts is the suite's prompts, shortest first (a copy).
func (s *Suite) Prompts() []PromptSpec { return append([]PromptSpec(nil), s.f.Prompts...) }

// Prompt finds a prompt by id.
func (s *Suite) Prompt(id string) (PromptSpec, bool) {
	for _, p := range s.f.Prompts {
		if p.ID == id {
			return p, true
		}
	}
	return PromptSpec{}, false
}

// Body is a prompt's text: the text's first N words, paragraph breaks kept,
// ending mid-sentence.
func (s *Suite) Body(p PromptSpec) string {
	var b strings.Builder
	left := p.Words
	for i, para := range s.paragraphs {
		if left <= 0 {
			break
		}
		if i > 0 {
			b.WriteString("\n\n")
		}
		n := min(left, len(para))
		b.WriteString(strings.Join(para[:n], " "))
		left -= n
	}
	return b.String()
}

// lastWord is the word a prompt ends on.
func (s *Suite) lastWord(p PromptSpec) string {
	left := p.Words
	for _, para := range s.paragraphs {
		if left <= len(para) {
			if left < 1 {
				return ""
			}
			return para[left-1]
		}
		left -= len(para)
	}
	return ""
}

// midSentence reports whether a word ends without punctuation: nothing
// after it closes a clause, a sentence or a quotation.
func midSentence(w string) bool {
	if w == "" {
		return false
	}
	r := w[len(w)-1]
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// Words counts the words of what Request(p, n) sends, for the
// tokens-per-word ratio a run learns from its first request.
func (s *Suite) Words(p PromptSpec, n int) int {
	return len(strings.Fields(s.requestText(p, n)))
}

// Request is the n-th request of a run for prompt p: the lead line,
// numbered, then the prompt's paragraphs, sent raw — no chat template, no
// system prompt. It is the only maker of a Prompt, and it makes one only
// from the embedded suite: called on any other Suite it panics, because a
// benchmark that could send other text is the thing this package exists
// to rule out.
func (s *Suite) Request(p PromptSpec, n int) Prompt {
	if !s.embedded {
		panic("suite: only the embedded benchmark suite's text can be sent to a model")
	}
	return Prompt{text: s.requestText(p, n)}
}

func (s *Suite) requestText(p PromptSpec, n int) string {
	if n < 0 {
		n = 0
	}
	return strings.ReplaceAll(s.f.Lead, "{n}", strconv.Itoa(n)) + "\n\n" + s.Body(p)
}

// Options are the runtime options every request of a run carries.
func (s *Suite) Options(numCtx int) Options {
	return Options{
		Temperature: s.f.Temperature,
		Seed:        s.f.Seed,
		NumPredict:  s.f.CompletionTokens,
		NumCtx:      numCtx,
	}
}
