package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"

	"github.com/tmc/langchaingo/documentloaders"
	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
	"github.com/tmc/langchaingo/schema"
	"github.com/tmc/langchaingo/textsplitter"
	"github.com/tmc/langchaingo/vectorstores"
	"github.com/tmc/langchaingo/vectorstores/redisvector"
)

const PROMPT_TEMPLATE = `
Answer the question based only on the following context:

%s

---

Answer the question based on the above context: %s
`

func main() {
	redisURL := "redis://127.0.0.1:6379"

	load := flag.String("load", "", "Load file")
	query := flag.String("query", "", "Prompt to use for query")
	// test_redis_rag
	index := flag.String("index", "", "Which index to use for load or query")

	flag.Parse()

	if *index == "" {
		log.Fatalln("The --index option is required")
	}

	// llmForPrompt, err := ollama.New(ollama.WithModel("llama3.1:8b"))
	llmForPrompt, err := ollama.New(ollama.WithModel("mistral"))
	if err != nil {
		log.Fatal(err)
	}
	// llmForEmbed, err := ollama.New(ollama.WithModel("gemma:2b"))
	llmForEmbed, err := ollama.New(ollama.WithModel("nomic-embed-text"))
	if err != nil {
		log.Fatal(err)
	}
	embedder := embeddingFunction(llmForEmbed)

	db, err := redisvector.New(context.Background(),
		redisvector.WithConnectionURL(redisURL),
		redisvector.WithIndexName(*index, true),
		redisvector.WithEmbedder(embedder),
	)
	if err != nil {
		slog.Error("Error creating Redis store", "err", err, "redisUrl", redisURL)
	}
	// chromaURL := os.Getenv("CHROMA_URL")
	// db, err := chroma.New(
	// 	chroma.WithChromaURL(chromaURL),
	// 	chroma.WithDistanceFunction(chroma_go.COSINE),
	// 	chroma.WithNameSpace(uuid.New().String()),
	// 	chroma.WithEmbedder(embedder),
	// )
	// if err != nil {
	// 	slog.Error("Error creating Chroma store", "err", err, "chroma_url", chromaURL)
	// }

	// prompt := "What would be a good company name for a company that makes colorful socks?"
	// completion, err := llms.GenerateFromSinglePrompt(ctx, llm, prompt)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// fmt.Println(completion)
	if *load != "" {
		// filename := "data/vam.pdf"
		filename := *load
		documents := loadDocuments(filename)
		addToStore(db, filename, documents)
		// fmt.Println(documents)
	}

	if *query != "" {
		queryRag(*query, db, llmForPrompt, embedder)
	}

}

func queryRag(query string, db *redisvector.Store, llm *ollama.LLM, embedder embeddings.Embedder) {
	slog.Info("Querying database", "query", query)
	embedderData, err := embedder.EmbedQuery(context.Background(), query)
	if err != nil {
		slog.Error("Error in embedding", "err", err, "query", query)
	}
	slog.Info("Embedding", "data", embedderData)
	results, err := db.SimilaritySearch(context.TODO(), query, 15,
		vectorstores.WithScoreThreshold(0.7))
	if err != nil {
		slog.Error("Error in similarity search", "err", err, "query", query)
	}

	// slog.Info("Results", "results", results)
	contextText := ""
	for _, r := range results {
		slog.Info("Context", "PageContent", r.PageContent)
		contextText += r.PageContent
		contextText += "\n\n---\n\n"
	}

	finalPrompt := fmt.Sprintf(PROMPT_TEMPLATE, contextText, query)
	slog.Info("Final promp", "finalPrompt", finalPrompt)

	content := []llms.MessageContent{
		// llms.TextParts(llms.ChatMessageTypeSystem, contextText),
		// llms.TextParts(llms.ChatMessageTypeHuman, query),
		llms.TextParts(llms.ChatMessageTypeHuman, finalPrompt),
	}
	_, err = llm.GenerateContent(context.Background(), content, llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
		fmt.Print(string(chunk))
		return nil
	}))
	if err != nil {
		slog.Error("Error while generating content", "err", err)
	}

	// for _, c := range completion.Choices {
	// 	fmt.Println(c.Content)
	// }

	fmt.Println()
	var meta []any
	for _, r := range results {
		for k, m := range r.Metadata {
			// slog.Info("Metadata", "k", k, "m", m)
			if k == "id" {
				meta = append(meta, m)
			}
		}

	}

	fmt.Println("Source", "Metadata", meta)
}

func loadDocuments(filename string) []schema.Document {
	file, err := os.Open(filename)
	if err != nil {
		slog.Error("Error opening file", "err", err, "filename", filename)
	}
	defer file.Close()
	fileInfo, err := file.Stat()
	if err != nil {
		slog.Error("Error getting file stat", "err", err, "filename", filename)
	}
	fileSize := fileInfo.Size()
	slog.Info("File info", "filename", filename, "fileSize", fileSize)
	return splitDocuments(documentloaders.NewText(file))
}

func splitDocuments(inputDocument documentloaders.Text) []schema.Document {
	textSplitter := textsplitter.NewRecursiveCharacter()
	// textSplitter.Separators = []string{"\n\n"}
	textSplitter.ChunkSize = 500
	textSplitter.ChunkOverlap = 50

	documents, err := inputDocument.LoadAndSplit(context.Background(), textSplitter)
	if err != nil {
		slog.Error("Error splitting document", "err", err, "inputDocument", inputDocument)
	}
	return documents

}
func embeddingFunction(llm *ollama.LLM) embeddings.Embedder {
	embedder, err := embeddings.NewEmbedder(llm)
	if err != nil {
		slog.Error("Error creating embedder", "err", err)
	}
	return embedder
}
func addToStore(db *redisvector.Store, filename string, documents []schema.Document) {

	calculateDocumentIds(filename, documents)

	for i, doc := range documents {
		slog.Info("Adding document", "i", i, "doc", doc, "metadata", doc.Metadata)
	}

	_, err := db.AddDocuments(context.Background(), documents)
	if err != nil {
		slog.Error("Error adding documents", "err", err)
	}

}

func calculateDocumentIds(filename string, documents []schema.Document) {
	filenameSplits := strings.Split(filename, "/")
	filenameShort := filenameSplits[len(filenameSplits)-1]
	currentChunkIndex := 0
	for _, doc := range documents {
		doc.Metadata["source"] = filenameShort
		source := doc.Metadata["source"]
		currentChunkIndex += 1
		chunkId := fmt.Sprintf("%s:%d", source, currentChunkIndex)
		doc.Metadata["id"] = chunkId
	}

}
