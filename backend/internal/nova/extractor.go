package nova

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/enas/orglens/internal/pipeline"
)

var validRelations = map[string]bool{
	"uses": true, "calls": true, "writes_to": true, "reads_from": true,
	"depends_on": true, "handles": true, "stores": true, "routes_to": true,
	"expires_after": true, "communicates_with": true,
}

const extractionPrompt = `You are a software architect extracting structured knowledge from engineering documents and source code.

Extract ALL factual relationships as subject-relation-object triples.

Use ONLY these relations:
uses, calls, writes_to, reads_from, depends_on, handles,
stores, routes_to, expires_after, communicates_with

Rules:
- Normalize entity names to PascalCase service names or kebab-case package names
- Only extract explicitly stated facts, never infer
- One triple per factual claim
- For code: import statements → depends_on, function calls → calls, DB writes → writes_to, DB reads → reads_from

Example 1 (prose):
Text: "UserService stores session data in Redis. It reads user profiles from PostgreSQL."
Output:
[
  {"subject": "UserService", "relation": "stores", "object": "Redis"},
  {"subject": "UserService", "relation": "reads_from", "object": "PostgreSQL"}
]

Example 2 (code):
Text: ` + "`" + `import "github.com/org/mailer"
func (s *NotificationService) Notify(evt Event) {
    s.mailer.Send(evt.Email)
}` + "`" + `
Output:
[
  {"subject": "NotificationService", "relation": "depends_on", "object": "mailer"},
  {"subject": "NotificationService", "relation": "calls", "object": "mailer.Send"}
]

Now extract from this text:
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
		return nil, fmt.Errorf("no JSON array found in model response")
	}

	var triples []struct {
		Subject  string `json:"subject"`
		Relation string `json:"relation"`
		Object   string `json:"object"`
	}
	if err := json.Unmarshal([]byte(raw), &triples); err != nil {
		return nil, fmt.Errorf("parse facts json: %w\nraw: %s", err, raw)
	}

	facts := make([]pipeline.Fact, 0, len(triples))
	for _, t := range triples {
		t.Subject = strings.TrimSpace(t.Subject)
		t.Relation = strings.TrimSpace(t.Relation)
		t.Object = strings.TrimSpace(t.Object)

		if t.Subject == "" || t.Relation == "" || t.Object == "" {
			continue
		}
		if !validRelations[t.Relation] {
			continue
		}
		facts = append(facts, pipeline.Fact{
			Subject:  t.Subject,
			Relation: t.Relation,
			Object:   t.Object,
			Text:     t.Subject + " " + t.Relation + " " + t.Object,
			Source:   source,
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
