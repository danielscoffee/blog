package httpapp

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/danielscoffee/danielscoffee.me/internal/content"
)

type SearchIndexer struct {
	docs []content.SearchDoc
}

type SearchResult struct {
	Type    string `json:"type"`
	Title   string `json:"title"`
	Slug    string `json:"slug"`
	Date    string `json:"date"`
	Summary string `json:"summary"`
	URL     string `json:"url"`
}

func NewSearchIndexer(docs []content.SearchDoc) *SearchIndexer {
	return &SearchIndexer{docs: append([]content.SearchDoc(nil), docs...)}
}

func (s *SearchIndexer) Search(raw string) []SearchResult {
	queryType, query := parseQuery(raw)
	if query == "" && queryType == "" {
		return nil
	}

	tokens := strings.Fields(strings.ToLower(query))
	type scored struct {
		doc   content.SearchDoc
		score int
	}
	scoredDocs := make([]scored, 0, len(s.docs))

	for _, doc := range s.docs {
		if queryType != "" && doc.Type != queryType {
			continue
		}
		if queryType != "" && len(tokens) == 0 {
			scoredDocs = append(scoredDocs, scored{doc: doc, score: 1})
			continue
		}
		score := scoreDoc(doc, tokens)
		if score == 0 {
			continue
		}
		scoredDocs = append(scoredDocs, scored{doc: doc, score: score})
	}

	sort.Slice(scoredDocs, func(i, j int) bool {
		if scoredDocs[i].score == scoredDocs[j].score {
			return scoredDocs[i].doc.Date > scoredDocs[j].doc.Date
		}
		return scoredDocs[i].score > scoredDocs[j].score
	})

	limit := 20
	if len(scoredDocs) < limit {
		limit = len(scoredDocs)
	}
	results := make([]SearchResult, 0, limit)
	for _, item := range scoredDocs[:limit] {
		results = append(results, SearchResult{
			Type:    item.doc.Type,
			Title:   item.doc.Title,
			Slug:    item.doc.Slug,
			Date:    item.doc.Date,
			Summary: item.doc.Summary,
			URL:     docURL(item.doc),
		})
	}
	return results
}

func (s *Server) searchHandler(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	results := s.searchIndexer.Search(q)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"results": results}); err != nil {
		s.logger.Error().Err(err).Msg("encode search response failed")
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
}

func parseQuery(raw string) (string, string) {
	q := strings.TrimSpace(raw)
	switch strings.ToLower(q) {
	case "blog":
		return "blog", ""
	case "projects":
		return "projects", ""
	}
	lower := strings.ToLower(q)
	if strings.HasPrefix(lower, "blog ") {
		return "blog", strings.TrimSpace(q[len("blog "):])
	}
	if strings.HasPrefix(lower, "projects ") {
		return "projects", strings.TrimSpace(q[len("projects "):])
	}
	return "", q
}

func scoreDoc(doc content.SearchDoc, tokens []string) int {
	title := strings.ToLower(doc.Title)
	summary := strings.ToLower(doc.Summary)
	body := strings.ToLower(doc.Body)
	tags := strings.ToLower(strings.Join(doc.Tags, " "))

	score := 0
	for _, token := range tokens {
		if strings.Contains(title, token) {
			score += 50
		}
		if strings.Contains(tags, token) {
			score += 30
		}
		if strings.Contains(summary, token) {
			score += 20
		}
		if strings.Contains(body, token) {
			score += 10
		}
	}
	return score
}

func docURL(doc content.SearchDoc) string {
	if doc.Type == "projects" {
		if doc.ParentSlug != "" {
			return "/projects/" + doc.ParentSlug + "/" + doc.Slug
		}
		return "/projects/" + doc.Slug
	}
	return "/post/" + doc.Slug
}
