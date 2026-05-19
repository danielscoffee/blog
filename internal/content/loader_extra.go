package content

import (
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const projectIndexFile = "index.norg"

// LoadProjects walks dir for folder-based projects:
//
//	dir/<project-slug>/index.norg     (required, defines the project)
//	dir/<project-slug>/<sub>.norg     (optional, subposts within the project)
//
// Draft projects (index draft: true) are skipped wholesale.
// Draft subposts are skipped individually.
func LoadProjects(dir string) ([]Project, error) {
	folders, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Project{}, nil
		}
		return nil, fmt.Errorf("read projects dir: %w", err)
	}

	projects := make([]Project, 0, len(folders))
	seenProject := make(map[string]struct{})

	for _, entry := range folders {
		if !entry.IsDir() {
			continue
		}
		project, ok, err := loadProjectFolder(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		if _, dup := seenProject[project.Slug]; dup {
			return nil, fmt.Errorf("duplicate project slug %q", project.Slug)
		}
		seenProject[project.Slug] = struct{}{}
		projects = append(projects, project)
	}

	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Date > projects[j].Date
	})

	return projects, nil
}

func loadProjectFolder(folder string) (Project, bool, error) {
	indexPath := filepath.Join(folder, projectIndexFile)
	indexEntry, err := loadContentFile(indexPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Project{}, false, nil
		}
		return Project{}, false, err
	}
	if indexEntry.meta.Draft {
		return Project{}, false, nil
	}
	if err := validateFrontMatter(indexEntry.meta); err != nil {
		return Project{}, false, fmt.Errorf("project %s: %w", folder, err)
	}

	project := Project{
		Published: Published{
			Title:   indexEntry.meta.Title,
			Slug:    indexEntry.meta.Slug,
			Date:    indexEntry.meta.Date,
			Summary: indexEntry.meta.Summary,
			Tags:    indexEntry.meta.Tags,
			Draft:   indexEntry.meta.Draft,
		},
		BodyMD:   indexEntry.body,
		BodyHTML: template.HTML(indexEntry.htmlBody),
	}

	subPosts, err := loadProjectSubPosts(folder, project.Slug)
	if err != nil {
		return Project{}, false, err
	}
	project.SubPosts = subPosts

	return project, true, nil
}

func loadProjectSubPosts(folder, projectSlug string) ([]ProjectSubPost, error) {
	files, err := filepath.Glob(filepath.Join(folder, "*.norg"))
	if err != nil {
		return nil, err
	}

	subPosts := make([]ProjectSubPost, 0, len(files))
	seenSub := make(map[string]struct{})

	for _, file := range files {
		if strings.EqualFold(filepath.Base(file), projectIndexFile) {
			continue
		}
		entry, err := loadContentFile(file)
		if err != nil {
			return nil, err
		}
		if entry.meta.Draft {
			continue
		}
		if err := validateFrontMatter(entry.meta); err != nil {
			return nil, fmt.Errorf("subpost %s: %w", file, err)
		}
		if _, dup := seenSub[entry.meta.Slug]; dup {
			return nil, fmt.Errorf("duplicate subpost slug %q in project %q", entry.meta.Slug, projectSlug)
		}
		seenSub[entry.meta.Slug] = struct{}{}

		subPosts = append(subPosts, ProjectSubPost{
			Published: Published{
				Title:   entry.meta.Title,
				Slug:    entry.meta.Slug,
				Date:    entry.meta.Date,
				Summary: entry.meta.Summary,
				Tags:    entry.meta.Tags,
				Draft:   entry.meta.Draft,
			},
			ParentSlug: projectSlug,
			BodyMD:     entry.body,
			BodyHTML:   template.HTML(entry.htmlBody),
		})
	}

	sort.Slice(subPosts, func(i, j int) bool {
		return subPosts[i].Date > subPosts[j].Date
	})

	return subPosts, nil
}

// LoadPage loads a single content file as a Page (used for /about etc).
func LoadPage(path string) (Page, error) {
	entry, err := loadContentFile(path)
	if err != nil {
		return Page{}, err
	}

	if entry.meta.Draft {
		return Page{}, fmt.Errorf("page %s is marked draft", path)
	}

	return Page{
		Title:    entry.meta.Title,
		Slug:     entry.meta.Slug,
		Date:     entry.meta.Date,
		Summary:  entry.meta.Summary,
		BodyMD:   entry.body,
		BodyHTML: template.HTML(entry.htmlBody),
	}, nil
}

func loadContentFile(path string) (contentEntry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return contentEntry{}, fmt.Errorf("read %s: %w", path, err)
	}

	meta, body, preRenderedHTML, err := splitFrontMatter(string(raw), filepath.Ext(path))
	if err != nil {
		return contentEntry{}, fmt.Errorf("parse %s: %w", path, err)
	}

	if preRenderedHTML == "" {
		return contentEntry{}, fmt.Errorf("unsupported content format %q", filepath.Ext(path))
	}

	return contentEntry{meta: meta, body: body, htmlBody: preRenderedHTML}, nil
}

type contentEntry struct {
	meta     frontMatter
	body     string
	htmlBody string
}

func loadEntries(dir string) ([]contentEntry, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.norg"))
	if err != nil {
		return nil, err
	}

	entries := make([]contentEntry, 0, len(files))
	for _, file := range files {
		entry, err := loadContentFile(file)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}
