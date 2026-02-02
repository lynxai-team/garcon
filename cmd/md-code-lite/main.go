package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func main() {
	cmd := CommandMdCodeLite()

	if err := cmd.Execute(); err != nil {
		cmd.PrintErr(err)
		os.Exit(1)
	}
}

func CommandMdCodeLite() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "md-code-lite",
		Short: "A lightweight tool to convert between markdown and source files",
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
	}

	cmd.AddCommand(CommandToCode())
	cmd.AddCommand(CommandToMarkdown())

	return cmd
}

func CommandToCode() *cobra.Command {
	var mdFile string
	var outputDir string

	cmd := &cobra.Command{
		Use:     "tocode",
		Short:   "Convert markdown file to source files",
		Example: `md-code-lite tocode -i docs.md -o src`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if mdFile == "" {
				return fmt.Errorf("markdown file is required (use --input or -i)")
			}
			if outputDir == "" {
				return fmt.Errorf("output directory is required (use --output or -o)")
			}

			cmd.Printf("Parsing markdown file: %s\n", mdFile)
			doc, err := FromMarkdown(mdFile)
			if err != nil {
				return fmt.Errorf("failed to parse markdown: %w", err)
			}

			cmd.Printf("Extracting %d code blocks to: %s\n", len(doc.Blocks), outputDir)
			err = doc.ToSourceFiles(outputDir)
			if err != nil {
				return fmt.Errorf("failed to extract files: %w", err)
			}

			cmd.Printf("Successfully extracted:")
			for _, block := range doc.Blocks {
				cmd.Printf("  - %s (%s)\n", block.Filename, block.Language)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&mdFile, "input", "i", "", "Input markdown file to extract from")
	cmd.MarkFlagRequired("input")

	cmd.Flags().StringVarP(&outputDir, "output", "o", "", "Output directory for extracted files")
	cmd.MarkFlagRequired("output")

	return cmd
}

func CommandToMarkdown() *cobra.Command {
	var outputFile string
	var directory string
	var filesFlag []string

	cmd := &cobra.Command{
		Use:     "tomd",
		Short:   "Convert source files to markdown",
		Example: `md-code-lite tomd -f file1 -f file2 -o output.md`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if outputFile == "" {
				return fmt.Errorf("output file is required (use --output or -o)")
			}

			var doc *Document
			var err error

			if directory == "" && len(filesFlag) == 0 {
				return fmt.Errorf("must specify either --dir or --files (or both)")
			}

			var allFiles []string

			if directory != "" {
				cmd.Printf("Scanning directory: %s\n", directory)
				dirFiles, err := getFilesFromDirectory(directory)
				if err != nil {
					return fmt.Errorf("failed to scan directory: %w", err)
				}
				allFiles = append(allFiles, dirFiles...)
			}

			if len(filesFlag) > 0 {
				for _, file := range filesFlag {
					allFiles = append(allFiles, strings.TrimSpace(file))
				}
			}

			if len(allFiles) == 0 {
				return fmt.Errorf("no files found to process")
			}

			cmd.Printf("Parsing %d files\n", len(allFiles))
			doc, err = FromSourceFilesList(allFiles...)
			if err != nil {
				return fmt.Errorf("failed to parse files: %w", err)
			}

			cmd.Printf("Generating markdown with %d code blocks: %s\n", len(doc.Blocks), outputFile)
			err = doc.ToMarkdown(outputFile)
			if err != nil {
				return fmt.Errorf("failed to generate markdown: %w", err)
			}

			cmd.Printf("Successfully generated markdown with:")
			for _, block := range doc.Blocks {
				cmd.Printf("  - %s (%s)\n", block.Filename, block.Language)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output markdown file")
	cmd.MarkFlagRequired("output")

	cmd.Flags().StringVarP(&directory, "dir", "d", "", "Directory to scan for source files")
	cmd.Flags().StringSliceVarP(&filesFlag, "files", "f", nil, "Files to include")

	return cmd
}

// Default patterns to skip (gitignore-style)
var defaultSkipPatterns = []string{
	// Directories
	".git/",
	"node_modules/",
	"vendor/",
	"dist/",
	"build/",
	".next/",
	"target/",
	"bin/",
	"obj/",
	".*", // Hidden directories (except current dir)
	// Files
	"*.exe",
	"*.dll",
	"*.so",
	"*.dylib",
	"*.o",
	"*.obj",
	"*.log",
	"package-lock.json",
	"yarn.lock",
	"Cargo.lock",
	".*", // Hidden files
}

// shouldSkip checks if a path matches any skip pattern
func shouldSkip(path string, isDir bool, patterns []string) bool {
	name := filepath.Base(path)

	for _, pattern := range patterns {
		// Handle directory patterns (ending with /)
		if strings.HasSuffix(pattern, "/") {
			if !isDir {
				continue
			}
			dirPattern := strings.TrimSuffix(pattern, "/")
			if matched, _ := filepath.Match(dirPattern, name); matched {
				return true
			}
		} else {
			// Handle file patterns
			if matched, _ := filepath.Match(pattern, name); matched {
				// Special case: don't skip current directory
				if pattern == ".*" && name == "." {
					continue
				}
				return true
			}
		}
	}
	return false
}

// getFilesFromDirectory returns all source files in a directory
func getFilesFromDirectory(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Check if we should skip this path
		if shouldSkip(path, info.IsDir(), defaultSkipPatterns) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Only add files, not directories
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}
