# Shopify Dev Documentation Extractor

This project is a Go-based web service that extracts and converts Shopify developer documentation pages into a structured JSON format, including Markdown content.

## Features

- Fetches Shopify developer documentation pages
- Parses HTML content and extracts relevant information
- Converts HTML to Markdown
- Structures the content into JSON format
- Provides an API endpoint for easy integration

## Prerequisites

- Go 1.16 or higher
- Git

## Installation

1. Clone the repository:

   ```
   git clone https://github.com/aiocean/shopify-doc-extractor.git
   cd shopify-doc-extractor
   ```

2. Install dependencies:
   ```
   go mod download
   ```

## Usage

1. Start the server:

   ```
   go run main.go
   ```

2. The server will start on port 8080 by default. You can change this by setting the `PORT` environment variable.

3. Use the `/extract` endpoint to fetch and parse Shopify documentation:

   ```
   GET http://localhost:8080/extract?url=https://shopify.dev/api/admin-rest/2023-04/resources/discount
   ```

4. The response will be a JSON object containing the parsed documentation, including:
   - Document title
   - Relative URL
   - Markdown content
   - Document sections

## API

### GET /extract

Extracts and parses a Shopify developer documentation page.

Query Parameters:

- `url`: The URL of the Shopify documentation page to extract (must start with `https://shopify.dev`)

Response:
