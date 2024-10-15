package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseDocPage(t *testing.T) {
	// Read HTML content from file
	htmlContent, err := os.ReadFile("testdata/discount.html")
	assert.NoError(t, err, "Failed to read test HTML file")

	// Create a mock HTTP server that serves the file content
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write(htmlContent)
	}))
	defer server.Close()

	// Test case
	testURL := server.URL
	expectedTitle := "About discounts" // Update this to match the actual title in your HTML file

	// Call the function
	docPage, err := ParseDocPage(testURL)

	// Assert results
	assert.NoError(t, err)
	assert.NotNil(t, docPage)
	assert.Equal(t, expectedTitle, docPage.DocTitle)
	assert.NotEmpty(t, docPage.ContentHtml)

	// len of doc sections should be 5
	assert.Equal(t, 5, len(docPage.DocSections))
	// the first doc section should be "Discount methods"
	assert.Equal(t, "Build with the GraphQL Admin API", docPage.DocSections[0].Title)

	assert.Equal(t, docPage.ContentHtml, "<div class=\"article-title-container \">\n  <h1 class=\"article-title\">\n    About discounts\n    \n  </h1>\n  \n</div>\n<p>Discount apps integrate with the Shopify admin to provide discount types for app users. This guide introduces the ways that you can extend your app code into Shopify checkout and customize the discount experience.</p>\n\n<ul>\n<li>Use the <a href=\"#build-with-the-graphql-admin-api\">GraphQL Admin API</a> to create and manage <a rel=\"external noreferrer noopener\" target=\"_blank\" href=\"https://help.shopify.com/manual/discounts/discount-types\">discounts that are native to Shopify</a>.</li>\n<li>Use <a href=\"#build-with-shopify-functions\">Shopify Functions</a> to extend your app code into Shopify checkout and create discount functionality that isn&#39;t offered out of the box with Shopify.</li>\n</ul>")
}
