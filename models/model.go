package models


type DocPage struct {
	SourceTitle        string      `json:"doc_title"`
	ContentMarkdown string       `json:"content_markdown"`
	DocSections     []DocSection `json:"doc_sections"`
	SourceUrl       string       `json:"source_url"`
}

type DocSection struct {
	SectionTitle           string `json:"section_title"`
	Order           int    `json:"order"`
	SourceTitle     string `json:"source_title"`
	SourceUrl       string `json:"source_url"`
	SectionAnchor   string `json:"section_anchor"`
	ContentMarkdown string `json:"content_markdown"`
}
