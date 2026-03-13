package nova

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/enas/orglens/internal/pipeline"
)

var validTypes = map[string]bool{
	"business_rule": true,
	"architecture":  true,
	"data_rule":     true,
	"behavior":      true,
	"constraint":    true,
	"decision":      true,
}

const extractionPrompt = `You are an expert software engineer. Extract factual knowledge statements that are EXPLICITLY present in the text below.

Extract only:
- Business rules (e.g. account limits, approval thresholds)
- Domain constraints (e.g. numeric limits, time windows, size caps)
- System behaviors (e.g. what happens on timeout, how retries work)
- Architectural decisions (e.g. which service owns what, how traffic flows)
- Data rules (e.g. where data is stored, retention policies)

Strict rules:
- Only extract facts that are EXPLICITLY stated in the text — do not infer or invent
- Every number, name, and condition must come directly from the text
- Skip generic boilerplate, imports, variable declarations with no business meaning
- If the text contains no meaningful facts, return []
- Do not use prior knowledge or information from other files. Only use the text provided.

Output: a JSON array of objects with "fact" (complete English sentence) and "type".
Valid types: business_rule, architecture, data_rule, behavior, constraint, decision

Text to extract from:
{text}

Return ONLY a valid JSON array. No explanation. No markdown.`

func (c *Client) ExtractFacts(ctx context.Context, text, source string) ([]pipeline.Fact, error) {
	prompt := strings.ReplaceAll(extractionPrompt, "{text}", text)

	raw, err := c.Invoke(ctx, prompt)
	if err != nil {
		return nil, err
	}

	raw = extractJSON(raw)
	if raw == "" {
		log.Printf("no JSON array found in model response\nraw: %s", raw)
		return []pipeline.Fact{}, nil
	}

	var items []struct {
		Fact string `json:"fact"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil, fmt.Errorf("parse facts json: %w\nraw: %s", err, raw)
	}

	seen := map[string]bool{}
	facts := make([]pipeline.Fact, 0, len(items))
	for _, item := range items {
		item.Fact = strings.TrimSpace(item.Fact)
		item.Type = strings.TrimSpace(item.Type)

		if len(item.Fact) < 10 || len(item.Fact) > 200 {
			continue
		}
		if seen[item.Fact] {
			continue
		}
		seen[item.Fact] = true

		factType := item.Type
		if !validTypes[factType] {
			factType = "architecture"
		}

		facts = append(facts, pipeline.Fact{
			Text:   item.Fact,
			Type:   factType,
			Source: source,
		})
	}
	return facts, nil
}

// extractJSON finds the outermost JSON array in the model response,
// handling markdown code fences without panicking.
func extractJSON(raw string) string {
	start := strings.Index(raw, "[")
	end := strings.LastIndex(raw, "]")
	if start == -1 || end == -1 || end < start {
		return ""
	}
	return strings.TrimSpace(raw[start : end+1])
}
