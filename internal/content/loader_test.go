package content

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writePost(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write post %s: %v", name, err)
	}
}

func TestLoadPosts_SortsNewestFirstAndSkipsDrafts(t *testing.T) {
	dir := t.TempDir()

	writePost(t, dir, "one.norg", `@document.meta
title: One
slug: one
date: 2026-01-01
summary: one summary
tags: [go]
draft: false
@end
* One
Body one.
`)
	writePost(t, dir, "two.norg", `@document.meta
title: Two
slug: two
date: 2026-03-01
summary: two summary
tags: [personal]
draft: false
@end
* Two
Body two.
`)
	writePost(t, dir, "draft.norg", `@document.meta
title: Draft
slug: draft
date: 2026-04-01
summary: draft summary
tags: [draft]
draft: true
@end
* Draft
`)

	posts, err := LoadPosts(dir)
	if err != nil {
		t.Fatalf("LoadPosts returned error: %v", err)
	}

	if len(posts) != 2 {
		t.Fatalf("expected 2 published posts, got %d", len(posts))
	}

	if posts[0].Slug != "two" {
		t.Fatalf("expected newest post first, got slug %q", posts[0].Slug)
	}

	if posts[1].Slug != "one" {
		t.Fatalf("expected oldest post second, got slug %q", posts[1].Slug)
	}

	if posts[0].BodyHTML == "" {
		t.Fatalf("expected rendered html body for post %q", posts[0].Slug)
	}
}

func TestLoadPosts_RequiresTitleSlugDate(t *testing.T) {
	dir := t.TempDir()

	writePost(t, dir, "bad.norg", `@document.meta
title: Missing slug
date: 2026-01-01
summary: no slug
@end
* Bad
`)

	_, err := LoadPosts(dir)
	if err == nil {
		t.Fatal("expected error for missing required frontmatter")
	}
}

func TestLoadPosts_IgnoresMarkdownFiles(t *testing.T) {
	dir := t.TempDir()

	writePost(t, dir, "legacy.md", `---
title: Legacy
slug: legacy
date: 2026-01-01
summary: should be ignored
---
# Legacy
`)

	posts, err := LoadPosts(dir)
	if err != nil {
		t.Fatalf("LoadPosts returned error: %v", err)
	}
	if len(posts) != 0 {
		t.Fatalf("expected 0 posts from markdown files, got %d", len(posts))
	}
}

func TestLoadPosts_SupportsNeorgFrontMatter(t *testing.T) {
	dir := t.TempDir()

	writePost(t, dir, "post.norg", `@document.meta
title: Norg Post
slug: norg-post
date: 2026-05-01
summary: from neorg frontmatter
tags:
  - norg
  - notes
draft: false
@end
# Norg heading
Body.
`)

	posts, err := LoadPosts(dir)
	if err != nil {
		t.Fatalf("LoadPosts returned error: %v", err)
	}

	if len(posts) != 1 {
		t.Fatalf("expected 1 post, got %d", len(posts))
	}

	if posts[0].Slug != "norg-post" {
		t.Fatalf("expected slug norg-post, got %q", posts[0].Slug)
	}

	if len(posts[0].Tags) != 2 || posts[0].Tags[0] != "norg" || posts[0].Tags[1] != "notes" {
		t.Fatalf("expected tags [norg notes], got %#v", posts[0].Tags)
	}
}

func TestLoadPosts_SupportsNeorgTaskRendering(t *testing.T) {
	dir := t.TempDir()

	writePost(t, dir, "task.norg", `@document.meta
title: Tasks
slug: tasks
date: 2026-05-01
@end
*** TODO first task
`)

	posts, err := LoadPosts(dir)
	if err != nil {
		t.Fatalf("LoadPosts error: %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("expected 1 post, got %d", len(posts))
	}
	if !strings.Contains(string(posts[0].BodyHTML), `data-task-state="todo"`) {
		t.Fatalf("expected checklist html, got %s", posts[0].BodyHTML)
	}
}

func writeProjectFile(t *testing.T, root, project, name, body string) {
	t.Helper()
	dir := filepath.Join(root, project)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write %s/%s: %v", dir, name, err)
	}
}

func TestLoadProjects_SortsNewestFirst(t *testing.T) {
	dir := t.TempDir()

	writeProjectFile(t, dir, "one", "index.norg", `@document.meta
title: One
slug: one
date: 2026-01-01
summary: one
@end
body`)
	writeProjectFile(t, dir, "two", "index.norg", `@document.meta
title: Two
slug: two
date: 2026-02-01
summary: two
@end
body`)

	projects, err := LoadProjects(dir)
	if err != nil {
		t.Fatalf("LoadProjects returned error: %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(projects))
	}
	if projects[0].Slug != "two" {
		t.Fatalf("expected newest project first, got %q", projects[0].Slug)
	}
}

func TestLoadProjects_LoadsSubPostsSortedAndSkipsDrafts(t *testing.T) {
	dir := t.TempDir()

	writeProjectFile(t, dir, "blog-project", "index.norg", `@document.meta
title: Blog Project
slug: blog-project
date: 2026-05-01
summary: overview
@end
body`)
	writeProjectFile(t, dir, "blog-project", "rebuild.norg", `@document.meta
title: Rebuild
slug: rebuild
date: 2026-05-10
summary: how
@end
body`)
	writeProjectFile(t, dir, "blog-project", "devlog.norg", `@document.meta
title: Devlog
slug: devlog
date: 2026-05-19
summary: devlog
@end
body`)
	writeProjectFile(t, dir, "blog-project", "secret.norg", `@document.meta
title: Secret
slug: secret
date: 2026-05-20
draft: true
@end
body`)

	projects, err := LoadProjects(dir)
	if err != nil {
		t.Fatalf("LoadProjects returned error: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	subs := projects[0].SubPosts
	if len(subs) != 2 {
		t.Fatalf("expected 2 subposts, got %d", len(subs))
	}
	if subs[0].Slug != "devlog" {
		t.Fatalf("expected newest subpost first, got %q", subs[0].Slug)
	}
	if subs[0].ParentSlug != "blog-project" {
		t.Fatalf("expected parent slug blog-project, got %q", subs[0].ParentSlug)
	}
}

func TestLoadProjects_SkipsDraftIndex(t *testing.T) {
	dir := t.TempDir()

	writeProjectFile(t, dir, "hidden", "index.norg", `@document.meta
title: Hidden
slug: hidden
date: 2026-05-01
draft: true
@end
body`)

	projects, err := LoadProjects(dir)
	if err != nil {
		t.Fatalf("LoadProjects returned error: %v", err)
	}
	if len(projects) != 0 {
		t.Fatalf("expected 0 projects, got %d", len(projects))
	}
}

func TestLoadProjects_IgnoresFoldersWithoutIndex(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "empty"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	projects, err := LoadProjects(dir)
	if err != nil {
		t.Fatalf("LoadProjects returned error: %v", err)
	}
	if len(projects) != 0 {
		t.Fatalf("expected 0 projects, got %d", len(projects))
	}
}

func TestLoadPage_LoadsSinglePage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "about.norg")
	writePost(t, dir, "about.norg", `@document.meta
title: About
slug: about
date: 2026-05-01
summary: about me
@end
* About`)

	page, err := LoadPage(path)
	if err != nil {
		t.Fatalf("LoadPage returned error: %v", err)
	}
	if page.Slug != "about" {
		t.Fatalf("expected slug about, got %q", page.Slug)
	}
}

func TestStore_BySlugAndByTag(t *testing.T) {
	store := NewStore([]Post{
		{Published: Published{Title: "One", Slug: "one", Date: "2026-01-01", Tags: []string{"go", "personal"}}},
		{Published: Published{Title: "Two", Slug: "two", Date: "2026-02-01", Tags: []string{"go"}}},
	})

	if _, ok := store.BySlug("one"); !ok {
		t.Fatal("expected slug lookup for one to succeed")
	}

	if _, ok := store.BySlug("missing"); ok {
		t.Fatal("expected slug lookup for missing to fail")
	}

	posts := store.ByTag("personal")
	if len(posts) != 1 || posts[0].Slug != "one" {
		t.Fatalf("expected personal tag to return one post 'one', got %#v", posts)
	}
}
