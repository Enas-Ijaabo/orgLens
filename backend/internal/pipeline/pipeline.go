package pipeline

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var nonAlphanumRe = regexp.MustCompile(`[^a-z0-9\s]`)

// normalizeText lowercases, strips punctuation, and collapses whitespace.
// Used as a dedup key so near-identical facts with minor wording differences are collapsed.
func normalizeText(s string) string {
	s = strings.ToLower(s)
	s = nonAlphanumRe.ReplaceAllString(s, "")
	return strings.Join(strings.Fields(s), " ")
}

var (
	allowedExts = map[string]bool{
		".md": true, ".txt": true, ".go": true, ".py": true,
		".js": true, ".ts": true, ".yaml": true, ".yml": true,
		".toml": true, ".json": true,
	}
	ignoredDirs = map[string]bool{
		"node_modules": true, "vendor": true, "dist": true,
		"build": true, "bin": true, ".git": true,
	}
	maxFileBytes = int64(50 * 1024) // 50KB
)

// ExtractFunc is called per chunk to extract facts from text.
type ExtractFunc func(ctx context.Context, text, source string) ([]Fact, error)

// Run walks datasetDir, chunks every file, and extracts facts from each chunk.
// Deduplicates facts by exact text across the whole run.
func Run(ctx context.Context, datasetDir string, extract ExtractFunc) ([]Fact, error) {
	files, err := WalkDataset(datasetDir)
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	var allFacts []Fact

	for _, path := range files {
		relSource, _ := filepath.Rel(datasetDir, path)

		chunks, err := ChunkFile(path)
		if err != nil {
			log.Printf("chunk [%s]: %v", relSource, err)
			continue
		}
		for i := range chunks {
			chunks[i].Source = relSource
		}

		log.Printf("Reading file: %s — %d chunks", relSource, len(chunks))

		var fileFacts []Fact
		for _, chunk := range chunks {
			facts, err := extract(ctx, chunk.Content, chunk.Source)
			if err != nil {
				log.Printf("extract [%s]: %v", relSource, err)
				continue
			}
			fileFacts = append(fileFacts, facts...)
		}

		log.Printf("Extracted %d statements", len(fileFacts))

		for _, f := range fileFacts {
			key := normalizeText(f.Text)
			if !seen[key] {
				seen[key] = true
				allFacts = append(allFacts, f)
			}
		}
	}

	log.Printf("Done. %d facts extracted.", len(allFacts))
	return allFacts, nil
}

// WalkDataset returns all readable file paths under dir.
func WalkDataset(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if ignoredDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !allowedExts[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > maxFileBytes {
			return nil
		}
		files = append(files, path)
		return nil
	})
	return files, err
}
