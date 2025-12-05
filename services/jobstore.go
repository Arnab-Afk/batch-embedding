package services

import (
	"batch-embedding-api/models"
	"sync"
	"time"

	"github.com/google/uuid"
)

// JobStore manages job storage (in-memory for now)
type JobStore struct {
	jobs  map[string]*models.Job
	mutex sync.RWMutex
}

// NewJobStore creates a new job store
func NewJobStore() *JobStore {
	return &JobStore{
		jobs: make(map[string]*models.Job),
	}
}

// CreateJob creates a new job
func (s *JobStore) CreateJob(files []string, model, callbackURL string) *models.Job {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	job := &models.Job{
		JobID:       uuid.New().String(),
		Status:      "queued",
		Progress:    0,
		Files:       files,
		Model:       model,
		CallbackURL: callbackURL,
		CreatedAt:   time.Now().Unix(),
		UpdatedAt:   time.Now().Unix(),
	}

	s.jobs[job.JobID] = job
	return job
}

// GetJob retrieves a job by ID
func (s *JobStore) GetJob(jobID string) *models.Job {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.jobs[jobID]
}

// UpdateJob updates a job
func (s *JobStore) UpdateJob(job *models.Job) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	job.UpdatedAt = time.Now().Unix()
	s.jobs[job.JobID] = job
}

// GetQueueDepth returns the number of pending/running jobs
func (s *JobStore) GetQueueDepth() int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	count := 0
	for _, job := range s.jobs {
		if job.Status == "queued" || job.Status == "running" {
			count++
		}
	}
	return count
}

// ListJobs returns all jobs
func (s *JobStore) ListJobs() []*models.Job {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	jobs := make([]*models.Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, job)
	}
	return jobs
}
