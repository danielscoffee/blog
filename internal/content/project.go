package content

import "html/template"

type Project struct {
	Published

	BodyMD   string
	BodyHTML template.HTML

	SubPosts []ProjectSubPost
}

type ProjectSubPost struct {
	Published

	ParentSlug string

	BodyMD   string
	BodyHTML template.HTML
}

func (p Project) searchDoc(docType string) SearchDoc {
	return SearchDoc{
		Type:    docType,
		Title:   p.Title,
		Slug:    p.Slug,
		Date:    p.Date,
		Summary: p.Summary,
		Tags:    append([]string(nil), p.Tags...),
		Body:    p.BodyMD,
	}
}

func (p ProjectSubPost) searchDoc(docType string) SearchDoc {
	return SearchDoc{
		Type:       docType,
		Title:      p.Title,
		Slug:       p.Slug,
		ParentSlug: p.ParentSlug,
		Date:       p.Date,
		Summary:    p.Summary,
		Tags:       append([]string(nil), p.Tags...),
		Body:       p.BodyMD,
	}
}
