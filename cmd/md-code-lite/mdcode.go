package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Document represents a collection of code blocks that can be converted
// between markdown and source files.
type Document struct {
	Blocks []CodeBlock
}

// CodeBlock represents a single code block with its metadata.
type CodeBlock struct {
	Filename string
	Language string
	Content  string
}

// FromMarkdown reads a markdown file and returns a Document with extracted code blocks.
func FromMarkdown(markdownPath string) (*Document, error) {
	file, err := os.Open(markdownPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var doc Document
	scanner := bufio.NewScanner(file)

	var inCodeBlock bool
	var language string
	var content strings.Builder
	var lastHeaderFilename string
	fileCounter := 0

	for scanner.Scan() {
		line := scanner.Text()

		if inCodeBlock {
			if strings.HasPrefix(strings.TrimSpace(line), "```") {
				fileCounter++
				filename := fmt.Sprintf("file%d", fileCounter)

				if lastHeaderFilename != "" && language != "" {
					headerExt := filepath.Ext(lastHeaderFilename)
					expectedExt := determineFileExtension(language)
					if headerExt == expectedExt {
						filename = lastHeaderFilename
					}
				} else if language != "" {
					ext := determineFileExtension(language)
					if ext != "" {
						filename = fmt.Sprintf("file%d%s", fileCounter, ext)
					}
				}

				doc.Blocks = append(doc.Blocks, CodeBlock{
					Filename: filename,
					Language: language,
					Content:  content.String(),
				})

				inCodeBlock = false
				language = ""
				content.Reset()
				lastHeaderFilename = ""
				continue
			}
			content.WriteString(line + "\n")
			continue
		}

		switch line := strings.TrimSpace(line); {
		case strings.HasPrefix(line, "```"):
			inCodeBlock = true
			language = strings.TrimSpace(line[3:])
		case strings.HasPrefix(line, "## "):
			headerText := strings.TrimSpace(line[3:])
			if headerText != "" {
				lastHeaderFilename = headerText
			}
		case line == "":
			continue
		default:
			lastHeaderFilename = ""
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	return &doc, nil
}

// FromSourceFilesList reads a list of source files and returns a Document.
func FromSourceFilesList(filePaths ...string) (*Document, error) {
	var doc Document

	for _, filePath := range filePaths {
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			return nil, fmt.Errorf("file does not exist: %s", filePath)
		}

		filename := filepath.Base(filePath)
		language := detectSourceLanguage(filename)
		if language == "" {
			continue
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
		}

		block := CodeBlock{
			Filename: filePath,
			Language: language,
			Content:  string(content),
		}

		doc.Blocks = append(doc.Blocks, block)
	}

	return &doc, nil
}

// ToMarkdown writes the Document as a markdown file.
func (d *Document) ToMarkdown(outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	if _, err := file.WriteString("# Code Files\n\n"); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	for i, block := range d.Blocks {
		if i > 0 {
			if _, err := file.WriteString("\n"); err != nil {
				return fmt.Errorf("failed to write spacing: %w", err)
			}
		}

		if _, err := file.WriteString(fmt.Sprintf("## %s\n\n", block.Filename)); err != nil {
			return fmt.Errorf("failed to write filename header: %w", err)
		}

		if _, err := file.WriteString(fmt.Sprintf("```%s\n", block.Language)); err != nil {
			return fmt.Errorf("failed to write code block start: %w", err)
		}

		if _, err := file.WriteString(fmt.Sprintf("%s```\n", block.Content)); err != nil {
			return fmt.Errorf("failed to write code block: %w", err)
		}
	}

	return nil
}

// ToSourceFiles writes the Document as individual source files.
func (d *Document) ToSourceFiles(outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	for _, block := range d.Blocks {
		fullPath := filepath.Join(outputDir, block.Filename)

		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}

		if err := os.WriteFile(fullPath, []byte(block.Content), 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", fullPath, err)
		}
	}

	return nil
}
