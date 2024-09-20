package main

import (
	"context"
	"fmt"
	ollama "github.com/amikos-tech/chroma-go/pkg/embeddings/ollama"
)

func ollamaembedder() {
	documents := []string{
		"Document 1 content here",
		"Document 2 content here",
	}
	// the `/api/embeddings` endpoint is automatically appended to the base URL
	ef, err := ollama.NewOllamaEmbeddingFunction(ollama.WithBaseURL("http://localhost:11434"), ollama.WithModel("nomic-embed-text"))
	if err != nil {
		fmt.Printf("Error creating Ollama embedding function: %s \n", err)
	}
	resp, err := ef.EmbedDocuments(context.Background(), documents)
	if err != nil {
		fmt.Printf("Error embedding documents: %s \n", err)
	}
	fmt.Printf("Embedding response: %v \n", resp)
}
