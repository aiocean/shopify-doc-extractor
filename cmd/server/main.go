package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/aiocean/shopify-doc-extractor/gen/extractor/v1/extractorv1connect"
	"github.com/aiocean/shopify-doc-extractor/gen/indexer/v1/indexerv1connect"
	"github.com/aiocean/shopify-doc-extractor/implement/extractor"
	"github.com/aiocean/shopify-doc-extractor/implement/indexer"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func main() {
	mux := http.NewServeMux()

	extractorPath, extractorHandler := extractorv1connect.NewExtractorServiceHandler(&extractor.ExtractorServer{})
	mux.Handle(extractorPath, extractorHandler)

	indexerPath, indexerHandler := indexerv1connect.NewIndexerServiceHandler(&indexer.IndexerServer{})
	mux.Handle(indexerPath, indexerHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.ListenAndServe(
		fmt.Sprintf(":%s", port),
		h2c.NewHandler(mux, &http2.Server{}),
	)
}
