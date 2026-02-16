package cli

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"

	"github.com/rwx-cloud/cli/internal/docs"
)

type DocsSearchConfig struct {
	Query       string
	Limit       int
	Json        bool
	StdoutIsTTY bool
}

type DocsSearchResult struct {
	Query     string              `json:"Query"`
	TotalHits int                 `json:"TotalHits"`
	Results   []docs.SearchResult `json:"Results"`
}

type DocsPullConfig struct {
	URL  string
	Json bool
}

type DocsPullResult struct {
	URL  string `json:"URL"`
	Body string `json:"Body"`
}

func (s Service) DocsSearch(cfg DocsSearchConfig) (*DocsSearchResult, error) {
	resp, err := s.DocsClient.Search(cfg.Query, cfg.Limit)
	if err != nil {
		return nil, err
	}

	if len(resp.Results) == 0 {
		fmt.Fprintf(s.Stdout, "No results found for %q\n", cfg.Query)
		return &DocsSearchResult{
			Query:     resp.Query,
			TotalHits: resp.TotalHits,
			Results:   resp.Results,
		}, nil
	}

	// Single result: print the body directly
	if len(resp.Results) == 1 {
		result := resp.Results[0]

		if cfg.Json {
			return &DocsSearchResult{
				Query:     resp.Query,
				TotalHits: resp.TotalHits,
				Results:   resp.Results,
			}, nil
		}

		if result.Body != "" {
			fmt.Fprintln(s.Stdout, result.Body)
		} else {
			// Fetch full article if body not included
			body, err := s.DocsClient.FetchArticle(result.Path)
			if err != nil {
				return nil, err
			}
			fmt.Fprintln(s.Stdout, body)
		}

		return &DocsSearchResult{
			Query:     resp.Query,
			TotalHits: resp.TotalHits,
			Results:   resp.Results,
		}, nil
	}

	// Multiple results, non-TTY: print list
	if !cfg.StdoutIsTTY {
		if !cfg.Json {
			for _, r := range resp.Results {
				fmt.Fprintln(s.Stdout, r.Title)
				fmt.Fprintf(s.Stdout, "  %s\n", r.URL)
				if r.Snippet != "" {
					fmt.Fprintf(s.Stdout, "  %s\n", r.Snippet)
				}
				fmt.Fprintln(s.Stdout)
			}
		}

		return &DocsSearchResult{
			Query:     resp.Query,
			TotalHits: resp.TotalHits,
			Results:   resp.Results,
		}, nil
	}

	// Multiple results, TTY: numbered list with prompt
	for i, r := range resp.Results {
		fmt.Fprintf(s.Stdout, "  %d. %s — %s\n", i+1, r.Title, r.URL)
	}
	fmt.Fprintln(s.Stdout)
	fmt.Fprintf(s.Stdout, "Enter a number (1-%d): ", len(resp.Results))

	scanner := bufio.NewScanner(s.Stdin)
	if !scanner.Scan() {
		return nil, fmt.Errorf("no input provided")
	}

	choice, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
	if err != nil || choice < 1 || choice > len(resp.Results) {
		return nil, fmt.Errorf("invalid selection: %s", scanner.Text())
	}

	selected := resp.Results[choice-1]
	body, err := s.DocsClient.FetchArticle(selected.Path)
	if err != nil {
		return nil, err
	}

	fmt.Fprintln(s.Stdout, body)

	return &DocsSearchResult{
		Query:     resp.Query,
		TotalHits: resp.TotalHits,
		Results:   resp.Results,
	}, nil
}

func (s Service) DocsPull(cfg DocsPullConfig) (*DocsPullResult, error) {
	body, err := s.DocsClient.FetchArticle(cfg.URL)
	if err != nil {
		return nil, err
	}

	result := &DocsPullResult{
		URL:  cfg.URL,
		Body: body,
	}

	if cfg.Json {
		return result, nil
	}

	fmt.Fprintln(s.Stdout, body)

	return result, nil
}
