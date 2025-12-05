# 📘 Batch Document Embedding API

A high-performance REST API for generating vector embeddings from text and documents. Built with Go for speed and reliability, designed for RapidAPI deployment.

## 🚀 Quick Start

```bash
# Clone and setup
git clone <your-repo>
cd batch-embedding

# Install dependencies
go mod download

# Configure environment
cp .env.example .env
# Edit .env with your API keys

# Build and run
go build -o embedding-api .
./embedding-api
```

Server starts at `http://localhost:8080`

## 📋 Features

- ✅ **Synchronous embedding** - Instant embeddings for small batches
- ✅ **File upload** - PDF and TXT support with text extraction
- ✅ **Async job processing** - Background workers for large files
- ✅ **Text chunking** - Automatic splitting with configurable chunk size
- ✅ **L2 normalization** - Optional vector normalization
- ✅ **Rate limiting** - Configurable per-client limits
- ✅ **API key auth** - Bearer token + RapidAPI proxy support
- ✅ **Webhooks** - Callback notifications for async jobs

## 🔧 Configuration

Copy `.env.example` to `.env` and configure:

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 8080 | Server port |
| `API_KEYS` | test-api-key | Comma-separated valid API keys |
| `RAPIDAPI_PROXY_SECRET` | | RapidAPI proxy secret for validation |
| `EMBEDDING_PROVIDER` | mock | `mock` or `openai` |
| `EMBEDDING_DIMENSION` | 512 | Vector dimension |
| `MAX_BATCH_SIZE` | 100 | Max inputs per request |
| `DEFAULT_CHUNK_SIZE` | 1000 | Characters per chunk |
| `RATE_LIMIT_PER_SECOND` | 10 | Rate limit |

## 📡 API Endpoints

### Health Check
```bash
GET /v1/health
```

### Synchronous Embedding
```bash
POST /v1/embed
Authorization: Bearer <API_KEY>
Content-Type: application/json

{
  "model": "embed-large-512",
  "inputs": [
    { "id": "doc1", "text": "Hello world" },
    { "id": "doc2", "text": "Another document" }
  ],
  "truncate_strategy": "split",
  "chunk_size": 1000,
  "normalize": true
}
```

### File Upload
```bash
POST /v1/embed/file
Authorization: Bearer <API_KEY>
Content-Type: multipart/form-data

file: <your-file.txt or .pdf>
model: embed-large-512
normalize: true
```

### Create Async Job
```bash
POST /v1/jobs
Authorization: Bearer <API_KEY>
Content-Type: application/json

{
  "model": "embed-large-512",
  "files": ["https://example.com/doc.pdf"],
  "callback_url": "https://your-webhook.com/callback"
}
```

### Check Job Status
```bash
GET /v1/jobs/{job_id}
Authorization: Bearer <API_KEY>
```

### Download Results
```bash
GET /v1/results/{filename}
Authorization: Bearer <API_KEY>
```

## 🏗️ Project Structure

```
batch-embedding/
├── main.go                  # Entry point, router setup
├── go.mod                   # Go module dependencies
├── .env                     # Local configuration
├── .env.example             # Configuration template
├── config/
│   └── config.go            # Configuration loader
├── models/
│   └── models.go            # Request/Response types
├── handlers/
│   └── handlers.go          # HTTP handlers
├── middleware/
│   └── middleware.go        # Auth & rate limiting
├── services/
│   ├── embedding.go         # Embedding generation
│   ├── jobstore.go          # Job management
│   └── worker.go            # Background processing
└── storage/                 # Job results (gitignored)
```

## 🚀 Deploy to RapidAPI

1. **Build for production:**
   ```bash
   # Linux
   GOOS=linux GOARCH=amd64 go build -o embedding-api .
   
   # Windows
   go build -o embedding-api.exe .
   ```

2. **Deploy to your server** (VPS, AWS EC2, DigitalOcean, etc.)

3. **Configure RapidAPI:**
   - Add your server URL as the base URL
   - Set `X-RapidAPI-Proxy-Secret` in your `.env`
   - Configure endpoints in RapidAPI dashboard

4. **Set production environment:**
   ```bash
   ENV=production
   API_KEYS=your-secure-key-1,your-secure-key-2
   RAPIDAPI_PROXY_SECRET=your-rapidapi-secret
   ```

## 🔒 Security

- All endpoints (except `/v1/health`) require authentication
- Supports both Bearer token and RapidAPI proxy validation
- Rate limiting prevents abuse
- File type validation for uploads

---

# 📘 Technical Specification

> *Detailed implementation guide below*

---

## System Overview

The API provides endpoints to generate vector embeddings from text and document inputs.
It supports:

* Synchronous embedding generation for small batches
* Asynchronous large-job processing for big PDFs and corpora
* File uploads (PDF, TXT)
* Chunking, text extraction, normalization
* Pluggable embedding model backend (mock, OpenAI, or custom)

**Tech stack:**

* **Go** with Gin web framework
* Background workers (goroutines) for async jobs
* Local storage for job results (extensible to S3)
* Mock embeddings (swap for OpenAI/HuggingFace in production)

---

# 2. **Embedding Pipeline Requirements**

Every embedding request must follow this pipeline:

1. **Input validation**

   * Ensure all items have `id` and `text`
   * Max size per request (configurable)
   * Return `413` for oversized inputs

2. **Chunking logic**

   * If text > `chunk_size`
   * If `truncate_strategy == "truncate"` → cut at limit
   * If `truncate_strategy == "split"` → split into fixed-size chunks
   * Maintain metadata: `chunk_id`, `start`, `end`, and optional snippet

3. **Embedding generation**

   * Convert each chunk into a vector (float array)
   * If `normalize = true`, apply L2 normalization

4. **Response formatting**
   Return:

   ```json
   {
     "results": [
       {
         "id": "doc1",
         "embeddings": [...],      // optional if chunked
         "chunks": [
           {
             "chunk_id": "doc1_0",
             "start": 0,
             "end": 500,
             "text_snippet": "first 200 chars…",
             "embedding": [0.12, -0.33, ...]
           }
         ]
       }
     ]
   }
   ```

---

# 3. **Endpoints to Implement**

## **3.1 POST /v1/embed (Sync)**

Generate embeddings for small text batches.

### Request

```json
{
  "model": "embed-large-512",
  "inputs": [
    { "id": "doc1", "text": "hello world" },
    { "id": "doc2", "text": "longer text..." }
  ],
  "truncate_strategy": "split",
  "chunk_size": 1000,
  "normalize": true
}
```

### Response

* 200 OK → Return embeddings immediately
* 400 Bad Request → Invalid schema
* 413 Payload Too Large → Request too big
* 429 → User exceeded rate limits

---

## **3.2 POST /v1/embed/file (Sync/Async)**

Handle PDF/TXT upload via multipart form.

### File handling:

* Accept `multipart/form-data`
* Validate file type (pdf, txt)
* If file > `SYNC_LIMIT_MB` → switch to async mode
* Extract text:

  * PDF: use pdfminer, pymupdf, or pypdf
  * TXT: read raw

### Response Logic:

| File Size | Behavior                                               |
| --------- | ------------------------------------------------------ |
| Small     | Synchronous: extract → chunk → embed → return 200      |
| Large     | Async: save file → create job → return 202 with job_id |

### 202 Response

```json
{
  "job_id": "job_123",
  "status": "queued",
  "message": "File accepted for processing"
}
```

---

## **3.3 POST /v1/jobs (Async)**

Submit large embedding jobs with file URLs or S3 URLs.

### Request

```json
{
  "model": "embed-large-512",
  "files": [
    "https://bucket.s3.amazonaws.com/doc1.pdf?signature",
    "https://bucket.s3.amazonaws.com/doc2.pdf?signature"
  ],
  "callback_url": "https://example.com/webhook",
  "priority": "normal"
}
```

### Backend Logic:

1. Validate URLs
2. Create new job record → `queued`
3. Add job to worker queue
4. Worker downloads file, extracts text, chunks, and embeds
5. Save results (JSON or NDJSON file)
6. Update job → `completed`
7. If `callback_url` provided → send POST with result metadata

---

## **3.4 GET /v1/jobs/{job_id} (Job Status)**

### Response format:

```json
{
  "job_id": "job_123",
  "status": "running",
  "progress": 45,
  "result_urls": [
    "https://storage/my-job-results/job_123_results.json"
  ]
}
```

### Status values:

* `queued`
* `running`
* `completed`
* `failed`

---

## **3.5 GET /v1/embeddings/{id} (Optional)**

Return stored embedding record if you choose to persist them.

---

## **3.6 GET /v1/health**

Should return:

```json
{
  "status": "ok",
  "version": "1.0.0",
  "queue_depth": 3
}
```

---

# 4. **Data Models**

## **Embedding Model Config**

```ts
interface EmbeddingModelConfig {
  modelName: string;
  dimension: number; // e.g., 512
  normalize: boolean;
  provider: "local" | "openai" | "hf" | "custom";
}
```

## **Job Schema**

```ts
interface EmbeddingJob {
  job_id: string;
  status: "queued" | "running" | "completed" | "failed";
  files: string[];
  created_at: number;
  updated_at: number;
  result_urls?: string[];
  error?: { code: string; message: string };
}
```

---

# 5. **Background Worker Specification**

The async worker must:

1. Poll job queue
2. Update job status (`running`)
3. For each file:

   * Download file
   * Extract text
   * Chunk
   * Embed
4. Save result file
5. Update job → `completed`
6. Send webhook if provided

---

# 6. **Error Handling Specification**

Use consistent error JSON format:

```json
{
  "code": "invalid_request",
  "message": "Chunk size must be > 0"
}
```

Common error codes:

* `invalid_request`
* `unauthorized`
* `payload_too_large`
* `too_many_requests`
* `not_found`
* `internal_error`

---

# 7. **Security Requirements**

* Token-based auth: `Authorization: Bearer <API_KEY>`
* Validate token on every request
* Block requests > size limit early
* Sanitize file names, never trust user input

---

# 8. **Performance Requirements**

* Target max sync request time: **< 10 seconds**
* Async jobs must be processed within **< 5 minutes** for typical PDFs
* Support concurrency:

  * Sync: ≥ 10 requests/sec
  * Async queue: ≥ 20 jobs concurrently

---

# 9. **Testing Checklist**

### Unit testing:

* Chunking logic
* PDF extraction
* Embedding function
* Error handling

### Integration testing:

* Multipart upload
* Job queue + worker
* Webhook delivery
* Rate limiting

### Load testing:

* Ensure system does not collapse under 100 simultaneous sync requests

---

# 10. **Deployment Architecture (Recommended)**

```
Client → API Gateway → FastAPI/Node Server → Worker Queue → Embedding Worker(s)
                                              ↓
                                      Storage (results)
```

Optional:

* Use **Redis** or **SQS** as job queue
* Use **S3** for storing job results
* Use **PostgreSQL** for job metadata

---

---

## Testing

```bash
# Health check
curl http://localhost:8080/v1/health

# Embed text
curl -X POST http://localhost:8080/v1/embed \
  -H "Authorization: Bearer test-api-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"embed-large-512","inputs":[{"id":"doc1","text":"Hello world"}]}'

# Upload file
curl -X POST http://localhost:8080/v1/embed/file \
  -H "Authorization: Bearer test-api-key" \
  -F "file=@test.txt" \
  -F "model=embed-large-512"
```

## License

MIT
