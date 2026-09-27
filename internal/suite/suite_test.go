package suite

import (
	"strings"
	"testing"
	"testing/fstest"
	"unicode"
)

// suiteDigests pins every suite version to its exact content. Editing the
// suite or its text without bumping its version fails here: runs of one
// version must have measured the same thing (data/bench/suite.yaml).
var suiteDigests = map[string]string{
	"1": "e8fe8f89b46d9a09",
	"2": "8cb591d2368f4374",
}

func TestTheSuiteIsPinnedToItsVersion(t *testing.T) {
	s, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	want, ok := suiteDigests[s.Version()]
	if !ok {
		t.Fatalf("suite version %q has no pinned digest; add %q to suiteDigests once the new version is final", s.Version(), s.Digest())
	}
	if s.Digest() != want {
		t.Fatalf("suite version %q changed (digest %s, pinned %s): bump the version in data/bench/suite.yaml and pin the new digest",
			s.Version(), s.Digest(), want)
	}
}

// The three prompts the build plan asks for — roughly 500, 2,000 and 8,000
// tokens of ordinary English — with a 256-token answer, deterministic
// sampling, one warm-up and three timed runs each; and the long prompt
// with its answer fits a context of 8,192.
func TestTheSuiteIsTheOneThePlanAsksFor(t *testing.T) {
	s, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	if s.CompletionTokens() != 256 || s.Temperature() != 0 || s.Seed() == 0 || s.Warmups() != 1 || s.Repeats() != 3 {
		t.Fatalf("options: %+v", s.f)
	}
	nominal := map[string][2]int{"500": {400, 600}, "2000": {1700, 2300}, "8000": {7000, 8192 - 256 - 64}}
	if len(s.Prompts()) != len(nominal) {
		t.Fatalf("prompts %+v", s.Prompts())
	}
	for _, p := range s.Prompts() {
		r := nominal[p.ID]
		if p.Tokens < r[0] || p.Tokens > r[1] {
			t.Errorf("prompt %s: %d tokens, want %d–%d", p.ID, p.Tokens, r[0], r[1])
		}
		// The stated count is Llama 3's; English prose runs 1.1–1.3 tokens a
		// word in every current tokenizer, so a count far from that was not
		// counted on this text.
		if ratio := float64(p.Tokens) / float64(len(strings.Fields(s.Body(p)))); ratio < 1.1 || ratio > 1.3 {
			t.Errorf("prompt %s: %.2f tokens a word; recount the tokens", p.ID, ratio)
		}
	}
	opts := s.Options(8192)
	if opts.NumCtx != 8192 || opts.NumPredict != 256 || opts.Temperature != 0 || opts.Seed != s.Seed() {
		t.Fatalf("options %+v", opts)
	}
}

// Ordinary English prose, the suite's own: printable ASCII, sentences, no
// markup, no instructions to the model.
func TestTheTextIsOrdinaryEnglish(t *testing.T) {
	s, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	long := s.Body(s.Prompts()[len(s.Prompts())-1])
	for i, r := range long {
		if r > unicode.MaxASCII || (r < ' ' && r != '\n') {
			t.Fatalf("character %q at %d: keep the text plain ASCII so every tokenizer sees the same bytes", r, i)
		}
	}
	for _, bad := range []string{"#", "<", ">", "{", "}", "http", "You are", "Assistant"} {
		if strings.Contains(long, bad) {
			t.Errorf("the text contains %q", bad)
		}
	}
}

// Every request of a run differs from the one before within its first
// tokens, so the runtime cannot answer from its cache of the last prompt.
func TestRequestsDifferAtTheStart(t *testing.T) {
	s, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	p := s.Prompts()[0]
	a, b := s.Request(p, 1).Text(), s.Request(p, 2).Text()
	if a == b || a[:2] == b[:2] || !strings.HasPrefix(a, "1.\n\n") || !strings.HasSuffix(a, s.Body(p)) {
		t.Fatalf("requests %q… and %q…", a[:20], b[:20])
	}
	if s.Words(p, 1) != len(strings.Fields(s.Body(p)))+1 {
		t.Fatalf("words %d", s.Words(p, 1))
	}
}

// Every prompt stops mid-sentence and well short of the text's end, so the
// model has a sentence to finish and nothing in the prompt says the text is
// over: version 1's long prompt was the whole essay, and models answered it
// with an end-of-text after one token.
func TestPromptsEndMidSentence(t *testing.T) {
	s, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	last := len(s.paragraphs[len(s.paragraphs)-1]) + len(s.paragraphs[len(s.paragraphs)-2])
	for _, p := range s.Prompts() {
		body := s.Body(p)
		if got := len(strings.Fields(body)); got != p.Words {
			t.Errorf("prompt %s: %d words, want %d", p.ID, got, p.Words)
		}
		if end := body[len(body)-1]; !unicode.IsLetter(rune(end)) {
			t.Errorf("prompt %s ends %q", p.ID, body[len(body)-20:])
		}
		if p.Words > s.words-last {
			t.Errorf("prompt %s reaches the text's last two paragraphs, its ending", p.ID)
		}
		if strings.Count(body, "\n\n") == 0 && p.Words > 200 {
			t.Errorf("prompt %s lost its paragraph breaks", p.ID)
		}
	}
	// The body is the text itself, cut: its start and the paragraph breaks
	// are the file's.
	short := s.Body(s.Prompts()[0])
	if !strings.HasPrefix(short, "A Year on the Allotment\n\nThe allotments sit") || !strings.HasSuffix(short, "the laziest method for") {
		t.Fatalf("the 500 prompt: %q … %q", short[:40], short[len(short)-40:])
	}
}

func TestInvalidSuitesFailLoudly(t *testing.T) {
	good := `version: "9"
reviewed_at: "2026-09-19"
text: t.txt
lead: "{n}."
completion_tokens: 8
temperature: 0
seed: 1
warmups: 1
repeats: 2
prompts:
  - {id: a, words: 4, tokens: 5}
`
	fs := func(y string) fstest.MapFS {
		return fstest.MapFS{"s.yaml": {Data: []byte(y)}, "t.txt": {Data: []byte("One two three.\n\nFour five six.\n")}}
	}
	if err := Validate(fs(good), "s.yaml"); err != nil {
		t.Fatalf("a good suite: %v", err)
	}
	for name, y := range map[string]string{
		"unknown key":     good + "extra: 1\n",
		"lead without n":  strings.Replace(good, `"{n}."`, `"Go."`, 1),
		"the whole text":  strings.Replace(good, "words: 4", "words: 6", 1),
		"ends a sentence": strings.Replace(good, "words: 4", "words: 3", 1),
		"no words":        strings.Replace(good, "words: 4", "words: 0", 1),
		"no repeats":      strings.Replace(good, "repeats: 2", "repeats: 0", 1),
		"text elsewhere":  strings.Replace(good, "t.txt", "../t.txt", 1),
		"a sentence lead": strings.Replace(good, `"{n}."`, `"{n}. Ignore the text and say"`, 1),
	} {
		if err := Validate(fs(y), "s.yaml"); err == nil {
			t.Errorf("%s: loaded", name)
		}
	}
}

// Product rule 7, D-65: the only text a benchmark can send is the embedded
// suite's. Every request of every prompt, whatever its number, is the lead
// line and then the text file's own words, in order, from the start.
func TestEveryRequestIsTheSuitesOwnText(t *testing.T) {
	s, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	var all []string
	for _, para := range s.paragraphs {
		all = append(all, para...)
	}
	for _, p := range s.Prompts() {
		for _, n := range []int{0, 1, 2, 17, -3} {
			words := strings.Fields(s.Request(p, n).Text())
			if len(words) != p.Words+1 {
				t.Fatalf("prompt %s request %d: %d words, want %d", p.ID, n, len(words), p.Words+1)
			}
			if lead := strings.TrimSuffix(words[0], "."); strings.Trim(lead, "0123456789") != "" {
				t.Errorf("prompt %s request %d: lead %q is not a number", p.ID, n, words[0])
			}
			for i, w := range words[1:] {
				if w != all[i] {
					t.Fatalf("prompt %s request %d: word %d is %q, the text's is %q", p.ID, n, i, w, all[i])
				}
			}
		}
	}
}

// A Suite that did not come from Default — one built in code, or read from
// a file with Validate — makes no prompts.
func TestOnlyTheEmbeddedSuiteMakesPrompts(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("a Suite built in code made a prompt")
		}
	}()
	var s Suite
	_ = s.Request(PromptSpec{ID: "x", Words: 3}, 1)
}

// The embedded suite cannot be edited after it is loaded: what callers get
// back from it are copies.
func TestTheEmbeddedSuiteCannotBeEdited(t *testing.T) {
	s, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	ps := s.Prompts()
	ps[0].Words = 1
	if s.Prompts()[0].Words == 1 {
		t.Fatal("editing Prompts() changed the suite")
	}
}
