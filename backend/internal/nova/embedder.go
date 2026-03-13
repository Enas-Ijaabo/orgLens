package nova

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
)

const embedModelID = "amazon.nova-2-multimodal-embeddings-v1:0"

// Embed returns a 1024-dimensional vector for the given text.
func (c *Client) Embed(ctx context.Context, text string) ([]float64, error) {
	body, _ := json.Marshal(map[string]any{
		"taskType": "SINGLE_EMBEDDING",
		"singleEmbeddingParams": map[string]any{
			"embeddingPurpose":   "GENERIC_INDEX",
			"embeddingDimension": 1024,
			"text": map[string]any{
				"truncationMode": "END",
				"value":          text,
			},
		},
	})

	var out *bedrockruntime.InvokeModelOutput
	var err error
	for attempt, delay := 0, time.Second; attempt < 5; attempt++ {
		out, err = c.bedrock.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
			ModelId:     aws.String(embedModelID),
			ContentType: aws.String("application/json"),
			Body:        body,
		})
		if err == nil {
			break
		}
		if !strings.Contains(err.Error(), "ThrottlingException") {
			return nil, fmt.Errorf("embed: %w", err)
		}
		time.Sleep(delay)
		delay *= 2
	}
	if err != nil {
		return nil, fmt.Errorf("embed: %w", err)
	}

	var resp struct {
		Embeddings []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"embeddings"`
	}
	if err := json.Unmarshal(out.Body, &resp); err != nil {
		return nil, fmt.Errorf("embed parse: %w", err)
	}
	if len(resp.Embeddings) == 0 || len(resp.Embeddings[0].Embedding) == 0 {
		return nil, fmt.Errorf("embed: empty vector returned")
	}
	return resp.Embeddings[0].Embedding, nil
}
