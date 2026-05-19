package httpapp

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/danielscoffee/danielscoffee.me/internal/web"
)

func (s *Server) RegisterRoutes() http.Handler {
	mux := http.NewServeMux()

	fileServer := http.FileServer(http.FS(web.Files))
	mux.Handle("/assets/", fileServer)

	mux.HandleFunc("/", s.homeHandler)
	mux.HandleFunc("/blog", s.blogIndexHandler)
	mux.HandleFunc("/about", s.aboutHandler)
	mux.HandleFunc("/projects", s.projectsIndexHandler)
	mux.HandleFunc("/projects/", s.projectsTreeHandler)
	mux.HandleFunc("/project/", s.legacyProjectRedirectHandler)
	mux.HandleFunc("/post/", s.postDetailHandler)
	mux.HandleFunc("/tag/", s.tagIndexHandler)
	mux.HandleFunc("/search", s.searchHandler)
	mux.HandleFunc("/health", s.healthHandler)
	mux.HandleFunc("/rss.xml", s.rssHandler)
	mux.HandleFunc("/sitemap.xml", s.sitemapHandler)
	mux.HandleFunc("/robots.txt", s.robotsHandler)

	return s.requestLoggingMiddleware(s.securityHeadersMiddleware(mux))
}

func (s *Server) securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requestLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(lrw, r)

		s.logger.Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", lrw.statusCode).
			Int("bytes", lrw.bytes).
			Dur("duration", time.Since(start)).
			Str("remote_ip", clientIP(r)).
			Str("user_agent", r.UserAgent()).
			Msg("http_request")
	})
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	resp := map[string]string{"status": "up"}
	payload, err := json.Marshal(resp)
	if err != nil {
		s.logger.Error().Err(err).Msg("marshal health response failed")
		http.Error(w, "Failed to marshal health check response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(payload); err != nil {
		s.logger.Error().Err(err).Msg("write health response failed")
	}
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
	bytes      int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

func (lrw *loggingResponseWriter) Write(b []byte) (int, error) {
	n, err := lrw.ResponseWriter.Write(b)
	lrw.bytes += n
	return n, err
}

func clientIP(r *http.Request) string {
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}
