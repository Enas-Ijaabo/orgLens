package reader

import (
	"os"
	"path/filepath"
	"strings"
)

const maxFileSize = 50 * 1024 // 50KB

var allowedExtensions = map[string]bool{
	".md": true, ".txt": true, ".go": true, ".py": true,
	".js": true, ".ts": true, ".yaml": true, ".yml": true,
	".toml": true, ".json": true,
}

var ignoredDirs = map[string]bool{
	"node_modules": true, "vendor": true, "dist": true,
	"build": true, "bin": true, ".git": true,
}

// Chunk represents a piece of text from a source file.
type Chunk struct {
	Text   string
	Source string
}

// ReadDataset reads all docs and repo files, returning text chunks split by paragraph.
func ReadDataset(datasetDir string) ([]Chunk, error) {
	var chunks []Chunk

	docsDir := filepath.Join(datasetDir, "docs")
	reposDir := filepath.Join(datasetDir, "repos")

	if entries, err := os.ReadDir(docsDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			path := filepath.Join(docsDir, e.Name())
			c, err := readFile(path)
			if err != nil {
				continue
			}
			chunks = append(chunks, c...)
		}
	}

	if err := filepath.WalkDir(reposDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if ignoredDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !allowedExtensions[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		c, err := readFile(path)
		if err != nil {
			return nil
		}
		chunks = append(chunks, c...)
		return nil
	}); err != nil {
		return nil, err
	}

	return chunks, nil
}

func readFile(path string) ([]Chunk, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > maxFileSize {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var chunks []Chunk
	for _, para := range strings.Split(string(data), "\n\n") {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		chunks = append(chunks, Chunk{Text: para, Source: path})
	}
	return chunks, nil
}
