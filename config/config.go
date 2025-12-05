package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                string
	Env                 string
	APIKeys             []string
	RapidAPIProxySecret string

	// Embedding
	EmbeddingProvider  string
	EmbeddingModel     string
	EmbeddingDimension int

	// OpenAI
	OpenAIAPIKey         string
	OpenAIEmbeddingModel string

	// Limits
	MaxBatchSize     int
	MaxChunkSize     int
	DefaultChunkSize int
	SyncFileLimitMB  int

	// Rate Limiting
	RateLimitPerSecond int
	RateLimitBurst     int

	// Storage
	StorageType string
	StoragePath string
}

var AppConfig *Config

func Load() (*Config, error) {
	// Load .env file if it exists
	_ = godotenv.Load()

	config := &Config{
		Port:                getEnv("PORT", "8080"),
		Env:                 getEnv("ENV", "development"),
		APIKeys:             strings.Split(getEnv("API_KEYS", "test-api-key"), ","),
		RapidAPIProxySecret: getEnv("RAPIDAPI_PROXY_SECRET", ""),

		EmbeddingProvider:  getEnv("EMBEDDING_PROVIDER", "mock"),
		EmbeddingModel:     getEnv("EMBEDDING_MODEL", "embed-large-512"),
		EmbeddingDimension: getEnvInt("EMBEDDING_DIMENSION", 512),

		OpenAIAPIKey:         getEnv("OPENAI_API_KEY", ""),
		OpenAIEmbeddingModel: getEnv("OPENAI_EMBEDDING_MODEL", "text-embedding-3-small"),

		MaxBatchSize:     getEnvInt("MAX_BATCH_SIZE", 100),
		MaxChunkSize:     getEnvInt("MAX_CHUNK_SIZE", 8000),
		DefaultChunkSize: getEnvInt("DEFAULT_CHUNK_SIZE", 1000),
		SyncFileLimitMB:  getEnvInt("SYNC_FILE_LIMIT_MB", 5),

		RateLimitPerSecond: getEnvInt("RATE_LIMIT_PER_SECOND", 10),
		RateLimitBurst:     getEnvInt("RATE_LIMIT_BURST", 20),

		StorageType: getEnv("STORAGE_TYPE", "local"),
		StoragePath: getEnv("STORAGE_PATH", "./storage"),
	}

	AppConfig = config
	return config, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}
