# danielscoffee.me

Personal site and blog engine built with Go, templ, Tailwind CSS, and Neorg content files.

The app serves blog posts, project pages, nested project devlogs, search, feeds, sitemap, robots.txt, theme switching, and custom error pages from a small `net/http` server.

## Features

- Server-rendered pages with [templ](https://templ.guide/)
- Neorg (`.norg`) content with frontmatter-style metadata
- Blog posts from `content/posts/*.norg`
- Project pages from `content/projects/<project>/index.norg`
- Nested project subposts/devlogs from `content/projects/<project>/*.norg`
- In-memory post/project stores loaded at startup
- Tag pages for blog posts
- Search endpoint and Ctrl+K search modal
- RSS feed, sitemap, and robots.txt
- Light/dark/system theme toggle stored in `localStorage`
- Custom templ-rendered 404 and 500 pages
- Structured request logging with zerolog
- Docker and GoReleaser support

## Architecture

```text
cmd/api              HTTP binary entrypoint and graceful shutdown
internal/app         Runtime bootstrap: env, logging, content loading, server wiring
internal/content     Neorg parser, loaders, domain models, stores, search docs
internal/http        Server, routes, handlers, feeds, search, middleware
internal/web         templ views, embedded assets, Tailwind styles
content              Site content authored as .norg files
templates            Example content templates
```

High-level flow:

1. `cmd/api/main.go` calls `app.NewRuntime()`.
2. `internal/app` reads environment, creates logger, loads posts/projects/about page from `content/`, builds search docs, and constructs the HTTP server.
3. `internal/http` registers routes with `net/http.ServeMux` and renders templ components from `internal/web`.
4. `internal/content` parses `.norg` files into typed Go models and pre-rendered HTML.
5. Static CSS/JS assets are embedded with `embed.FS` and served from `/assets/`.

## Content Format

Content uses Neorg files with a metadata block:

```norg
@document.meta
title: My Post
slug: my-post
date: 2026-05-20
summary: Short description
tags: [go, web]
draft: false
@end

* Heading
Body content.
```

Supported body features include headings, paragraphs, ordered/unordered lists, task states, tables, blockquotes, definition lists, inline formatting, code blocks with Chroma highlighting, and CDN-hosted images.

Task states:

```norg
*** TODO write post
*** DOING edit draft
*** DONE publish
*** CANCELLED old idea
```

## Routes

- `/` redirects to `/blog`
- `/blog` lists blog posts
- `/post/{slug}` shows a blog post
- `/tag/{tag}` lists posts by tag
- `/about` shows the about page
- `/projects` lists projects
- `/projects/{slug}` shows a project
- `/projects/{slug}/{subpost}` shows a project devlog/subpost
- `/project/{slug}` redirects to `/projects/{slug}`
- `/search?q=...` returns JSON search results
- `/rss.xml` returns RSS for blog posts
- `/sitemap.xml` returns sitemap XML
- `/robots.txt` returns crawler policy
- `/health` returns JSON health status

Search filters:

```text
blog go
projects search
```

Bare `blog` or `projects` returns all items for that type.

## Getting Started

### Prerequisites

- Go 1.24+
- `make`
- `templ` CLI (installed automatically by `make generate` if missing)
- Tailwind standalone binary (downloaded automatically by `make generate` if missing)

### Configuration

Copy the example environment file:

```bash
cp .env.example .env
```

Environment variables:

| Variable | Default | Description |
| --- | --- | --- |
| `PORT` | `8080` | HTTP port |
| `SITE_URL` | `http://localhost:<PORT>` | Absolute base URL for feeds/sitemap |
| `APP_ENV` | `local` in `.env.example` | Runtime environment label for compose/deploy |
| `LOG_FORMAT` | `json` | `json` or `text` |
| `LOG_LEVEL` | `info` | zerolog level such as `debug`, `info`, `warn`, `error` |

### Run Locally

```bash
make run
```

The site runs at `http://localhost:8080` by default.

### Build

```bash
make build
./main
```

### Generate Assets

```bash
make generate
```

This runs:

- `templ generate -path .`
- Tailwind build from `internal/web/styles/input.css` to `internal/web/assets/css/output.css`

## Development

Useful commands:

```bash
make run          # generate assets and run with go run
make build        # generate assets and build ./main
make test         # generate assets and run go test ./... -v
make watch        # run with air, installing it if missing
make clean        # remove ./main
make docker-run   # run Docker Compose
make docker-down  # stop Docker Compose
```

## Testing

```bash
make test
```

Focused packages:

```bash
go test ./internal/content -v
go test ./internal/http -v
go test ./internal/web -v
go test ./internal/logging -v
```

Coverage areas include:

- Neorg parsing and validation
- Content loading, sorting, draft skipping, project subposts
- Store lookup behavior
- HTTP routes and status codes
- Search filtering/ranking and nested project URLs
- Feed endpoints
- Logging configuration
- Required style hooks

## Docker

```bash
make docker-run
make docker-down
```

The Docker build generates templ output and Tailwind CSS before compiling the Go binary.

## Releases

Tagged releases matching `v*.*.*` run GoReleaser via GitHub Actions.

```bash
git tag v0.1.0
git push origin v0.1.0
```

GoReleaser builds Linux, macOS, and Windows binaries for amd64 and arm64.

## Writing Content

### Blog post

Create `content/posts/<slug>.norg`:

```norg
@document.meta
title: My Post
slug: my-post
date: 2026-05-20
summary: Short summary
tags: [go, personal]
draft: false
@end

* My Post
Body content.
```

### Project

Create `content/projects/<project>/index.norg`:

```norg
@document.meta
title: My Project
slug: my-project
date: 2026-05-20
summary: What this project does
tags: [go, web]
draft: false
@end

* Overview
Project overview.
```

### Project subpost/devlog

Create another `.norg` file in the same project folder:

```norg
@document.meta
title: Devlog 1
slug: devlog-1
date: 2026-05-21
summary: What changed
tags: [devlog]
draft: false
@end

* Devlog 1
Build notes.
```

Draft project indexes are skipped with all subposts. Draft subposts are skipped individually.

## Security Notes

- Security headers include `X-Content-Type-Options`, `Referrer-Policy`, and `X-Frame-Options`.
- Image rendering only accepts HTTPS image URLs whose host contains `cdn`.
- Parsed content HTML is rendered with `templ.Raw`, so parser output must remain trusted and tested.

## TODO / Open Questions

- RSS and sitemap currently cover blog posts, not project pages/subposts.
- Project tag links point to `/projects?tag=...`, but project index filtering is not implemented yet.
- README assumes generated `*_templ.go` files may be regenerated during build; keep CI/build steps aligned with repository tracking policy.
- `content/posts/blogcreation.norg` exists but has almost no body content.

## License

No license file is currently present.
