package httpapp

import (
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/danielscoffee/danielscoffee.me/internal/content"
)

func testBlogServer() *Server {
	posts := []content.Post{
		{
			Published: content.Published{
				Title:   "Hello World",
				Slug:    "hello-world",
				Date:    "2026-04-26",
				Summary: "Ship the first post",
				Tags:    []string{"go", "personal"},
			},
			BodyMD:   "# Hello",
			BodyHTML: template.HTML(`<h1>Hello</h1>`),
		},
		{
			Published: content.Published{
				Title:   "Now",
				Slug:    "now",
				Date:    "2026-04-20",
				Summary: "What I'm doing now",
				Tags:    []string{"now"},
			},
			BodyMD:   "# Now",
			BodyHTML: template.HTML(`<h1>Now</h1>`),
		},
	}

	projects := []content.Project{
		{
			Published: content.Published{
				Title:   "Side Project",
				Slug:    "side-project",
				Date:    "2026-05-01",
				Summary: "Small app",
				Tags:    []string{"go", "web"},
			},
			BodyMD:   "overview",
			BodyHTML: template.HTML(`<p>overview</p>`),
			SubPosts: []content.ProjectSubPost{
				{
					Published: content.Published{
						Title:   "Rebuild Notes",
						Slug:    "rebuild",
						Date:    "2026-05-10",
						Summary: "how I rebuilt it",
					},
					ParentSlug: "side-project",
					BodyMD:     "rebuild body",
					BodyHTML:   template.HTML(`<p>rebuild body</p>`),
				},
			},
		},
	}

	about := content.Page{
		Title:    "About Me",
		Slug:     "about",
		Date:     "2026-05-01",
		Summary:  "About",
		BodyMD:   "about text",
		BodyHTML: template.HTML(`<p>about text</p>`),
	}

	return &Server{
		port:          8080,
		contentStore:  content.NewStore(posts),
		projectStore:  content.NewProjectStore(projects),
		aboutPage:     about,
		siteURL:       "https://example.com",
		logger:        zerolog.New(io.Discard),
		searchIndexer: NewSearchIndexer(content.BuildSearchDocs(posts, projects)),
	}
}

func TestBlogRoutes(t *testing.T) {
	s := testBlogServer()
	h := s.RegisterRoutes()

	cases := []struct {
		path       string
		statusCode int
		contains   string
	}{
		{path: "/", statusCode: http.StatusFound, contains: ""},
		{path: "/blog", statusCode: http.StatusOK, contains: "Blog"},
		{path: "/about", statusCode: http.StatusOK, contains: "About Me"},
		{path: "/projects", statusCode: http.StatusOK, contains: "Projects"},
		{path: "/projects/side-project", statusCode: http.StatusOK, contains: "Side Project"},
		{path: "/projects/side-project", statusCode: http.StatusOK, contains: "project-subposts-grid"},
		{path: "/projects/side-project/rebuild", statusCode: http.StatusOK, contains: "Rebuild Notes"},
		{path: "/projects/side-project/missing", statusCode: http.StatusNotFound, contains: "404 page not found"},
		{path: "/projects/missing", statusCode: http.StatusNotFound, contains: "404 page not found"},
		{path: "/project/side-project", statusCode: http.StatusMovedPermanently, contains: ""},
		{path: "/post/hello-world", statusCode: http.StatusOK, contains: "<article"},
		{path: "/tag/go", statusCode: http.StatusOK, contains: "Tagged with"},
		{path: "/post/missing", statusCode: http.StatusNotFound, contains: "404 page not found"},
	}

	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)

		if w.Code != tc.statusCode {
			t.Fatalf("path %s expected status %d got %d", tc.path, tc.statusCode, w.Code)
		}

		if tc.contains != "" && !strings.Contains(w.Body.String(), tc.contains) {
			t.Fatalf("path %s expected body to contain %q; got %q", tc.path, tc.contains, w.Body.String())
		}
		if tc.path == "/" {
			if got := w.Header().Get("Location"); got != "/blog" {
				t.Fatalf("expected redirect to /blog, got %q", got)
			}
		}
	}
}

func TestBaseTemplateIncludesThemeControls(t *testing.T) {
	s := testBlogServer()
	h := s.RegisterRoutes()

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/blog", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	assertContainsAll(t, w.Body.String(), []string{
		`data-theme="system"`,
		`id="theme-toggle"`,
		`/assets/js/theme-init.js`,
		`/assets/js/theme-toggle.js`,
		`/assets/js/search.js`,
		`theme-preference`,
		`search-modal`,
	})
}

func TestThemeAssetsAreServed(t *testing.T) {
	s := testBlogServer()
	h := s.RegisterRoutes()

	cases := []struct {
		path     string
		contains string
	}{
		{path: "/assets/js/theme-init.js", contains: "theme-preference"},
		{path: "/assets/js/theme-toggle.js", contains: "Theme:"},
		{path: "/assets/js/search.js", contains: "Ctrl+K"},
	}

	for _, tc := range cases {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.path, nil))

		if w.Code != http.StatusOK {
			t.Fatalf("%s expected status 200, got %d", tc.path, w.Code)
		}
		if !strings.Contains(w.Body.String(), tc.contains) {
			t.Fatalf("%s expected body to contain %q", tc.path, tc.contains)
		}
	}
}

func TestSearchRoute(t *testing.T) {
	s := testBlogServer()
	h := s.RegisterRoutes()

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/search?q=projects+side", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"type":"projects"`) {
		t.Fatalf("expected projects result, got %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), `"type":"blog"`) {
		t.Fatalf("expected filtered results only, got %s", w.Body.String())
	}
}

func TestPagesExposeStyleHooks(t *testing.T) {
	s := testBlogServer()
	h := s.RegisterRoutes()

	cases := []struct {
		path    string
		markers []string
	}{
		{
			path:    "/blog",
			markers: []string{"page-title", "page-subtitle", "section-title", "post-list", "post-item", "post-link", "tag-chip"},
		},
		{
			path:    "/blog",
			markers: []string{"page-title", "post-list", "post-meta-row"},
		},
		{
			path:    "/post/hello-world",
			markers: []string{"post-prose", "post-header", "post-title", "post-date"},
		},
		{
			path:    "/projects",
			markers: []string{"project-list", "project-link"},
		},
	}

	for _, tc := range cases {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.path, nil))

		if w.Code != http.StatusOK {
			t.Fatalf("%s expected status 200, got %d", tc.path, w.Code)
		}
		assertContainsAll(t, w.Body.String(), tc.markers)
	}
}

func assertContainsAll(t *testing.T, body string, markers []string) {
	t.Helper()
	for _, marker := range markers {
		if !strings.Contains(body, marker) {
			t.Fatalf("expected body to contain %q", marker)
		}
	}
}

func TestHealthRoute(t *testing.T) {
	s := testBlogServer()
	h := s.RegisterRoutes()

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	if got := w.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Fatalf("expected content type application/json, got %q", got)
	}

	if !strings.Contains(w.Body.String(), `"status":"up"`) {
		t.Fatalf("expected health body to contain status up, got %q", w.Body.String())
	}
}
