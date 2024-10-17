package indexer

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"text/template"

	"connectrpc.com/connect"
	extractorv1 "github.com/aiocean/shopify-doc-extractor/gen/extractor/v1"
	indexerv1 "github.com/aiocean/shopify-doc-extractor/gen/indexer/v1"
	"github.com/google/uuid"
	"github.com/qdrant/go-client/qdrant"
)

type IndexerServer struct{}

var getQdrantClient = sync.OnceValue(func() *qdrant.Client {
	port, err := strconv.Atoi(os.Getenv("QDRANT_PORT"))
	if err != nil {
		panic(err)
	}
	client, err := qdrant.NewClient(&qdrant.Config{
		Host:   os.Getenv("QDRANT_HOST"),
		Port:   port,
		APIKey: os.Getenv("QDRANT_API_KEY"),
		UseTLS: true,
	})

	if err != nil {
		panic(err)
	}
	return client
})

const shopifyDocsCollectionName = "shopify-doc"

func ensureCollectionExists(ctx context.Context, client *qdrant.Client) error {
	isCollectionExists, err := client.CollectionExists(ctx, shopifyDocsCollectionName)
	if err != nil {
		return err
	}

	if isCollectionExists {
		return nil
	}

	if err := client.CreateCollection(ctx, &qdrant.CreateCollection{
		CollectionName: shopifyDocsCollectionName,
		VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
			Size:     3072, // gemini models/text-embedding-004
			Distance: qdrant.Distance_Euclid,
		}),
	}); err != nil {
		return err
	}

	return nil
}

func completeDocContent(ctx context.Context, doc *extractorv1.DocPage) (string, error) {
	var contentTemplate = `---
Source title: {{.SourceTitle}}
Source URL: {{.SourceUrl}}
Sections:
{{range .DocSections}}
  - [{{.SectionTitle}}]({{.SourceUrl}}{{.SectionAnchor}})
{{end}}
---

{{.ContentMarkdown}}`
	tmpl, err := template.New("docTemplate").Parse(contentTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, doc); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

func compileSectionContent(ctx context.Context, section *extractorv1.DocSection) (string, error) {
	var contentTemplate = `---
Source Title: {{.SourceTitle}} / {{.SectionTitle}}
Source URL: {{.SourceUrl}}{{.SectionAnchor}}
---

{{.ContentMarkdown}}
`

	tmpl, err := template.New("sectionContent").Parse(contentTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, section); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

func (s *IndexerServer) Index(
	ctx context.Context,
	req *connect.Request[indexerv1.IndexRequest],
) (*connect.Response[indexerv1.IndexResponse], error) {

	qdrantClient := getQdrantClient()

	if err := ensureCollectionExists(ctx, qdrantClient); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}


	embeddingModel := getEmbeddingModel()

	indexingDocContent, err := completeDocContent(ctx, req.Msg.DocPage)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res, err := embeddingModel.EmbedContent(ctx, indexingDocContent)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	docUUID := uuid.NewSHA1(uuid.NameSpaceURL, []byte(req.Msg.DocPage.SourceUrl))

	// Prepare the point for upsert
	point := &qdrant.PointStruct{
		Id: &qdrant.PointId{
			PointIdOptions: &qdrant.PointId_Uuid{
				Uuid: docUUID.String(),
			},
		},
		Vectors: &qdrant.Vectors{
			VectorsOptions: &qdrant.Vectors_Vector{
				Vector: &qdrant.Vector{
					Data: res,
				},
			},
		},
		Payload: map[string]*qdrant.Value{
			"content":      qdrant.NewValueString(req.Msg.DocPage.ContentMarkdown),
			"source_title": qdrant.NewValueString(req.Msg.DocPage.SourceTitle),
			"source_url":   qdrant.NewValueString(req.Msg.DocPage.SourceUrl),
		},
	}

	// Create a WaitGroup to wait for all workers to finish
	var wg sync.WaitGroup
	// Create a channel to collect results and errors
	resultChan := make(chan error, len(req.Msg.DocPage.DocSections))

	// Worker function to process each section
	processSectionWorker := func(section *extractorv1.DocSection) {
		defer wg.Done()

		indexingContent, err := compileSectionContent(ctx, section)
		if err != nil {
			resultChan <- fmt.Errorf("failed to compile section content: %w", err)
			return
		}

		res, err := embeddingModel.EmbedContent(ctx, indexingContent)
		if err != nil {
			resultChan <- fmt.Errorf("failed to embed content: %w", err)
			return
		}

		sectionUUID := uuid.NewSHA1(uuid.NameSpaceURL, []byte(section.SourceUrl+section.SectionAnchor))
		point := &qdrant.PointStruct{
			Id: &qdrant.PointId{
				PointIdOptions: &qdrant.PointId_Uuid{
					Uuid: sectionUUID.String(),
				},
			},
			Vectors: &qdrant.Vectors{
				VectorsOptions: &qdrant.Vectors_Vector{
					Vector: &qdrant.Vector{
						Data: res,
					},
				},
			},
			Payload: map[string]*qdrant.Value{
				"content":      qdrant.NewValueString(section.ContentMarkdown),
				"source_title": qdrant.NewValueString(section.SourceTitle + "/" + section.SectionTitle),
				"source_url":   qdrant.NewValueString(section.SourceUrl + section.SectionAnchor),
				"source_order": qdrant.NewValueInt(int64(section.Order)),
			},
		}

		_, err = qdrantClient.Upsert(ctx, &qdrant.UpsertPoints{
			CollectionName: shopifyDocsCollectionName,
			Points:         []*qdrant.PointStruct{point},
		})
		if err != nil {
			resultChan <- fmt.Errorf("failed to upsert point: %w", err)
			return
		}

		resultChan <- nil
	}

	// Start a goroutine for each section
	for _, section := range req.Msg.DocPage.DocSections {
		wg.Add(1)
		go processSectionWorker(section)
	}

	// Close the result channel when all workers are done
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect and process results
	var errors []error
	for err := range resultChan {
		if err != nil {
			errors = append(errors, err)
		}
	}

	// Handle errors if any
	if len(errors) > 0 {
		errorMsg := "failed to process sections:"
		for _, err := range errors {
			errorMsg += "\n" + err.Error()
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf(errorMsg))
	}

	// Perform the upsert operation
	
	if _, err := qdrantClient.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: shopifyDocsCollectionName,
		Points:         []*qdrant.PointStruct{point},
	}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to upsert point: %w", err))
	}

	return connect.NewResponse(&indexerv1.IndexResponse{
		Success: true,
	}), nil
}
