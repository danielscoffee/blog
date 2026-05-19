package content

type SearchDoc struct {
	Type       string
	Title      string
	Slug       string
	ParentSlug string
	Date       string
	Summary    string
	Tags       []string
	Body       string
}

func BuildSearchDocs(posts []Post, projects []Project) []SearchDoc {
	docs := make([]SearchDoc, 0, len(posts)+len(projects))
	docs = appendSearchDocs(docs, posts, "blog")
	docs = appendSearchDocs(docs, projects, "projects")
	for _, project := range projects {
		docs = appendSearchDocs(docs, project.SubPosts, "projects")
	}
	return docs
}
