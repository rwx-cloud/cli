package providers

import "github.com/rwx-cloud/cli/internal/captain/config"

// TODO: add node_index, node_total
// - make it available to the run command
// - make it accessible with to the generic provider
// - expose it via job_tags

type GenericEnv struct {
	Who            string
	Branch         string
	Sha            string
	CommitMessage  string
	BuildURL       string
	Title          string
	PartitionIndex int `env:"RWX_TEST_PARTITION_INDEX" envDefault:"-1"`
	PartitionTotal int `env:"RWX_TEST_PARTITION_TOTAL" envDefault:"-1"`
}

func (cfg GenericEnv) MakeProvider() Provider {
	return Provider{
		AttemptedBy:   cfg.Who,
		BranchName:    cfg.Branch,
		CommitSha:     cfg.Sha,
		CommitMessage: cfg.CommitMessage,
		ProviderName:  "generic",
		JobTags:       map[string]any{"captain_build_url": cfg.BuildURL},
		Title:         cfg.Title,
		PartitionNodes: config.PartitionNodes{
			Index: cfg.PartitionIndex,
			Total: cfg.PartitionTotal,
		},
	}
}

// GitMetadataProvider is the subset of git functionality needed to populate generic env defaults.
type GitMetadataProvider interface {
	GetUser() string
	GetBranch() string
	GetHeadSha() string
}

// PopulateFromGit fills empty fields from git as lowest-priority defaults.
func (cfg *GenericEnv) PopulateFromGit(gitClient GitMetadataProvider) {
	if cfg.Who == "" {
		cfg.Who = gitClient.GetUser()
	}
	if cfg.Branch == "" {
		cfg.Branch = gitClient.GetBranch()
	}
	if cfg.Sha == "" {
		cfg.Sha = gitClient.GetHeadSha()
	}
}

func MergeGeneric(into GenericEnv, from GenericEnv) GenericEnv {
	into.Who = firstNonempty(from.Who, into.Who)
	into.Branch = firstNonempty(from.Branch, into.Branch)
	into.Sha = firstNonempty(from.Sha, into.Sha)
	into.CommitMessage = firstNonempty(from.CommitMessage, into.CommitMessage)
	into.BuildURL = firstNonempty(from.BuildURL, into.BuildURL)
	into.Title = firstNonempty(from.Title, into.Title)
	return into
}
