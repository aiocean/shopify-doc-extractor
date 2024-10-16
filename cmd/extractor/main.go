package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"
	"github.com/aiocean/shopify-doc-extractor/models"
	"github.com/gofiber/fiber/v2"
)

func ParseDocPage(pageUrl string) (*models.DocPage, error) {
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
	sourceUrl := strings.TrimPrefix(pageUrl, "https://shopify.dev")
	articleDocs := doc.Find(".article--docs")

	docSections := parseDocSections(articleDocs, title, sourceUrl)
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
		sectionList += fmt.Sprintf("- [%s](%s%s)\n", section.SectionTitle, sourceUrl, section.SectionAnchor)
	}
	contentMarkdown = fmt.Sprintf("%s\n\n%s", contentMarkdown, sectionList)

	return &models.DocPage{
		SourceTitle:        title,
		SourceUrl:       sourceUrl,
		DocSections:     docSections,
		ContentMarkdown: strings.TrimSpace(contentMarkdown),
	}, nil
}

func parseDocSections(articleDocs *goquery.Selection, docTitle, sourceUrl string) []models.DocSection {
	var docSections []models.DocSection

	articleDocs.Find(".feedback-section").Each(func(index int, s *goquery.Selection) {
		// Replace script elements with type="text/plain" with code blocks
		s.Find("script[type='text/plain']").Each(func(i int, script *goquery.Selection) {
			language := script.AttrOr("data-language", "")
			title := script.AttrOr("data-title", "")
			code := script.Text()
			
			var codeBlock string
			if title != "" {
				codeBlock = fmt.Sprintf("```%s title=\"%s\"\n%s\n```", language, title, code)
			} else {
				codeBlock = fmt.Sprintf("```%s\n%s\n```", language, code)
			}
			
			script.ReplaceWithHtml(codeBlock)
		})

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

		docSections = append(docSections, models.DocSection{
			Order:           index,
			SectionTitle:    title,
			SourceTitle:     docTitle,
			SourceUrl:       sourceUrl,
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

	if !strings.HasPrefix(url, "https://shopify.dev") {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "URL must start with https://shopify.dev"})
	}

	docPage, err := ParseDocPage(url)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to parse HTML"})
	}

	return c.JSON(docPage)
}

var (
	_mdConverter *md.Converter
)

func convertHtmlToMarkdown(html string) (string, error) {
	if _mdConverter == nil {
		_mdConverter = md.NewConverter("", true, nil)
	}

	return _mdConverter.ConvertString(html)
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
