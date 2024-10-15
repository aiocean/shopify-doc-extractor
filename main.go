package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"
	"github.com/gofiber/fiber/v2"
)

type DocPage struct {
	DocTitle        string       `json:"doc_title"`
	ContentMarkdown string       `json:"content_markdown"`
	ResourceUrls    []string     `json:"resource_urls"`
	DocSections     []DocSection `json:"doc_sections"`
	RelativeUrl     string       `json:"relative_url"`
}

type DocSection struct {
	Title           string `json:"title"`
	Order           int    `json:"order"`
	SourceTitle     string `json:"source_title"`
	SourceUrl       string `json:"source_url"`
	SectionAnchor   string `json:"section_anchor"`
	ContentMarkdown string `json:"content_markdown"`
}

func ParseDocPage(pageUrl string) (*DocPage, error) {
	resp, err := http.Get(pageUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch page: %w", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	title := strings.TrimSpace(doc.Find(".article-title").Text())
	relativeUrl := strings.TrimPrefix(pageUrl, "https://shopify.dev")
	articleDocs := doc.Find(".article--docs")

	docSections := parseDocSections(articleDocs, title, relativeUrl)
	articleDocs.Find("#FeedbackFloatingAnchor").Remove()

	contentHtml, err := articleDocs.Html()
	if err != nil {
		return nil, fmt.Errorf("failed to get HTML content: %w", err)
	}

	contentMarkdown, err := convertHtmlToMarkdown(contentHtml)
	if err != nil {
		return nil, fmt.Errorf("failed to convert HTML to Markdown: %w", err)
	}

	// add section list to the content markdown
	sectionList := "## Sections\n\n"
	for _, section := range docSections {
		sectionList += fmt.Sprintf("- [%s](#%s)\n", section.Title, section.SectionAnchor)
	}
	contentMarkdown = fmt.Sprintf("%s\n\n%s", contentMarkdown, sectionList)

	return &DocPage{
		DocTitle:        title,
		RelativeUrl:     relativeUrl,
		DocSections:     docSections,
		ContentMarkdown: strings.TrimSpace(contentMarkdown),
	}, nil
}

func parseDocSections(articleDocs *goquery.Selection, docTitle, relativeUrl string) []DocSection {
	var docSections []DocSection

	articleDocs.Find(".feedback-section").Each(func(index int, s *goquery.Selection) {
		contentHtml, err := s.Html()
		if err != nil {
			log.Printf("Error getting HTML for doc section: %v", err)
			return
		}

		contentMarkdown, err := convertHtmlToMarkdown(contentHtml)
		if err != nil {
			log.Printf("Error converting HTML to Markdown for doc section: %v", err)
			return
		}

		sectionAnchor := s.Find(".heading-wrapper > .article-anchor-link").AttrOr("href", "")

		title := s.Find(".heading-wrapper > h2").Text()

		docSections = append(docSections, DocSection{
			Order:           index,
			Title:           title,
			SourceTitle:     docTitle,
			SourceUrl:       relativeUrl,
			SectionAnchor:   sectionAnchor,
			ContentMarkdown: contentMarkdown,
		})

		s.Remove()
	})

	return docSections
}

func extractHandler(c *fiber.Ctx) error {
	url := c.Query("url")
	if url == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "URL parameter is required"})
	}

	docPage, err := ParseDocPage(url)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to parse HTML"})
	}

	return c.JSON(docPage)
}

var (
	mdConverter *md.Converter
)

func initMdConverter() {
	mdConverter = md.NewConverter("", true, nil)
}

func convertHtmlToMarkdown(html string) (string, error) {
	if mdConverter == nil {
		initMdConverter()
	}
	return mdConverter.ConvertString(html)
}

func main() {
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		},
	})

	app.Get("/extract", extractHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	if err := app.Listen(fmt.Sprintf(":%s", port)); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
