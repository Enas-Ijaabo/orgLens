# NovaLore

**The knowledge hidden in your code, instantly queryable.**

Drop in any codebase or doc folder. NovaLore uses Amazon Nova to extract factual knowledge statements from every file, stores them as embeddings, and lets you ask natural-language questions — answered with grounded citations pointing back to the source.

No manual documentation. No hallucinations. Just your code, made searchable.

Built for the **Amazon Nova AI Hackathon 2026**.

### Why NovaLore?

Small teams and solo developers accumulate knowledge in code, configs, and docs — but that knowledge is buried and unsearchable. Onboarding a new engineer, revisiting a service after months, or answering "how does X work?" means grepping through files hoping to find the right one. NovaLore extracts that knowledge once and makes it queryable forever, grounded in citations so you can always trace an answer back to its source.

---

## How it works

```
Drop in your codebase / docs
        │
        ▼
  Nova Lite reads each file and extracts factual knowledge statements
  "Auth service rotates JWT secrets every 24 hours [auth.go]"
        │
        ▼
  Nova Multimodal Embeddings converts each fact to a 1024-dim vector
        │
        ▼
  ChromaDB stores facts + embeddings (persistent volume)
        │
        ▼
  Ask a question → vector search → Nova Lite synthesizes a grounded answer
```

Ingestion starts automatically on startup. The **Ingest** tab shows live per-file progress (`extracting → indexing → done`). Hit **Re-analyze** any time to re-index.

---

## Prerequisites

- Docker & Docker Compose
- AWS credentials with Bedrock model access in **us-east-1**
  - `amazon.nova-lite-v1:0` — fact extraction + answer synthesis
  - `amazon.nova-2-multimodal-embeddings-v1:0` — embeddings

---

## Quickstart

```bash
# 1. Clone
git clone https://github.com/Enas-Ijaabo/novalore
cd novalore

# 2. Set up credentials
cp .env.example .env
# Fill in AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY in .env

# 3. Start everything
docker compose up --build

# 4. Open http://localhost:3000
```

The Ingest tab auto-starts indexing on startup. Switch to **Knowledge** to browse extracted facts, or **Ask** to query in natural language.

---

## Architecture

| Component | Tech |
|---|---|
| Backend API | Go, net/http |
| Fact extraction + synthesis | Amazon Nova Lite (`amazon.nova-lite-v1:0`) |
| Embeddings | Amazon Nova Multimodal Embeddings (1024-dim) |
| Vector store | ChromaDB 0.4.24 |
| Frontend | Next.js App Router + Tailwind CSS |
| Orchestration | Docker Compose |

---

## API

| Method | Path | Description |
|---|---|---|
| `GET`  | `/api/ingest/status` | Per-file status `{running, total, files[]}` |
| `POST` | `/api/ingest` | Trigger re-analysis → `202` |
| `GET`  | `/api/facts` | All extracted knowledge statements |
| `POST` | `/api/query` | `{"q": "..."}` → `{answer, sources}` |
| `GET`  | `/api/health` | Health check |

---

## Dataset

The bundled dataset is a sample multi-service backend:

```
dataset/
  docs/
    architecture_overview.txt
    auth_design.txt
    meeting_notes.txt
  repos/
    auth-service/
    payment-service/
    api-gateway/
```

To index your own project: replace the contents of `dataset/` with your code and docs, then hit **Re-analyze**. NovaLore picks up `.go`, `.py`, `.ts`, `.js`, `.md`, `.txt`, `.yaml`, `.toml`, and `.json` files up to 50 KB each, and skips `node_modules`, `vendor`, `.git`, and `build` directories automatically.

---

## Verifying it works

1. **Credentials first** — copy `.env.example` to `.env`, set `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY`, then run `docker compose up --build` so the backend can call Bedrock.
2. **Watch ingest** — open http://localhost:3000/ingest. Indexing starts automatically; watch files move from `extracting → indexing → done`. All files should reach `done` within a few minutes.
3. **Browse knowledge** — head to `/knowledge`. Facts appear grouped by type (business rules, behaviors, constraints, etc.). Use the search bar or type filter to explore.
4. **Ask questions** — go to `/ask` and try:
   - *"How does authentication work?"*
   - *"What are the payment limits?"*
   - *"How is traffic routed between services?"*
   - Each answer should cite specific source files.
5. **API smoke test** — from another terminal:
```bash
curl http://localhost:8080/api/health
# → {"status":"ok"}

curl http://localhost:8080/api/facts | python3 -m json.tool | head -20
# → JSON array of extracted knowledge statements

curl -X POST http://localhost:8080/api/query \
  -H 'Content-Type: application/json' \
  -d '{"q": "how does authentication work?"}' | python3 -m json.tool
# → {"answer": "...", "sources": [...]}
```

---

## Development (without Docker)

```bash
# Terminal 1 — ChromaDB
docker run -p 8001:8000 chromadb/chroma:0.4.24

# Terminal 2 — Backend
cd backend
export $(grep -v '^#' ../.env | xargs) && go run ./cmd/server

# Terminal 3 — Frontend
cd frontend
npm install && npm run dev
```
