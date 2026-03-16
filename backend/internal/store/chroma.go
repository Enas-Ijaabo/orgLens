package store

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/enas/novalore/internal/pipeline"
	"github.com/google/uuid"
)

const collectionName = "novalore_knowledge"
const metaCollectionName = "novalore_meta"

type Client struct {
	base             string
	collectionID     string
	metaCollectionID string
	httpClient       *http.Client
}

// FileMeta holds per-file ingestion state stored in novalore_meta.
type FileMeta struct {
	File       string    `json:"file"`
	IngestedAt time.Time `json:"ingested_at"`
	FactsCount int       `json:"facts_count"`
}

func NewClient() *Client {
	base := os.Getenv("CHROMA_URL")
	if base == "" {
		base = "http://localhost:8001"
	}
	return &Client{base: base, httpClient: &http.Client{}}
}

// EnsureCollection creates the collection if it does not already exist, then fetches its UUID.
func (c *Client) EnsureCollection(ctx context.Context) error {
	body, _ := json.Marshal(map[string]any{
		"name": collectionName,
	})
	resp, err := c.post(ctx, "/api/v1/collections", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)
	ok := resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusConflict
	if !ok && resp.StatusCode == http.StatusInternalServerError {
		ok = strings.Contains(string(b), "already exists")
	}
	if !ok {
		return fmt.Errorf("ensure collection: unexpected status %d", resp.StatusCode)
	}

	return c.fetchCollectionID(ctx)
}

// fetchCollectionID fetches and stores the UUID for the collection.
func (c *Client) fetchCollectionID(ctx context.Context) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		c.base+"/api/v1/collections/"+collectionName, nil)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetch collection id: %w", err)
	}
	defer resp.Body.Close()

	var col struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&col); err != nil {
		return fmt.Errorf("fetch collection id decode: %w", err)
	}
	if col.ID == "" {
		return fmt.Errorf("fetch collection id: empty id")
	}
	c.collectionID = col.ID
	return nil
}

func (c *Client) colPath(suffix string) string {
	return "/api/v1/collections/" + c.collectionID + suffix
}

// Add stores facts with their embeddings. IDs are generated if empty.
func (c *Client) Add(ctx context.Context, facts []pipeline.Fact, embeddings [][]float64) error {
	if len(facts) == 0 {
		return nil
	}

	ids := make([]string, len(facts))
	documents := make([]string, len(facts))
	metadatas := make([]map[string]string, len(facts))

	for i, f := range facts {
		id := f.ID
		if id == "" {
			id = uuid.NewString()
		}
		ids[i] = id
		documents[i] = f.Text
		metadatas[i] = map[string]string{
			"source": f.Source,
			"type":   f.Type,
		}
	}

	body, _ := json.Marshal(map[string]any{
		"ids":        ids,
		"documents":  documents,
		"embeddings": embeddings,
		"metadatas":  metadatas,
	})

	resp, err := c.post(ctx, c.colPath("/add"), body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("add: unexpected status %d", resp.StatusCode)
	}
	return nil
}

// Query returns the top n facts closest to the given embedding vector.
func (c *Client) Query(ctx context.Context, embedding []float64, n int) ([]pipeline.Fact, error) {
	body, _ := json.Marshal(map[string]any{
		"query_embeddings": [][]float64{embedding},
		"n_results":        n,
		"include":          []string{"documents", "metadatas"},
	})

	resp, err := c.post(ctx, c.colPath("/query"), body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		IDs       [][]string            `json:"ids"`
		Documents [][]string            `json:"documents"`
		Metadatas [][]map[string]string `json:"metadatas"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("query decode: %w", err)
	}

	if len(result.IDs) == 0 {
		return nil, nil
	}

	facts := make([]pipeline.Fact, len(result.IDs[0]))
	for i, id := range result.IDs[0] {
		facts[i] = pipeline.Fact{
			ID:   id,
			Text: result.Documents[0][i],
		}
		if len(result.Metadatas[0]) > i {
			facts[i].Source = result.Metadatas[0][i]["source"]
			facts[i].Type = result.Metadatas[0][i]["type"]
		}
	}
	return facts, nil
}

// GetAll returns every fact stored in the collection.
func (c *Client) GetAll(ctx context.Context) ([]pipeline.Fact, error) {
	body, _ := json.Marshal(map[string]any{
		"include": []string{"documents", "metadatas"},
	})

	resp, err := c.post(ctx, c.colPath("/get"), body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		IDs       []string            `json:"ids"`
		Documents []string            `json:"documents"`
		Metadatas []map[string]string `json:"metadatas"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("getall decode: %w", err)
	}

	facts := make([]pipeline.Fact, len(result.IDs))
	for i, id := range result.IDs {
		facts[i] = pipeline.Fact{
			ID:   id,
			Text: result.Documents[i],
		}
		if i < len(result.Metadatas) {
			facts[i].Source = result.Metadatas[i]["source"]
			facts[i].Type = result.Metadatas[i]["type"]
		}
	}
	return facts, nil
}

// Count returns the number of facts stored in the collection.
func (c *Client) Count(ctx context.Context) (int, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		c.base+c.colPath("/count"), nil)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("count: %w", err)
	}
	defer resp.Body.Close()

	var n int
	if err := json.NewDecoder(resp.Body).Decode(&n); err != nil {
		return 0, fmt.Errorf("count decode: %w", err)
	}
	return n, nil
}

// Reset deletes and recreates the collection.
func (c *Client) Reset(ctx context.Context) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodDelete,
		c.base+"/api/v1/collections/"+collectionName, nil)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("reset delete: %w", err)
	}
	resp.Body.Close()
	// 404 is fine — collection didn't exist
	return c.EnsureCollection(ctx)
}

// EnsureMetaCollection creates novalore_meta if it does not exist, then fetches its UUID.
func (c *Client) EnsureMetaCollection(ctx context.Context) error {
	body, _ := json.Marshal(map[string]any{"name": metaCollectionName})
	resp, err := c.post(ctx, "/api/v1/collections", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	ok := resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusConflict
	if !ok && resp.StatusCode == http.StatusInternalServerError {
		ok = strings.Contains(string(b), "already exists")
	}
	if !ok {
		return fmt.Errorf("ensure meta collection: unexpected status %d", resp.StatusCode)
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		c.base+"/api/v1/collections/"+metaCollectionName, nil)
	res, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetch meta collection id: %w", err)
	}
	defer res.Body.Close()
	var col struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&col); err != nil || col.ID == "" {
		return fmt.Errorf("fetch meta collection id: bad response")
	}
	c.metaCollectionID = col.ID
	return nil
}

func (c *Client) metaColPath(suffix string) string {
	return "/api/v1/collections/" + c.metaCollectionID + suffix
}

// WriteFileMeta upserts a per-file ingestion record into novalore_meta.
func (c *Client) WriteFileMeta(ctx context.Context, file string, ingestedAt time.Time, factsCount int) error {
	body, _ := json.Marshal(map[string]any{
		"ids":        []string{file},
		"documents":  []string{file},
		"embeddings": [][]float64{{0.0}},
		"metadatas":  []map[string]any{{"ingested_at": ingestedAt.Format(time.RFC3339), "facts_count": factsCount}},
	})
	resp, err := c.post(ctx, c.metaColPath("/upsert"), body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("write file meta: status %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

// GetAllMeta returns all per-file ingestion records from novalore_meta.
func (c *Client) GetAllMeta(ctx context.Context) ([]FileMeta, error) {
	body, _ := json.Marshal(map[string]any{
		"include": []string{"documents", "metadatas"},
	})
	resp, err := c.post(ctx, c.metaColPath("/get"), body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result struct {
		IDs       []string         `json:"ids"`
		Metadatas []map[string]any `json:"metadatas"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("getallmeta decode: %w", err)
	}
	metas := make([]FileMeta, len(result.IDs))
	for i, id := range result.IDs {
		metas[i].File = id
		if i < len(result.Metadatas) {
			m := result.Metadatas[i]
			if v, ok := m["ingested_at"].(string); ok {
				t, _ := time.Parse(time.RFC3339, v)
				metas[i].IngestedAt = t
			}
			if v, ok := m["facts_count"].(float64); ok {
				metas[i].FactsCount = int(v)
			}
		}
	}
	return metas, nil
}

// DeleteBySource removes all facts whose metadata source equals the given value.
func (c *Client) DeleteBySource(ctx context.Context, source string) error {
	body, _ := json.Marshal(map[string]any{
		"where": map[string]any{"source": map[string]any{"$eq": source}},
	})
	resp, err := c.post(ctx, c.colPath("/delete"), body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete by source: status %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

// DeleteFileMeta removes the meta record for a single file.
func (c *Client) DeleteFileMeta(ctx context.Context, file string) error {
	body, _ := json.Marshal(map[string]any{"ids": []string{file}})
	resp, err := c.post(ctx, c.metaColPath("/delete"), body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete file meta: status %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

// ResetMeta deletes and recreates the novalore_meta collection.
func (c *Client) ResetMeta(ctx context.Context) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodDelete,
		c.base+"/api/v1/collections/"+metaCollectionName, nil)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("reset meta delete: %w", err)
	}
	resp.Body.Close()
	return c.EnsureMetaCollection(ctx)
}

func (c *Client) post(ctx context.Context, path string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.base+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("chroma %s: %w", path, err)
	}
	return resp, nil
}
