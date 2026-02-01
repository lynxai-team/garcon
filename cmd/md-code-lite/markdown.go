package main

import (
	"path/filepath"
	"strings"
)

// detectSourceLanguage returns the programming language based on the filename extension.
func detectSourceLanguage(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	// Programming languages
	case ".go":
		return "go"
	case ".js":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".py":
		return "python"
	case ".java":
		return "java"
	case ".c":
		return "c"
	case ".cpp", ".cxx":
		return "cpp"
	case ".cs":
		return "csharp"
	case ".php":
		return "php"
	case ".rb":
		return "ruby"
	case ".rs":
		return "rust"
	case ".swift":
		return "swift"
	case ".kt":
		return "kotlin"
	case ".scala":
		return "scala"
	case ".pl":
		return "perl"
	case ".lua":
		return "lua"
	case ".r":
		return "r"

	// Web technologies
	case ".html", ".htm":
		return "html"
	case ".css":
		return "css"
	case ".xml":
		return "xml"
	case ".json":
		return "json"
	case ".yml", ".yaml":
		return "yaml"
	case ".toml":
		return "toml"

	// Shell/scripting
	case ".sh":
		return "bash"
	case ".ps1":
		return "powershell"
	case ".bat", ".cmd":
		return "batch"
	case ".fish":
		return "fish"
	case ".zsh":
		return "zsh"

	// Database
	case ".sql":
		return "sql"

	// Markup/config
	case ".md":
		return "markdown"
	case ".tex":
		return "latex"
	case ".ini":
		return "ini"
	case ".properties":
		return "properties"
	case ".dockerfile":
		return "dockerfile"
	case ".makefile":
		return "makefile"
	case ".gitignore":
		return "gitignore"

	// Data formats
	case ".csv":
		return "csv"
	case ".jsonl":
		return "jsonl"
	case ".tsv":
		return "tsv"

	// Other formats
	case ".txt":
		return "text"
	case ".diff":
		return "diff"
	case ".log":
		return "log"
	case ".conf":
		return "conf"

	default:
		return ""
	}
}

// determineFileExtension returns the appropriate file extension for a given programming language.
func determineFileExtension(language string) string {
	switch strings.ToLower(language) {
	// Programming languages
	case "go":
		return ".go"
	case "javascript", "js":
		return ".js"
	case "typescript", "ts":
		return ".ts"
	case "python", "py":
		return ".py"
	case "java":
		return ".java"
	case "c":
		return ".c"
	case "cpp", "c++", "cxx":
		return ".cpp"
	case "csharp", "c#", "cs":
		return ".cs"
	case "php":
		return ".php"
	case "ruby", "rb":
		return ".rb"
	case "rust", "rs":
		return ".rs"
	case "swift":
		return ".swift"
	case "kotlin", "kt":
		return ".kt"
	case "scala":
		return ".scala"
	case "perl":
		return ".pl"
	case "lua":
		return ".lua"
	case "r":
		return ".r"

	// Web technologies
	case "html":
		return ".html"
	case "css":
		return ".css"
	case "xml":
		return ".xml"
	case "json":
		return ".json"
	case "yaml", "yml":
		return ".yml"
	case "toml":
		return ".toml"

	// Shell/scripting
	case "bash", "shell", "sh":
		return ".sh"
	case "powershell", "ps1":
		return ".ps1"
	case "batch", "cmd", "bat":
		return ".bat"
	case "fish":
		return ".fish"
	case "zsh":
		return ".zsh"

	// Database
	case "sql", "mysql", "postgresql", "postgres", "sqlite":
		return ".sql"

	// Markup/config
	case "markdown", "md":
		return ".md"
	case "latex", "tex":
		return ".tex"
	case "ini":
		return ".ini"
	case "properties":
		return ".properties"
	case "dockerfile", "docker":
		return ".dockerfile"
	case "makefile", "make":
		return ".makefile"
	case "gitignore":
		return ".gitignore"

	// Data formats
	case "csv":
		return ".csv"
	case "jsonl":
		return ".jsonl"
	case "tsv":
		return ".tsv"

	// Other formats
	case "text", "txt", "plain":
		return ".txt"
	case "diff":
		return ".diff"
	case "log":
		return ".log"
	case "conf", "config":
		return ".conf"

	default:
		return ""
	}
}
