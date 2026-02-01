package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarkdownToSourceFiles(t *testing.T) {
	doc, err := FromMarkdown("testdata/golden.md")
	require.NoError(t, err, "parse markdown file")

	tempDir := t.TempDir()
	err = doc.ToSourceFiles(tempDir)
	require.NoError(t, err, "generate source files")

	compareFiles := []struct {
		actual string
		wanted string
	}{
		{"main.go", "testdata/code/main.go"},
		{"helper.js", "testdata/code/helper.js"},
		{"file3.css", "testdata/code/main.css"},
	}

	for _, cf := range compareFiles {
		actualPath := filepath.Join(tempDir, cf.actual)
		actualContent, err := os.ReadFile(actualPath)
		require.NoError(t, err, "read file %s", actualPath)

		expectedContent, err := os.ReadFile(cf.wanted)
		require.NoError(t, err, "read file %s", cf.wanted)

		assert.Equal(t, string(expectedContent), string(actualContent), "compare file content")
	}
}

func TestSourceFilesToMarkdown(t *testing.T) {
	doc, err := FromSourceFilesList(
		"testdata/code/main.go",
		"testdata/code/helper.js",
		"testdata/code/main.css",
	)
	require.NoError(t, err, "parse source files")

	outputPath := filepath.Join(t.TempDir(), "generated.md")
	err = doc.ToMarkdown(outputPath)
	require.NoError(t, err, "generate markdown")

	actualContent, err := os.ReadFile(outputPath)
	require.NoError(t, err, "read generated markdown")

	wantedContent, err := os.ReadFile("testdata/golden-code.md")
	require.NoError(t, err, "read expected markdown")

	assert.Equal(t, string(wantedContent), string(actualContent), "generated markdown should match expected")
}
