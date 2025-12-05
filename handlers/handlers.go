package handlers

import (
	"batch-embedding-api/config"
	"batch-embedding-api/models"
	"batch-embedding-api/services"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// Handler contains all HTTP handlers
type Handler struct {
	config           *config.Config
	embeddingService *services.EmbeddingService
	jobStore         *services.JobStore
	worker           *services.Worker
}

// NewHandler creates a new handler instance
func NewHandler(cfg *config.Config, embeddingService *services.EmbeddingService, jobStore *services.JobStore, worker *services.Worker) *Handler {
	return &Handler{
		config:           cfg,
		embeddingService: embeddingService,
		jobStore:         jobStore,
		worker:           worker,
	}
}

// Health handles GET /v1/health
func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, models.HealthResponse{
		Status:     "ok",
		Version:    "1.0.0",
		QueueDepth: h.jobStore.GetQueueDepth(),
	})
}

// Embed handles POST /v1/embed - synchronous embedding
func (h *Handler) Embed(c *gin.Context) {
	var req models.EmbedRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.Error{
			Code:    "invalid_request",
			Message: err.Error(),
		})
		return
	}

	// Validate batch size
	if len(req.Inputs) > h.config.MaxBatchSize {
		c.JSON(http.StatusRequestEntityTooLarge, models.Error{
			Code:    "payload_too_large",
			Message: "Batch size exceeds maximum allowed",
		})
		return
	}

	// Validate chunk size
	if req.ChunkSize < 0 || req.ChunkSize > h.config.MaxChunkSize {
		c.JSON(http.StatusBadRequest, models.Error{
			Code:    "invalid_request",
			Message: "Invalid chunk_size",
		})
		return
	}

	// Validate truncate strategy
	if req.TruncateStrategy != "" && req.TruncateStrategy != "truncate" && req.TruncateStrategy != "split" {
		c.JSON(http.StatusBadRequest, models.Error{
			Code:    "invalid_request",
			Message: "truncate_strategy must be 'truncate' or 'split'",
		})
		return
	}

	// Generate embeddings
	resp, err := h.embeddingService.GenerateEmbeddings(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.Error{
			Code:    "internal_error",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// EmbedFile handles POST /v1/embed/file - file upload embedding
func (h *Handler) EmbedFile(c *gin.Context) {
	// Get file from form
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.Error{
			Code:    "invalid_request",
			Message: "No file uploaded",
		})
		return
	}
	defer file.Close()

	// Validate file type
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".pdf" && ext != ".txt" {
		c.JSON(http.StatusBadRequest, models.Error{
			Code:    "invalid_request",
			Message: "Unsupported file type. Only PDF and TXT files are allowed.",
		})
		return
	}

	// Check file size
	fileSizeMB := float64(header.Size) / (1024 * 1024)
	if fileSizeMB > float64(h.config.SyncFileLimitMB) {
		// Too large for sync processing - create async job
		// For now, just return an error. Full async would save file first.
		c.JSON(http.StatusRequestEntityTooLarge, models.Error{
			Code:    "payload_too_large",
			Message: "File too large for synchronous processing. Use /v1/jobs for async processing.",
		})
		return
	}

	// Read file content
	content, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.Error{
			Code:    "internal_error",
			Message: "Failed to read file",
		})
		return
	}

	// Extract text from file
	text, err := h.embeddingService.ExtractTextFromFile(header.Filename, content)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.Error{
			Code:    "invalid_request",
			Message: err.Error(),
		})
		return
	}

	// Get form parameters
	model := c.DefaultPostForm("model", h.config.EmbeddingModel)
	truncateStrategy := c.DefaultPostForm("truncate_strategy", "split")
	chunkSize := h.config.DefaultChunkSize
	normalize := c.DefaultPostForm("normalize", "true") == "true"

	// Generate embeddings
	req := &models.EmbedRequest{
		Model:            model,
		Inputs:           []models.InputItem{{ID: header.Filename, Text: text}},
		TruncateStrategy: truncateStrategy,
		ChunkSize:        chunkSize,
		Normalize:        normalize,
	}

	resp, err := h.embeddingService.GenerateEmbeddings(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.Error{
			Code:    "internal_error",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// CreateJob handles POST /v1/jobs - create async job
func (h *Handler) CreateJob(c *gin.Context) {
	var req models.AsyncJobRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.Error{
			Code:    "invalid_request",
			Message: err.Error(),
		})
		return
	}

	// Validate files
	if len(req.Files) == 0 {
		c.JSON(http.StatusBadRequest, models.Error{
			Code:    "invalid_request",
			Message: "At least one file URL is required",
		})
		return
	}

	// Validate URLs
	for _, fileURL := range req.Files {
		if !strings.HasPrefix(fileURL, "http://") && !strings.HasPrefix(fileURL, "https://") {
			// Check if it's a valid local path (for testing)
			if !strings.Contains(fileURL, "/") && !strings.Contains(fileURL, "\\") {
				c.JSON(http.StatusBadRequest, models.Error{
					Code:    "invalid_request",
					Message: "Invalid file URL: " + fileURL,
				})
				return
			}
		}
	}

	// Create job
	job := h.jobStore.CreateJob(req.Files, req.Model, req.CallbackURL)

	// Enqueue for processing
	h.worker.EnqueueJob(job.JobID)

	c.JSON(http.StatusAccepted, models.AsyncAcceptedResponse{
		JobID:   job.JobID,
		Status:  job.Status,
		Message: "Job accepted for processing",
	})
}

// GetJob handles GET /v1/jobs/:job_id
func (h *Handler) GetJob(c *gin.Context) {
	jobID := c.Param("job_id")

	job := h.jobStore.GetJob(jobID)
	if job == nil {
		c.JSON(http.StatusNotFound, models.Error{
			Code:    "not_found",
			Message: "Job not found",
		})
		return
	}

	c.JSON(http.StatusOK, models.JobStatus{
		JobID:      job.JobID,
		Status:     job.Status,
		Progress:   job.Progress,
		ResultURLs: job.ResultURLs,
		Error:      job.Error,
	})
}

// GetResults handles GET /v1/results/:filename - serve result files
func (h *Handler) GetResults(c *gin.Context) {
	filename := c.Param("filename")

	// Sanitize filename to prevent directory traversal
	filename = filepath.Base(filename)

	filePath := filepath.Join(h.config.StoragePath, filename)
	c.File(filePath)
}

// ListJobs handles GET /v1/jobs - list all jobs (optional endpoint)
func (h *Handler) ListJobs(c *gin.Context) {
	jobs := h.jobStore.ListJobs()

	statuses := make([]models.JobStatus, 0, len(jobs))
	for _, job := range jobs {
		statuses = append(statuses, models.JobStatus{
			JobID:      job.JobID,
			Status:     job.Status,
			Progress:   job.Progress,
			ResultURLs: job.ResultURLs,
			Error:      job.Error,
		})
	}

	c.JSON(http.StatusOK, gin.H{"jobs": statuses})
}
