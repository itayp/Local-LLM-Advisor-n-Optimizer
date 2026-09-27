package egress

// This file is the audit. Every request the advisor makes to another
// computer goes to a host listed here, for one of the purposes listed here,
// over HTTPS — the transport Client builds refuses anything else, first hop
// and every redirect alike, and internal/archtest fails the build if any
// other package opens a connection of its own (ARCHITECTURE.md D-10, D-64).
//
// Adding a line here is a product decision, reviewed like one: say what is
// fetched, why, and what goes out with the request. Today the answer to the
// last question is the same for every line: the address itself, the
// advisor's name and version in the User-Agent, and nothing about the
// person or the computer. The paths the advisor asks for are a function of
// its own embedded data files (families.yaml, external.yaml, aliases.yaml)
// and its version — never of anything typed into it.
//
// The runtime's own API (Ollama on 127.0.0.1) is not a destination on this
// list: it is this computer. Local, beside Client, is the only way to reach
// it, and refuses to dial any address that is not loopback.

// Purpose is why a request leaves the machine. A client is built for one
// purpose and can reach only that purpose's hosts.
type Purpose string

const (
	// ModelList is the curated catalogue's files on Hugging Face: each
	// size's file listing and the first kilobytes of each tracked GGUF file
	// (the header, never the weights — D-33, D-34), and, for the new-model
	// watch, the list of repositories a curated family's maker has published
	// (D-57). Asked for by "Fetch the model list" and by the daily watch
	// when it is on.
	ModelList Purpose = "model_list"

	// PublicScores is the approved public benchmark sources (D-53, D-56):
	// Hugging Face's eval results on each size's original repository,
	// Arena's leaderboard files, and Epoch AI's benchmark data. Read after
	// the model list, on each source's own schedule.
	PublicScores Purpose = "public_scores"

	// OllamaDownload is Ollama's official installer and the checksum file
	// Ollama publishes beside it (D-29, D-66). Only ever after the person
	// clicks "Install Ollama".
	OllamaDownload Purpose = "ollama_download"

	// UpdateCheck is the Settings screen's "Check for updates" button: one
	// request for this project's latest release (D-62). Never on a timer.
	UpdateCheck Purpose = "update_check"
)

// Destination is one host the advisor may contact.
type Destination struct {
	// Host is an exact host name.
	Host string
	// Subdomains also admits any name ending in "."+Host (a CDN's regional
	// hosts), never a look-alike: "evilhuggingface.co" is not a subdomain
	// of "huggingface.co".
	Subdomains bool
	// PathPrefix, when set, is the only part of the host the advisor asks
	// for: a host that serves many parties' files (GitHub) is narrowed to
	// the one project the advisor has business with.
	PathPrefix string
	// For is the purposes this host serves.
	For []Purpose
	// What is fetched from it, in words — the line a reviewer reads.
	What string
}

// allowList is every host the advisor may contact. See the file comment.
var allowList = []Destination{
	{
		Host: "huggingface.co", Subdomains: true,
		For:  []Purpose{ModelList, PublicScores},
		What: "Hugging Face: the curated models' file listings and GGUF headers (range requests, no weights), makers' repository lists for the new-model watch, eval results on each model's original repository, and Arena's leaderboard dataset files",
	},
	{
		Host: "hf.co", Subdomains: true,
		For:  []Purpose{ModelList, PublicScores},
		What: "Hugging Face's download network (cdn-lfs, cas-bridge.xethub and regional CDN hosts) that huggingface.co redirects file reads to",
	},
	{
		Host: "epoch.ai",
		For:  []Purpose{PublicScores},
		What: "Epoch AI's benchmark data ZIP (Epoch's own runs only)",
	},
	{
		Host: "ollama.com", PathPrefix: "/download/",
		For:  []Purpose{OllamaDownload},
		What: "Ollama's official download links, which redirect to its GitHub release",
	},
	{
		Host: "github.com", PathPrefix: "/ollama/ollama/releases/",
		For:  []Purpose{OllamaDownload},
		What: "Ollama's GitHub releases: the installer and the sha256sum.txt Ollama publishes with every release",
	},
	{
		Host: "release-assets.githubusercontent.com",
		For:  []Purpose{OllamaDownload},
		What: "GitHub's download host, which github.com redirects release files to (signed, short-lived links)",
	},
	{
		Host: "objects.githubusercontent.com",
		For:  []Purpose{OllamaDownload},
		What: "GitHub's older download host for release files, still used for some redirects",
	},
	{
		Host: "api.github.com", PathPrefix: "/repos/itayp/-Local-LLM-Advisor-n-Optimizer/releases/",
		For:  []Purpose{UpdateCheck},
		What: "this project's latest release, when the person clicks Check for updates",
	},
}
