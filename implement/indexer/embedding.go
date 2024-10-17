package indexer

import (
	"context"
	"os"
	"sync"

	"github.com/google/generative-ai-go/genai"
	openai "github.com/sashabaranov/go-openai"
	"google.golang.org/api/option"
)


var getGeminiClient = sync.OnceValue(func() *genai.Client {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(os.Getenv("GEMINI_API_KEY")))
	if err != nil {
		panic(err)
	}
	return client
})

var getOpenAiClient = sync.OnceValue(func() *openai.Client {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		panic("OPENAI_API_KEY environment variable is not set")
	}
	client := openai.NewClient(apiKey)
	return client
})

type EmbeddingModel interface {
	EmbedContent(ctx context.Context, content string) ([]float32, error)
}

type OpenAIEmbeddingModel struct {
	client *openai.Client	
}

func NewOpenAIEmbeddingModel(client *openai.Client) EmbeddingModel {
	return &OpenAIEmbeddingModel{client: client}
}

func (m *OpenAIEmbeddingModel) EmbedContent(ctx context.Context, content string) ([]float32, error) {
	resp, err := m.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Input: []string{content},
		Model: openai.LargeEmbedding3,
	})
	if err != nil {
		return nil, err
	}

	return resp.Data[0].Embedding, nil
}

var getEmbeddingModel = sync.OnceValue(func() EmbeddingModel {
	openaiClient := getOpenAiClient()
	return NewOpenAIEmbeddingModel(openaiClient)
})
