package services

import (
	"batch-embedding-api/config"
	"batch-embedding-api/models"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"unicode/utf8"
)

// EmbeddingService handles all embedding operations
type EmbeddingService struct {
	config *config.Config
}

// NewEmbeddingService creates a new embedding service
func NewEmbeddingService(cfg *config.Config) *EmbeddingService {
	return &EmbeddingService{config: cfg}
}

// GenerateEmbeddings generates embeddings for the given inputs
func (s *EmbeddingService) GenerateEmbeddings(req *models.EmbedRequest) (*models.EmbedResponse, error) {
	results := make([]models.EmbedResult, 0, len(req.Inputs))

	chunkSize := req.ChunkSize
	if chunkSize <= 0 {
		chunkSize = s.config.DefaultChunkSize
	}
	if chunkSize > s.config.MaxChunkSize {
		chunkSize = s.config.MaxChunkSize
	}

	truncateStrategy := req.TruncateStrategy
	if truncateStrategy == "" {
		truncateStrategy = "truncate"
	}

	for _, input := range req.Inputs {
		result := models.EmbedResult{ID: input.ID}

		textLen := utf8.RuneCountInString(input.Text)

		if textLen <= chunkSize {
			// No chunking needed
			embedding := s.generateEmbedding(input.Text, req.Normalize)
			result.Embeddings = embedding
		} else {
			// Chunking needed
			chunks := s.chunkText(input.ID, input.Text, chunkSize, truncateStrategy)
			result.Chunks = make([]models.Chunk, 0, len(chunks))

			for _, chunk := range chunks {
				embedding := s.generateEmbedding(chunk.Text, req.Normalize)
				result.Chunks = append(result.Chunks, models.Chunk{
					ChunkID:     chunk.ChunkID,
					Start:       chunk.Start,
					End:         chunk.End,
					TextSnippet: truncateSnippet(chunk.Text, 200),
					Embedding:   embedding,
				})
			}
		}

		results = append(results, result)
	}

	return &models.EmbedResponse{Results: results}, nil
}

// TextChunk represents a chunk of text
type TextChunk struct {
	ChunkID string
	Text    string
	Start   int
	End     int
}

// chunkText splits text into chunks based on strategy
func (s *EmbeddingService) chunkText(docID, text string, chunkSize int, strategy string) []TextChunk {
	runes := []rune(text)
	textLen := len(runes)

	if strategy == "truncate" {
		// Just return first chunk
		end := chunkSize
		if end > textLen {
			end = textLen
		}
		return []TextChunk{{
			ChunkID: fmt.Sprintf("%s_0", docID),
			Text:    string(runes[:end]),
			Start:   0,
			End:     end,
		}}
	}

	// Split strategy - split into multiple chunks
	var chunks []TextChunk
	chunkIndex := 0

	for start := 0; start < textLen; start += chunkSize {
		end := start + chunkSize
		if end > textLen {
			end = textLen
		}

		chunks = append(chunks, TextChunk{
			ChunkID: fmt.Sprintf("%s_%d", docID, chunkIndex),
			Text:    string(runes[start:end]),
			Start:   start,
			End:     end,
		})
		chunkIndex++
	}

	return chunks
}

// generateEmbedding generates embedding for text
// This is a mock implementation - replace with actual model call
func (s *EmbeddingService) generateEmbedding(text string, normalize bool) []float32 {
	dimension := s.config.EmbeddingDimension

	switch s.config.EmbeddingProvider {
	case "openai":
		// TODO: Implement OpenAI embedding
		return s.mockEmbedding(text, dimension, normalize)
	case "mock":
		fallthrough
	default:
		return s.mockEmbedding(text, dimension, normalize)
	}
}

// mockEmbedding generates a deterministic mock embedding based on text
func (s *EmbeddingService) mockEmbedding(text string, dimension int, normalize bool) []float32 {
	// Use text hash as seed for reproducibility
	seed := int64(0)
	for _, r := range text {
		seed = seed*31 + int64(r)
	}
	rng := rand.New(rand.NewSource(seed))

	embedding := make([]float32, dimension)
	for i := range embedding {
		embedding[i] = rng.Float32()*2 - 1 // Range [-1, 1]
	}

	if normalize {
		embedding = normalizeL2(embedding)
	}

	return embedding
}

// normalizeL2 applies L2 normalization to a vector
func normalizeL2(vec []float32) []float32 {
	var sumSq float64
	for _, v := range vec {
		sumSq += float64(v * v)
	}

	norm := math.Sqrt(sumSq)
	if norm == 0 {
		return vec
	}

	result := make([]float32, len(vec))
	for i, v := range vec {
		result[i] = float32(float64(v) / norm)
	}
	return result
}

// truncateSnippet truncates text to maxLen characters
func truncateSnippet(text string, maxLen int) string {
	runes := []rune(text)
	if len(runes) <= maxLen {
		return text
	}
	return string(runes[:maxLen]) + "..."
}

// ExtractTextFromFile extracts text from file bytes
func (s *EmbeddingService) ExtractTextFromFile(filename string, content []byte) (string, error) {
	lowerName := strings.ToLower(filename)

	if strings.HasSuffix(lowerName, ".txt") {
		return string(content), nil
	}

	if strings.HasSuffix(lowerName, ".pdf") {
		// Simple PDF text extraction (basic implementation)
		// For production, use a proper PDF library
		text := extractPDFText(content)
		if text == "" {
			return "", fmt.Errorf("could not extract text from PDF")
		}
		return text, nil
	}

	return "", fmt.Errorf("unsupported file type: %s", filename)
}

// extractPDFText is a basic PDF text extractor
// For production, use github.com/ledongthuc/pdf or similar
func extractPDFText(content []byte) string {
	// This is a very basic implementation
	// It looks for text between "stream" and "endstream" markers
	// For real PDF parsing, use a proper library

	text := string(content)

	// Try to find readable text (very basic approach)
	var result strings.Builder
	inText := false

	for i := 0; i < len(text); i++ {
		c := text[i]

		// Look for BT (Begin Text) and ET (End Text) markers
		if i < len(text)-1 {
			if text[i] == 'B' && text[i+1] == 'T' {
				inText = true
				continue
			}
			if text[i] == 'E' && text[i+1] == 'T' {
				inText = false
				result.WriteRune(' ')
				continue
			}
		}

		if inText {
			// Extract text between parentheses
			if c == '(' {
				j := i + 1
				for j < len(text) && text[j] != ')' {
					if text[j] >= 32 && text[j] < 127 {
						result.WriteByte(text[j])
					}
					j++
				}
				i = j
			}
		}
	}

	// Fallback: if no text found, try to extract any readable ASCII
	if result.Len() == 0 {
		for _, c := range content {
			if c >= 32 && c < 127 {
				result.WriteByte(c)
			} else if c == '\n' || c == '\r' || c == '\t' {
				result.WriteByte(' ')
			}
		}
	}

	return strings.TrimSpace(result.String())
}
