package services

import (
	"batch-embedding-api/config"
	"batch-embedding-api/models"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Worker handles async job processing
type Worker struct {
	config           *config.Config
	jobStore         *JobStore
	embeddingService *EmbeddingService
	jobQueue         chan string
	wg               sync.WaitGroup
	stopCh           chan struct{}
}

// NewWorker creates a new background worker
func NewWorker(cfg *config.Config, jobStore *JobStore, embeddingService *EmbeddingService) *Worker {
	return &Worker{
		config:           cfg,
		jobStore:         jobStore,
		embeddingService: embeddingService,
		jobQueue:         make(chan string, 100),
		stopCh:           make(chan struct{}),
	}
}

// Start starts the worker with n concurrent processors
func (w *Worker) Start(numWorkers int) {
	for i := 0; i < numWorkers; i++ {
		w.wg.Add(1)
		go w.processLoop(i)
	}
	log.Printf("Started %d background workers", numWorkers)
}

// Stop gracefully stops the worker
func (w *Worker) Stop() {
	close(w.stopCh)
	w.wg.Wait()
	log.Println("All workers stopped")
}

// EnqueueJob adds a job to the processing queue
func (w *Worker) EnqueueJob(jobID string) {
	w.jobQueue <- jobID
}

func (w *Worker) processLoop(workerID int) {
	defer w.wg.Done()

	for {
		select {
		case <-w.stopCh:
			return
		case jobID := <-w.jobQueue:
			w.processJob(workerID, jobID)
		}
	}
}

func (w *Worker) processJob(workerID int, jobID string) {
	job := w.jobStore.GetJob(jobID)
	if job == nil {
		log.Printf("[Worker %d] Job %s not found", workerID, jobID)
		return
	}

	log.Printf("[Worker %d] Processing job %s", workerID, jobID)

	// Update status to running
	job.Status = "running"
	job.Progress = 0
	w.jobStore.UpdateJob(job)

	// Process each file
	results := make([]models.EmbedResponse, 0)
	totalFiles := len(job.Files)

	for i, fileURL := range job.Files {
		// Download file
		content, filename, err := w.downloadFile(fileURL)
		if err != nil {
			log.Printf("[Worker %d] Error downloading %s: %v", workerID, fileURL, err)
			job.Status = "failed"
			job.Error = &models.Error{Code: "download_failed", Message: err.Error()}
			w.jobStore.UpdateJob(job)
			w.sendCallback(job)
			return
		}

		// Extract text
		text, err := w.embeddingService.ExtractTextFromFile(filename, content)
		if err != nil {
			log.Printf("[Worker %d] Error extracting text from %s: %v", workerID, filename, err)
			job.Status = "failed"
			job.Error = &models.Error{Code: "extraction_failed", Message: err.Error()}
			w.jobStore.UpdateJob(job)
			w.sendCallback(job)
			return
		}

		// Generate embeddings
		req := &models.EmbedRequest{
			Model:            job.Model,
			Inputs:           []models.InputItem{{ID: filename, Text: text}},
			TruncateStrategy: "split",
			ChunkSize:        w.config.DefaultChunkSize,
			Normalize:        true,
		}

		resp, err := w.embeddingService.GenerateEmbeddings(req)
		if err != nil {
			log.Printf("[Worker %d] Error generating embeddings for %s: %v", workerID, filename, err)
			job.Status = "failed"
			job.Error = &models.Error{Code: "embedding_failed", Message: err.Error()}
			w.jobStore.UpdateJob(job)
			w.sendCallback(job)
			return
		}

		results = append(results, *resp)

		// Update progress
		job.Progress = ((i + 1) * 100) / totalFiles
		w.jobStore.UpdateJob(job)
	}

	// Save results
	resultPath, err := w.saveResults(job.JobID, results)
	if err != nil {
		log.Printf("[Worker %d] Error saving results for job %s: %v", workerID, jobID, err)
		job.Status = "failed"
		job.Error = &models.Error{Code: "storage_failed", Message: err.Error()}
		w.jobStore.UpdateJob(job)
		w.sendCallback(job)
		return
	}

	// Mark as completed
	job.Status = "completed"
	job.Progress = 100
	job.ResultURLs = []string{resultPath}
	w.jobStore.UpdateJob(job)

	log.Printf("[Worker %d] Job %s completed", workerID, jobID)

	// Send callback
	w.sendCallback(job)
}

func (w *Worker) downloadFile(url string) ([]byte, string, error) {
	// Check if it's a local file path
	if _, err := os.Stat(url); err == nil {
		content, err := os.ReadFile(url)
		if err != nil {
			return nil, "", err
		}
		return content, filepath.Base(url), nil
	}

	// Download from URL
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("failed to download: status %d", resp.StatusCode)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	// Extract filename from URL
	filename := filepath.Base(url)
	if filename == "" || filename == "." || filename == "/" {
		filename = "downloaded_file"
	}

	return content, filename, nil
}

func (w *Worker) saveResults(jobID string, results []models.EmbedResponse) (string, error) {
	// Ensure storage directory exists
	storagePath := w.config.StoragePath
	if err := os.MkdirAll(storagePath, 0755); err != nil {
		return "", err
	}

	// Create result file
	filename := fmt.Sprintf("%s_results.json", jobID)
	filepath := filepath.Join(storagePath, filename)

	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return "", err
	}

	// Return URL path (in production, this would be S3 URL or similar)
	return fmt.Sprintf("/v1/results/%s", filename), nil
}

func (w *Worker) sendCallback(job *models.Job) {
	if job.CallbackURL == "" {
		return
	}

	payload := map[string]interface{}{
		"job_id":      job.JobID,
		"status":      job.Status,
		"result_urls": job.ResultURLs,
	}

	if job.Error != nil {
		payload["error"] = job.Error
	}

	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshaling callback payload: %v", err)
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(job.CallbackURL, "application/json", bytes.NewReader(data))
	if err != nil {
		log.Printf("Error sending callback to %s: %v", job.CallbackURL, err)
		return
	}
	defer resp.Body.Close()

	log.Printf("Callback sent to %s: status %d", job.CallbackURL, resp.StatusCode)
}
