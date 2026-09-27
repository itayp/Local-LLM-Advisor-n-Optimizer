package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

// Only a tag the curated catalogue names is handed to the runtime to
// download (D-65): the pull endpoint is not a way to send the runtime's
// registry — or Hugging Face through it — a name typed into a request.
func TestPullOnlyDownloadsWhatTheCatalogueNames(t *testing.T) {
	srv, ts := newCatalogTestServer(t, &fakeBackend{name: "ollama"})
	if err := srv.SyncCatalogue(context.Background()); err != nil {
		t.Fatal(err)
	}
	for tag, want := range map[string]int{
		"llama3.2:1b":                         http.StatusAccepted,
		"evil/model:latest":                   http.StatusBadRequest,
		"hf.co/someone/some-repo-GGUF:Q4_K_M": http.StatusBadRequest,
		"llama3.2:1b-instruct-q8_0":           http.StatusBadRequest, // a tag of the family the list does not name
	} {
		body, _ := json.Marshal(PullRequest{OllamaTag: tag})
		resp, err := http.Post(ts.URL+"/api/models/pull", "application/json", strings.NewReader(string(body)))
		if err != nil {
			t.Fatal(err)
		}
		var e APIError
		_ = json.NewDecoder(resp.Body).Decode(&e)
		resp.Body.Close()
		if resp.StatusCode != want {
			t.Errorf("pull %q: status %d, want %d", tag, resp.StatusCode, want)
		}
		if want == http.StatusBadRequest && e.Error.Code != "not_in_catalogue" {
			t.Errorf("pull %q: code %q, want not_in_catalogue", tag, e.Error.Code)
		}
		// Let the accepted pull finish before the next one is asked for.
		if want == http.StatusAccepted {
			for i := 0; i < 100 && srv.pulls.current().Status == "running"; i++ {
				time.Sleep(5 * time.Millisecond)
			}
		}
	}
}
