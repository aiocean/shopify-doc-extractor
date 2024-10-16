package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"connectrpc.com/connect"
	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"
	extractorv1 "github.com/aiocean/shopify-doc-extractor/gen/extractor/v1"
	"github.com/aiocean/shopify-doc-extractor/gen/extractor/v1/extractorv1connect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func ParseDocPage(pageUrl string) (*extractorv1.DocPage, error) {
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

	return &extractorv1.DocPage{
		SourceTitle:        title,
		SourceUrl:       sourceUrl,
		DocSections:     docSections,
		ContentMarkdown: strings.TrimSpace(contentMarkdown),
	}, nil
}

func parseDocSections(articleDocs *goquery.Selection, docTitle, sourceUrl string) []*extractorv1.DocSection {
	var docSections []*extractorv1.DocSection

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

		docSections = append(docSections, &extractorv1.DocSection{
			Order:           int32(index),
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

var (
	_mdConverter *md.Converter
)

func convertHtmlToMarkdown(html string) (string, error) {
	if _mdConverter == nil {
		_mdConverter = md.NewConverter("", true, nil)
	}

	return _mdConverter.ConvertString(html)
}


type ExtractorServer struct{}

func (s *ExtractorServer) Extract(
	ctx context.Context,
	req *connect.Request[extractorv1.ExtractRequest],
) (*connect.Response[extractorv1.ExtractResponse], error) {
	docPage, err := ParseDocPage(req.Msg.Url)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to parse HTML: %w", err))
	}

	res := connect.NewResponse(&extractorv1.ExtractResponse{
		DocPage: docPage,
	})

	return res, nil
}

func main() {
	extractor := &ExtractorServer{}
	mux := http.NewServeMux()
	path, handler := extractorv1connect.NewExtractorServiceHandler(extractor)
	mux.Handle(path, handler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.ListenAndServe(
		fmt.Sprintf(":%s", port),
		h2c.NewHandler(mux, &http2.Server{}),
	)
}