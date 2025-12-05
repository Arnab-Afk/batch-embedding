package models

// EmbedRequest represents the request body for /v1/embed
type EmbedRequest struct {
	Model            string      `json:"model" binding:"required"`
	Inputs           []InputItem `json:"inputs" binding:"required,min=1"`
	TruncateStrategy string      `json:"truncate_strategy,omitempty"` // "truncate" or "split"
	ChunkSize        int         `json:"chunk_size,omitempty"`
	Normalize        bool        `json:"normalize,omitempty"`
}

// InputItem represents a single text input
type InputItem struct {
	ID   string `json:"id" binding:"required"`
	Text string `json:"text" binding:"required"`
}

// EmbedResponse represents the response for /v1/embed
type EmbedResponse struct {
	Results []EmbedResult `json:"results"`
}

// EmbedResult represents embedding result for a single input
type EmbedResult struct {
	ID         string    `json:"id"`
	Embeddings []float32 `json:"embeddings,omitempty"`
	Chunks     []Chunk   `json:"chunks,omitempty"`
}

// Chunk represents a text chunk with its embedding
type Chunk struct {
	ChunkID     string    `json:"chunk_id"`
	Start       int       `json:"start"`
	End         int       `json:"end"`
	TextSnippet string    `json:"text_snippet"`
	Embedding   []float32 `json:"embedding"`
}

// FileEmbedRequest represents the request for file upload
type FileEmbedRequest struct {
	Model            string `form:"model"`
	TruncateStrategy string `form:"truncate_strategy"`
	ChunkSize        int    `form:"chunk_size"`
	Normalize        bool   `form:"normalize"`
}

// AsyncJobRequest represents the request for async job creation
type AsyncJobRequest struct {
	Model       string   `json:"model" binding:"required"`
	Files       []string `json:"files" binding:"required,min=1"`
	CallbackURL string   `json:"callback_url,omitempty"`
	Priority    string   `json:"priority,omitempty"` // "low", "normal", "high"
}

// Job represents an async embedding job
type Job struct {
	JobID       string   `json:"job_id"`
	Status      string   `json:"status"` // "queued", "running", "completed", "failed"
	Progress    int      `json:"progress,omitempty"`
	Files       []string `json:"files"`
	Model       string   `json:"model"`
	ResultURLs  []string `json:"result_urls,omitempty"`
	Error       *Error   `json:"error,omitempty"`
	CreatedAt   int64    `json:"created_at"`
	UpdatedAt   int64    `json:"updated_at"`
	CallbackURL string   `json:"callback_url,omitempty"`
}

// JobStatus represents job status response
type JobStatus struct {
	JobID      string   `json:"job_id"`
	Status     string   `json:"status"`
	Progress   int      `json:"progress"`
	ResultURLs []string `json:"result_urls,omitempty"`
	Error      *Error   `json:"error,omitempty"`
}

// HealthResponse represents health check response
type HealthResponse struct {
	Status     string `json:"status"`
	Version    string `json:"version"`
	QueueDepth int    `json:"queue_depth"`
}

// Error represents API error response
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// AsyncAcceptedResponse represents 202 response for async jobs
type AsyncAcceptedResponse struct {
	JobID   string `json:"job_id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}
