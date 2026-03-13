package pipeline

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Chunk is a logical unit of content ready to be sent to Nova for extraction.
type Chunk struct {
	Content   string
	Source    string
	StartLine int
	EndLine   int
}

var (
	codeExts = map[string]bool{
		".go": true, ".py": true, ".js": true, ".ts": true,
	}
	constVarBlockRe = regexp.MustCompile(`^(const|var)\s*\(`)
	goFuncRe        = regexp.MustCompile(`^func[\s(]`)
	pyFuncRe        = regexp.MustCompile(`^(async\s+)?def\s+`)
	jsFuncRe        = regexp.MustCompile(`^(export\s+)?(default\s+)?(async\s+)?function\s+`)
)

// ChunkFile reads a file and returns logical chunks with source line metadata.
func ChunkFile(path string) ([]Chunk, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content := string(data)
	ext := strings.ToLower(filepath.Ext(path))
	if codeExts[ext] {
		return chunkCode(content, path, ext), nil
	}
	return chunkDocs(content, path), nil
}

func chunkDocs(content, source string) []Chunk {
	var raw []Chunk
	lineNum := 1
	for _, para := range strings.Split(content, "\n\n") {
		paraLines := strings.Count(para, "\n") + 1
		trimmed := strings.TrimSpace(para)
		if trimmed != "" {
			raw = append(raw, Chunk{
				Content:   trimmed,
				Source:    source,
				StartLine: lineNum,
				EndLine:   lineNum + paraLines - 1,
			})
		}
		lineNum += paraLines + 1 // +1 for the blank separator line
	}

	// Merge title-only chunks (single line) with the following chunk.
	var chunks []Chunk
	for i := 0; i < len(raw); i++ {
		if !strings.Contains(raw[i].Content, "\n") && i+1 < len(raw) {
			merged := raw[i]
			merged.Content = raw[i].Content + "\n\n" + raw[i+1].Content
			merged.EndLine = raw[i+1].EndLine
			chunks = append(chunks, merged)
			i++ // skip next
		} else {
			chunks = append(chunks, raw[i])
		}
	}
	return chunks
}

func chunkCode(content, source, ext string) []Chunk {
	lines := strings.Split(content, "\n")
	fileContext := extractFileContext(lines)

	// Small files: one chunk = entire file
	if len(lines) < 200 {
		return []Chunk{{
			Content:   content,
			Source:    source,
			StartLine: 1,
			EndLine:   len(lines),
		}}
	}

	funcRe := funcRegex(ext)
	if funcRe == nil {
		return []Chunk{{
			Content:   content,
			Source:    source,
			StartLine: 1,
			EndLine:   len(lines),
		}}
	}

	// Identify top-level function boundary positions
	type span struct{ start, end int }
	var spans []span
	funcStart := -1

	for i, line := range lines {
		if funcRe.MatchString(line) {
			if funcStart >= 0 {
				spans = append(spans, span{funcStart, i - 1})
			}
			funcStart = i
		}
	}
	if funcStart >= 0 {
		spans = append(spans, span{funcStart, len(lines) - 1})
	}

	// No functions found: return whole file
	if len(spans) == 0 {
		return []Chunk{{
			Content:   content,
			Source:    source,
			StartLine: 1,
			EndLine:   len(lines),
		}}
	}

	chunks := make([]Chunk, 0, len(spans))
	for _, s := range spans {
		funcBody := strings.Join(lines[s.start:s.end+1], "\n")
		chunkContent := funcBody
		if fileContext != "" {
			chunkContent = fileContext + "\n" + funcBody
		}
		chunks = append(chunks, Chunk{
			Content:   chunkContent,
			Source:    source,
			StartLine: s.start + 1,
			EndLine:   s.end + 1,
		})
	}
	return chunks
}

// extractFileContext collects top-level const(...) and var(...) blocks.
// These carry the actual business rule values (e.g. TokenExpiry = 24h).
func extractFileContext(lines []string) string {
	var result []string
	i := 0
	for i < len(lines) {
		if constVarBlockRe.MatchString(lines[i]) {
			depth := 0
			for i < len(lines) {
				result = append(result, lines[i])
				depth += strings.Count(lines[i], "(") - strings.Count(lines[i], ")")
				i++
				if depth <= 0 {
					break
				}
			}
		} else {
			i++
		}
	}
	return strings.Join(result, "\n")
}

func funcRegex(ext string) *regexp.Regexp {
	switch ext {
	case ".go":
		return goFuncRe
	case ".py":
		return pyFuncRe
	case ".js", ".ts":
		return jsFuncRe
	}
	return nil
}
