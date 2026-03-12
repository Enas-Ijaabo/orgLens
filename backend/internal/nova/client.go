package nova

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
)

const modelID = "amazon.nova-lite-v1:0"

type Client struct {
	bedrock *bedrockruntime.Client
}

func NewClient(ctx context.Context) (*Client, error) {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "us-east-1"
	}
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	return &Client{bedrock: bedrockruntime.NewFromConfig(cfg)}, nil
}

func (c *Client) Invoke(ctx context.Context, prompt string) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"messages": []map[string]any{
			{"role": "user", "content": []map[string]any{{"text": prompt}}},
		},
		"inferenceConfig": map[string]any{"maxTokens": 512, "temperature": 0},
	})

	out, err := c.bedrock.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(modelID),
		ContentType: aws.String("application/json"),
		Body:        body,
	})
	if err != nil {
		return "", fmt.Errorf("invoke model: %w", err)
	}

	var resp struct {
		Output struct {
			Message struct {
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"message"`
		} `json:"output"`
	}
	if err := json.Unmarshal(out.Body, &resp); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if len(resp.Output.Message.Content) == 0 {
		return "", fmt.Errorf("empty response")
	}
	return resp.Output.Message.Content[0].Text, nil
}
