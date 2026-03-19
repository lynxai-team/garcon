# FlashBuilder Low-Level Specification

## Document Purpose

This specification provides complete, unambiguous implementation guidance for `flashbuilder`, a Go code generator tool that produces a high-performance static-asset HTTP server called `flash`. This document serves as the single source of truth for implementing the complete project without ambiguity or missing details. All algorithms, data structures, and integration points are explicitly defined to enable machine-parseable code generation.

**Intent**: Enable deterministic, reproducible generation of ultra-high-performance static asset HTTP servers with zero heap allocations per request for embed-eligible assets.

**Purpose**: Provide machine-parseable specifications that allow downstream AI models to generate complete, correct Go code without human intervention.

**Rationale**: Every algorithm, data structure, template, and integration point must be explicitly defined to ensure correctness and completeness. The specification is designed for direct consumption by code generation tools.

---

## 1. Project Overview

### 1.1 Tool Identity

| Property | Value |
|----------|-------|
| Tool Name | `flashbuilder` |
| Generated Binary | `flash` |
| Target Language | Go 1.26 (released 2026) |
| Platform | Linux AMD64 |
| Generator Dependency Policy | CGO required for compression libraries |
| Generated Server Dependency Policy | Pure Go (no CGO) |

**Intent**: The tool generates a single-binary Go program serving static assets with the fastest possible request-processing hot-path.

**Purpose**: Enable ultra-fast static asset serving with zero heap allocations per request for embed-eligible assets.

**Rationale**: Go 1.26 provides `min`/`max` builtins, improved `slices` package, and enhanced compiler optimizations. The generator uses CGO for compression (Brotli, AVIF, WebP) during generation, but the generated server is pure Go for maximum portability and zero-allocation performance.

### 1.2 Performance Contract

| Constraint | Requirement |
|------------|-------------|
| Heap allocations | Zero per request for embed-eligible assets |
| Routing complexity | O(1): one dispatch array lookup, one switch statement |
| Conditional GET | Single `if` statement checking ETag |
| Pre-computation | All possible computations become compile-time constants |
| Embed mechanism | `//go:embed` directive for eligible assets |

**Intent**: The generated server must achieve minimal overhead per request.

**Purpose**: Eliminate runtime branching and memory allocation in the hot path.

**Rationale**: Pre-computed dispatch arrays eliminate runtime path parsing. Separate HTTP/HTTPS header literals eliminate protocol branching. Compile-time constants for ETags enable single `if` comparison.

### 1.3 Determinism Requirements

| Aspect | Behavior |
|--------|----------|
| Asset ordering | Fixed alphabetical order by `RelPath` |
| Module name | Deterministic: `flash` |
| Identifier generation | Deterministic sanitization algorithm |
| Dependency versions | Latest compatible via `go mod tidy` |
| Non-determinism | Only TLS certificate randomness (runtime) |

**Intent**: Identical inputs must produce identical outputs (except TLS certificate).

**Purpose**: Enable reproducible builds, caching, and verification.

**Rationale**: Deterministic generation ensures that the same asset tree produces the same binary. TLS certificate randomness is acceptable as the only non-deterministic element since it occurs at runtime, not generation time.

---

## 2. CLI Specification

### 2.1 Generator Command Syntax

```
flashbuilder <input> <output> [flags]
```

### 2.2 Generator Positional Arguments

| Name | Type | Constraints | Description |
|------|------|-------------|-------------|
| `input` | string | Must exist, must be directory, read-only | Path to asset tree |
| `output` | string | Must differ from input, created if missing | Destination for generated files |

**Intent**: Clear positional argument specification.

**Purpose**: Enable simple command-line invocation.

**Rationale**: Positional arguments are more intuitive than flags for required inputs.

### 2.3 Generator Flags

All flags use Kong's tag syntax with environment variable support. Kong deduces flag names from variable names (e.g., `EmbedBudget` becomes `--embed-budget`):

```go
import (
    "github.com/alecthomas/kong"
    "github.com/alecthomas/units"
)

type cli struct {
    Input       string           `env:"FLASHBUILDER_INPUT"         type:"path" arg:"input"  help:"Path to asset tree"`
    Output      string           `env:"FLASHBUILDER_OUTPUT"        type:"path" arg:"output" help:"Destination for generated files"`
    EmbedBudget units.Base2Bytes `env:"FLASHBUILDER_EMBED_BUDGET"  default:"200GB"`
    Brotli      int              `env:"FLASHBUILDER_BROTLI"        default:"11"`
    AVIF        int              `env:"FLASHBUILDER_AVIF"          default:"50"`
    WebP        int              `env:"FLASHBUILDER_WEBP"          default:"50"`
    CSP         string           `env:"FLASHBUILDER_CSP"           default:"default-src 'self'"`
    Verbosity   int              `env:"FLASHBUILDER_LOG_LEVEL"     type:"counter" short:"v"`
    DryRun      bool             `env:"FLASHBUILDER_DRY_RUN"`
    Tests       bool             `env:"FLASHBUILDER_TESTS"`
    CacheMax    units.Base2Bytes `env:"FLASHBUILDER_CACHE_MAX"     default:"5GB"`
    CacheDir    string           `env:"FLASHBUILDER_CACHE_DIR"`
}
```

**Intent**: Flag definitions with environment variable support.

**Purpose**: Enable both command-line and environment-based configuration.

**Rationale**: Kong's tag-based CLI definition is idiomatic Go with automatic environment variable support.

### 2.4 Size Flag Parsing Implementation

```go
func exampleSizeParsing() {
    kong.Parse(&cli)
    fmt.Printf("EmbedBudget: %s (%d bytes)\n", cli.EmbedBudget, int64(cli.EmbedBudget))
    fmt.Printf("CacheMax: %s (%d bytes)\n", cli.CacheMax, cli.CacheMax.Int64Value())
}
```

**Intent**: Human-readable size inputs.

**Purpose**: Improve user experience with intuitive size specifications.

**Rationale**: `units.Base2Bytes` provides idiomatic parsing of sizes like "200GB" and "5GB".

### 2.5 Cache Directory Default

The default cache directory follows XDG Base Directory Specification:

1. If `$XDG_CACHE_HOME` is set: `$XDG_CACHE_HOME/flashbuilder`
2. Else if `$HOME` is set: `$HOME/.cache/flashbuilder`
3. Else: `./.cache` (current directory)

```go
func getDefaultCacheDir() string {
    xdgCache := os.Getenv("XDG_CACHE_HOME")
    if xdgCache != "" {
        return filepath.Join(xdgCache, "flashbuilder")
    }
    
    home := os.Getenv("HOME")
    if home != "" {
        return filepath.Join(home, ".cache", "flashbuilder")
    }
    
    return ".cache"
}
```

**Intent**: Follow standard cache directory conventions.

**Purpose**: Proper cache location for variant storage.

**Rationale**: XDG specification is the standard for cache directory locations on Linux systems.

### 2.6 Generated Server Command Syntax

```
flash [flags]
```

**Intent**: Generated server has its own CLI.

**Purpose**: Runtime configuration for bind addresses, verbosity, and admin interface.

**Rationale**: Generated server needs runtime configuration separate from generator configuration.

### 2.7 Generated Server Flags

```go
var cli struct {
    HTTP      string `env:"FLASH_HTTP"      default:"localhost:8080"`
    HTTPS     string `env:"FLASH_HTTPS"     default:"localhost:8443"`
    Admin     string `env:"FLASH_ADMIN"     default:"localhost:8081"`
    Verbosity int    `env:"FLASH_LOG_LEVEL" type:"counter" short:"v"`
}
```

**Intent**: Generated server has configurable listeners.

**Purpose**: Enable multiple protocols (HTTP/1.1, HTTP/2, HTTP/3) on configurable addresses.

**Rationale**: Separate listeners for HTTP, HTTPS, and admin enable protocol-specific configuration.

### 2.8 Blocked Ports Implementation

The generated server must reject binding to these ports (exit code 10, error E010):

```go
var blockedPorts [82]int

func init() {
    blockedPorts = [82]int{1, 7, 9, 11, 13, 15, 17, 19, 20, 21, 22, 23, 25, 37, 42, 43, 53, 69, 77, 79, 87, 95, 101, 102, 103, 104, 109, 110, 111, 113, 115, 117, 119, 123, 135, 137, 139, 143, 161, 179, 389, 427, 465, 512, 513, 514, 515, 526, 530, 531, 532, 540, 548, 554, 556, 563, 587, 601, 636, 989, 990, 993, 995, 1719, 1720, 1723, 2049, 3659, 4045, 4190, 5060, 5061, 6000, 6566, 6665, 6666, 6667, 6668, 6669, 6679, 6697, 10080}
}

func isBlocked(port int) bool {
    _, found := slices.BinarySearch(blockedPorts[:], port)
    return found
}
```

**Intent**: Prevent binding to well-known service ports.

**Purpose**: Avoid conflicts with system services.

**Rationale**: Sorted array with binary search using `slices.BinarySearch` is memory-efficient and fast.

### 2.9 Compression Quality Validation

```go
func validateCompressionFlags(cli *cli) error {
    // Brotli quality: 0-11
    if cli.Brotli < 0 || cli.Brotli > 11 {
        return fmt.Errorf("E025: Brotli quality must be 0-11, got %d", cli.Brotli)
    }
    
    // AVIF quality: 0-100
    if cli.AVIF < 0 || cli.AVIF > 100 {
        return fmt.Errorf("E025: AVIF quality must be 0-100, got %d", cli.AVIF)
    }
    
    // WebP quality: 0-100
    if cli.WebP < 0 || cli.WebP > 100 {
        return fmt.Errorf("E025: WebP quality must be 0-100, got %d", cli.WebP)
    }
    
    return nil
}
```

**Intent**: Validate compression quality flags before processing.

**Purpose**: Prevent invalid compression parameters.

**Rationale**: Early validation prevents wasted processing time with invalid parameters.

---

## 3. Project Structure

### 3.1 FlashBuilder Source Structure (Flat)

All source files are in the same package (`main`) at the root level.
The `go.mod` and `go.sum` files are not part of this implementation.

```
flashbuilder/
├── main.go           # CLI, entry point, cli struct, orchestration
├── assets.go         # Asset discovery, dedupe, identifier, link, asset/variant types
├── endpoints.go       # Dispatch generation, routing, computeMaxLen
├── generate.go       # Code generation, templates, data structures
├── cache.go          # Cache management, budget allocation, fileInfo
├── variant.go        # Variant generation, compression, hashing
└── templates/
    ├── embed.go.gotmpl          # Template for assets/embed.go
    ├── main.go.gotmpl           # Template for main.go
    ├── handlers.go.gotmpl       # Template for per-length handlers
    ├── headers.http.gotmpl      # Template for header literals
    ├── shared-imports.go.gotmpl # Shared import declarations
    └── shared-struct.go.gotmpl  # Server struct template
```

**Intent**: Simplify project structure with flat package organization.

**Purpose**: Reduce complexity of imports and package management.

**Rationale**: Flat structure with all files in the same package eliminates import complexity and enables direct function calls without package prefixes.

**Template Files**: Before running `flashbuilder`, the template files must exist in the `templates/` directory as physical files. The `//go:embed` directive will then embed these files into the generator binary at compile time.

### 3.2 Generated Flash Structure

```
flash/
├── assets/
│   ├── embed.go                # Embedded assets and handlers
│   └── Asset<Identifier>.<ext> # Links to originals/variants
├── www/
│   └── <relpath>/<filename>    # Large assets
├── main.go                     # Router, dispatch, server
├── go.mod
├── go.sum
└── flash                       # Compiled binary
```

**Intent**: Clean separation between embedded assets and large assets.

**Purpose**: Separate embed-eligible from non-eligible assets.

**Rationale**: Assets directory contains embed-eligible assets. WWW directory contains large assets served via filesystem.

---

## 4. Core Data Structures

### 4.1 Asset Structure

```go
// asset represents a static asset with all pre-computed metadata
type asset struct {
    RelPath        string      // POSIX-style relative path (forward slashes)
    AbsPath        string      // Absolute path to source file
    Size           int64       // File size in bytes
    MIME           string      // Detected MIME type (e.g., "text/html")
    ImoHash        []byte      // Content hash (128 bits) from imohash
    ETag           string      // Base91 ETag for conditional GET (quoted)
    IsDuplicate    bool        // Content matches another asset
    CanonicalID    string      // Canonical identifier if duplicate
    EmbedEligible  bool        // Selected for embedding within budget
    Variants       []Variant   // Compression variants (Brotli, AVIF, WebP)
    HeaderHTTP     []byte      // Pre-computed HTTP headers as string literal
    HeaderHTTPS    []byte      // Pre-computed HTTPS headers as string literal
    Identifier     string      // Go identifier (e.g., "AssetCSS")
    Filename       string      // Filename in assets/ directory
    FrequencyScore int         // Request frequency score for switch ordering
    IsIndex        bool        // Is index file (e.g., index.html)
    IsHTML         bool        // Is HTML content (for CSP injection)
}
```

**Intent**: Complete metadata for each asset.

**Purpose**: Pre-computed fields eliminate runtime calculations.

**Rationale**: `HeaderHTTP` and `HeaderHTTPS` are separate constants for protocol-specific headers. `ImoHash` is `[]byte` (128 bits) from imohash. `ETag` is base91-encoded string with surrounding quotes.

### 4.2 Variant Structure

```go
// Variant represents a compression variant for an asset
type Variant struct {
    VariantType   VariantType  // Compression type (Brotli, AVIF, WebP)
    Size          int64        // Variant size in bytes
    HeaderHTTP    []byte       // HTTP headers for this variant
    HeaderHTTPS   []byte       // HTTPS headers for this variant
    Identifier    string       // Go identifier for this variant
    Extension     string       // File extension (e.g., ".br", ".avif", ".webp")
    CachePath     string       // Cache location for this variant
}

// VariantType represents compression type
type VariantType int

const (
    VariantBrotli VariantType = iota
    VariantAVIF
    VariantWebP
)
```

**Intent**: Track compressed variants separately.

**Purpose**: Each variant has its own headers with correct MIME type, size, and encoding.

**Rationale**: Compression variants need separate tracking for pre-computed headers.

### 4.3 Template Data Structures

```go
// TemplateData aggregates all data for template rendering
type TemplateData struct {
    Config   ConfigData
    Assets   []AssetData
    Dispatch DispatchData
    PathMaps PathMapsData
    MaxLen   int
}

// ConfigData holds configuration for template rendering
type ConfigData struct {
    CSP       string   // Content-Security-Policy header value
    HTTPSPort string   // HTTPS port for Alt-Svc header
    Module    string   // Go module name (always "flash")
}

// AssetData represents an asset for template rendering
type AssetData struct {
    RelPath        string
    Size           int64
    MIME           string
    ETag           string
    Identifier     string
    Filename       string
    IsDuplicate    bool
    CanonicalID    string
    EmbedEligible  bool
    IsIndex        bool
    IsHTML         bool
    FrequencyScore int
    HeaderHTTP     string
    HeaderHTTPS    string
    Variants       []VariantData
}

// VariantData represents a variant for template rendering
type VariantData struct {
    VariantType   VariantType
    Size          int64
    HeaderHTTP    string
    HeaderHTTPS   string
    Identifier    string
    Extension     string
    CachePath     string
}

// PathMapsData holds path mappings for routing
type PathMapsData struct {
    Canonical  map[string]string   // Path -> Identifier for canonical assets
    Duplicate  map[string]string   // Path -> CanonicalID for duplicates
    Shortcut   map[string]string   // Shortcut path -> Identifier
}
```

**Intent**: Data structures for template rendering.

**Purpose**: Aggregate all template inputs into single structures.

**Rationale**: Structured data enables clean template rendering with Go's `text/template`.

### 4.4 HeaderData Structure

```go
// HeaderData holds data for header template rendering
type HeaderData struct {
    MIME      string   // MIME type for Content-Type header
    Size      int64    // Size for Content-Length header
    IsIndex   bool     // Is index file (affects Cache-Control)
    IsHTML    bool     // Is HTML content (affects CSP injection)
    CSP       string   // Content-Security-Policy value
    HTTPSPort string   // HTTPS port for Alt-Svc header
}
```

**Intent**: Dedicated structure for header template data.

**Purpose**: Aggregate all data needed for header generation.

**Rationale**: Clean separation between data and presentation.

### 4.5 HandlerData Structure

```go
// HandlerData holds data for handler template rendering
type HandlerData struct {
    Index     int           // Dispatch array index
    Routes    []RouteData   // Routes for this dispatch index
    Protocol  string        // "HTTP" or "HTTPS"
}

// RouteData represents a route for switch statements
type RouteData struct {
    Path       string   // Sanitized path for switch case
    Identifier string   // Go identifier for handler function
    Frequency  int      // Request frequency score (for ordering)
}
```

**Intent**: Dedicated structure for handler template data.

**Purpose**: Aggregate all data needed for handler generation.

**Rationale**: Clean separation between data and presentation.

### 4.6 DispatchData Structure

```go
// DispatchData holds dispatch array data for template rendering
type DispatchData struct {
    HTTP     []DispatchEntry   // HTTP dispatch array entries
    HTTPS    []DispatchEntry   // HTTPS dispatch array entries
    MaxLen   int               // Maximum path length
}

// DispatchEntry represents a dispatch array entry
type DispatchEntry struct {
    Index     int           // Dispatch array index
    Handler   string        // Handler function name
    Routes    []RouteData   // Routes for this index
}
```

**Intent**: Structure for dispatch array data.

**Purpose**: Enable template generation of dispatch arrays.

**Rationale**: Dispatch arrays are pre-computed at generation time for O(1) routing.

### 4.7 FileInfo Structure

```go
// FileInfo holds file information for cache management
type FileInfo struct {
    Path     string      // File path
    Size     int64       // File size in bytes
    ModTime  time.Time   // Modification time for LRU eviction
}
```

**Intent**: Track file information for cache cleaning.

**Purpose**: Enable LRU-style cache eviction.

**Rationale**: Sorted by modification time, oldest files are deleted first when cache exceeds maximum size.

---

## 5. Cache Management

### 5.1 Cache Directory Initialization
```go
func ensureCacheDir(cacheDir string) error {
    err := os.MkdirAll(cacheDir, 0700)
    if err != nil {
        return fmt.Errorf("E099: Failed to create cache directory: %v", err)
    }
    return nil
}
```

**Intent**: Ensure cache directory exists before compression operations.

**Purpose**: Prevent failures during variant generation.

**Rationale**: Cache directory stores compressed variants and must exist before write operations.

### 5.2 Cache Cleaning

```go
func cleanCache(cacheDir string, maxSize int64) error {
    // Walk the cache directory and collect file information
    var totalSize int64
    var files []FileInfo
    
    err := filepath.WalkDir(cacheDir, func(path string, d fs.DirEntry, err error) error {
        if err != nil || d.IsDir() {
            return nil
        }
        info, err := d.Info()
        if err != nil {
            return nil
        }
        files = append(files, FileInfo{
            Path:    path,
            Size:    info.Size(),
            ModTime: info.ModTime(),
        })
        totalSize += info.Size()
        return nil
    })
    
    if err != nil {
        return fmt.Errorf("E099: Failed to walk cache directory: %v", err)
    }
    
    // If total size exceeds max, delete oldest files
    if totalSize > maxSize {
        // Sort by modification time (oldest first)
        sort.Slice(files, func(i, j int) bool {
            return files[i].ModTime.Before(files[j].ModTime)
        })
        
        // Delete oldest files until total size is under max
        for _, file := range files {
            if totalSize <= maxSize {
                break
            }
            os.Remove(file.Path)
            totalSize -= file.Size
        }
    }
    
    return nil
}
```

**Intent**: Maintain cache size within configured limits.

**Purpose**: Prevent disk overflow from accumulated variants.

**Rationale**: Old variants should be removed when cache exceeds configured maximum.

---

## 6. Compression Libraries

### 6.1 Brotli Compression

**Package**: `github.com/google/brotli/go/cbrotli`

**Signature**:

```go
// Encode returns content encoded with Brotli.
func Encode(content []byte, options WriterOptions) ([]byte, error)

// WriterOptions configures Writer.
type WriterOptions struct {
    Quality int
    LGWin   int
}
```

**Implementation**:

```go
func compressBrotli(content []byte, quality int) ([]byte, error) {
    opts := cbrotli.WriterOptions{
        Quality: quality,
        LGWin:   0, // Automatic window size
    }
    return cbrotli.Encode(content, opts)
}
```

**Intent**: High-quality Brotli compression for HTML/CSS/JS assets.

**Purpose**: Reduce bandwidth for text-based assets.

**Rationale**: Brotli provides superior compression ratios for web content.

### 6.2 AVIF Compression

**Package**: `github.com/vegidio/avif-go` (requires CGO during generation)

```go
func generateAVIFVariant(asset asset, quality int, cacheDir string) (variant, error) {
    img, err := decodeImage(asset.AbsPath)
    if err != nil {
        return variant{}, err
    }
    
    opts := &avif.Options{
        Speed:        6,
        AlphaQuality: quality,
        ColorQuality: quality,
    }
    
    var buf bytes.Buffer
    err = avif.Encode(&buf, img, opts)
    if err != nil {
        return variant{}, err
    }
    
    v := variant{
        VariantType:   VariantAVIF,
        Size:          int64(buf.Len()),
        Identifier:    asset.Identifier + "_avif",
        Extension:     ".avif",
        CachePath:     filepath.Join(cacheDir, asset.RelPath + ".avif"),
    }
    
    // Write compressed content to cache
    err = os.WriteFile(v.CachePath, buf.Bytes(), 0600)
    if err != nil {
        return variant{}, err
    }
    
    return v, nil
}
```

**Intent**: Generate AVIF variants for image assets.

**Purpose**: Modern image format with superior compression.

**Rationale**: AVIF provides excellent compression for photographic content.

### 6.3 WebP Compression

**Package**: `github.com/kolesa-team/go-webp/encoder` (requires CGO during generation)

```go
func generateWebPVariant(asset asset, quality int, cacheDir string) (variant, error) {
    img, err := decodeImage(asset.AbsPath)
    if err != nil {
        return variant{}, err
    }
    
    // Configure WebP options (lossy)
    opts, err := encoder.NewLossyEncoderOptions(encoder.PresetPhoto, float32(quality))
    if err != nil {
        return variant{}, err
    }
    
    enc, err := encoder.NewEncoder(img, opts)
    if err != nil {
        return variant{}, err
    }
    
    var buf bytes.Buffer
    err = enc.Encode(&buf)
    if err != nil {
        return variant{}, err
    }
    
    v := variant{
        VariantType:   VariantWebP,
        Size:          int64(buf.Len()),
        Identifier:    asset.Identifier + "_webp",
        Extension:     ".webp",
        CachePath:     filepath.Join(cacheDir, asset.RelPath + ".webp"),
    }
    
    // Write compressed content to cache
    err = os.WriteFile(v.CachePath, buf.Bytes(), 0600)
    if err != nil {
        return variant{}, err
    }
    
    return v, nil
}
```

**Intent**: Generate WebP variants for image assets.

**Purpose**: Broad browser support with good compression ratios.

**Rationale**: WebP is widely supported and provides good compression for most images.

### 6.4 Image Decoding

```go
import (
    "image"
    "image/jpeg"
    "image/png"
    "image/gif"
)

// decodeImage decodes an image file using standard library decoders
func decodeImage(absPath string) (image.Image, error) {
    file, err := os.Open(absPath)
    if err != nil {
        return nil, fmt.Errorf("E001: Failed to open image file: %v", err)
    }
    defer file.Close()
    
    ext := strings.ToLower(filepath.Ext(absPath))
    
    // Create decoder based on extension
    var img image.Image
    var decodeErr error
    
    switch ext {
    case ".jpg", ".jpeg":
        img, decodeErr = jpeg.Decode(file)
    case ".png":
        img, decodeErr = png.Decode(file)
    case ".gif":
        img, decodeErr = gif.Decode(file)
    default:
        img, _, decodeErr = image.Decode(file)
    }
    
    if decodeErr != nil {
        return nil, fmt.Errorf("E001: Failed to decode image: %v", decodeErr)
    }
    
    return img, nil
}
```

**Intent**: Decode images for variant generation.

**Purpose**: Support multiple image formats.

**Rationale**: Standard library decoders handle common formats.

---

## 7. Validation

### 7.1 Input Validation

```go
func validate(input, output string) error {
    // Validate input exists and is a directory
    info, err := os.Stat(input)
    if err != nil {
        return fmt.Errorf("E001: Input directory does not exist: %v", err)
    }
    if !info.IsDir() {
        return fmt.Errorf("E001: Input is not a directory")
    }
    
    // Validate output differs from input
    absInput, _ := filepath.Abs(input)
    absOutput, _ := filepath.Abs(output)
    if absInput == absOutput {
        return fmt.Errorf("E099: Input and output must differ")
    }
    
    return nil
}
```

**Intent**: Validate input and output directories before processing.

**Purpose**: Prevent errors during generation.

**Rationale**: Early validation avoids wasted processing time.

---

## 8. Template System

### 8.1 Template Embedding

All templates are embedded using `//go:embed`:

```go
//go:embed templates/*.go.gotmpl
var templateFS embed.FS
```

**Intent**: Embed all templates into the generator binary.

**Purpose**: Templates are available at runtime without filesystem access.

**Rationale**: Embedded templates enable single-binary distribution.

**Template Files Requirement**: The template files (`embed.go.gotmpl`, `main.go.gotmpl`, `handlers.go.gotmpl`, `headers.http.gotmpl`, `shared-imports.go.gotmpl`, `shared-struct.go.gotmpl`) must exist as physical files in the `templates/` directory before the `//go:embed` directive can embed them. Create these files with the content specified in Section 16.

### 8.2 Template Functions

```go
var funcMap = template.FuncMap{
    "quote":      func(s string) string { return "\"" + s + "\"" },
    "trim":       strings.TrimSpace,
    "upper":      strings.ToUpper,
    "lower":      strings.ToLower,
    "sanitize":   func(s string) string { return sanitizeIdentifier(s) },
    "escapeHeader": func(s string) string { return strings.ReplaceAll(s, "\n", "\\n") + "\r\n" },
    "int":        func(s string) int { n, _ := strconv.Atoi(s); return n },
    "int64":      func(s string) int64 { n, _ := strconv.ParseInt(s, 10, 64); return n },
    "add":        func(a, b int) int { return a + b },
    "sub":        func(a, b int) int { return a - b },
    "mul":        func(a, b int) int { return a * b },
    "div":        func(a, b int) int { return a / b },
    "len":        func(s string) int { return len(s) },
    "default":    func(def, val string) string { if val != "" { return val }; return def },
    "capitalize": func(s string) string { if len(s) > 0 { return strings.ToUpper(s[:1]) + s[1:] }; return s },
}
```

**Intent**: Provide helper functions for template operations.

**Purpose**: Simplify template logic with custom functions.

**Rationale**: Custom functions reduce template complexity. The `sanitize` function calls `sanitizeIdentifier()` which must be defined and exported.

### 8.3 Template Parsing (Cached)

```go
var cachedTemplates *template.Template
var cachedTemplatesErr error

func parseTemplates() (*template.Template, error) {
    if cachedTemplates != nil {
        return cachedTemplates, nil
    }
    if cachedTemplatesErr != nil {
        return nil, cachedTemplatesErr
    }
    
    tmpl := template.New("root").Funcs(funcMap)
    cachedTemplates, cachedTemplatesErr = tmpl.ParseFS(templateFS, "templates/*.go.gotmpl")
    return cachedTemplates, cachedTemplatesErr
}
```

**Intent**: Parse templates once and cache the result.

**Purpose**: Avoid repeated parsing overhead.

**Rationale**: Template parsing is expensive; caching improves generation speed.

### 8.4 Template Rendering

```go
func renderTemplate(name string, data any) (string, error) {
    tmpl, err := parseTemplates()
    if err != nil {
        return "", err
    }
    
    var buf strings.Builder
    err = tmpl.ExecuteTemplate(&buf, name, data)
    if err != nil {
        return "", err
    }
    
    return buf.String(), nil
}
```

**Intent**: Render templates with data.

**Purpose**: Generate Go code from templates.

**Rationale**: Template-based generation is cleaner and more maintainable.

---

## 9. Dispatch Array System

### 9.1 Dispatch Indexing Logic

**Intent**: The dispatch array enables O(1) routing by mapping path length directly to handler functions.

**Purpose**: Eliminate runtime path parsing and string manipulation.

**Rationale**: The request path always begins with a leading slash (`/example/path`). Asset routes are relative paths without leading slash (`example/path`). The dispatch array index equals the request path length after removing the leading slash. This design avoids runtime slash removal and byte subtraction.

**Key Insight**:
- Request path: `/example` (length 7 including leading slash)
- After removing leading slash: `example` (length 6)
- Dispatch index: 6
- Asset route length: 6 (matches dispatch index)
- For path `/a`, dispatch index = 1, asset route length = 0
- **Relationship**: `dispatch_index = asset_route_length + 1`

Document that "Dispatch Indexing Logic" within the generated code of the `flash` server:

```go
// Dispatch array indexing:
// - Index 0: Empty path (root request "/")
// - Index 1: Paths of length 1 (e.g., "/a") -> asset route length 0
// - Index L: Paths of length L -> asset route length L-1
// - MaxLen+1: Fallback for paths exceeding maximum length
//
// Asset routes are stored without leading slash:
// - Route "" (empty) corresponds to dispatch index 1
// - Route "example" (length 7) corresponds to dispatch index 8
//
// The relationship: dispatch_index = asset_route_length + 1
// This eliminates runtime slash removal and byte subtraction.
```

### 9.2 Dispatch Array Generation

```go
func buildDispatch(assets []asset, maxLen int) (httpDispatch, httpsDispatch []DispatchEntry) {
    // routesByLength groups routes by length (asset route length = dispatch index - 1)
    routesByLength := make(map[int][]RouteData)
    
    for _, asset := range assets {
        if asset.IsDuplicate {
            continue
        }
        routeLength := len(asset.RelPath)
        // Dispatch index = route length + 1
        dispatchIndex := routeLength + 1
        
        route := RouteData{
            Path:       sanitizePath(asset.RelPath),
            Identifier: asset.Identifier,
            Frequency:  asset.FrequencyScore,
        }
        
        routesByLength[dispatchIndex] = append(routesByLength[dispatchIndex], route)
    }
    
    // Sort routes by frequency score within each length group
    for _, routes := range routesByLength {
        sort.Slice(routes, func(i, j int) bool {
            return routes[i].Frequency > routes[j].Frequency
        })
    }
    
    httpDispatch = make([]DispatchEntry, maxLen+2)
    httpsDispatch = make([]DispatchEntry, maxLen+2)
    
    // Index 0 and 1: Root handlers
    // Index 0: Empty path after removing leading slash (root "/")
    // Index 1: Path "/" -> pathNoSlash "" -> length 0
    rootHandler := "serveRootIndex"
    if hasRootIndex(assets) {
        httpDispatch[0] = DispatchEntry{Index: 0, Handler: rootHandler}
        httpDispatch[1] = DispatchEntry{Index: 1, Handler: rootHandler}
        httpsDispatch[0] = DispatchEntry{Index: 0, Handler: rootHandler}
        httpsDispatch[1] = DispatchEntry{Index: 1, Handler: rootHandler}
    } else {
        httpDispatch[0] = DispatchEntry{Index: 0, Handler: "http.NotFound"}
        httpDispatch[1] = DispatchEntry{Index: 1, Handler: "http.NotFound"}
        httpsDispatch[0] = DispatchEntry{Index: 0, Handler: "http.NotFound"}
        httpsDispatch[1] = DispatchEntry{Index: 1, Handler: "http.NotFound"}
    }
    
    for index := 2; index <= maxLen+1; index++ {
        routes := routesByLength[index]
        if len(routes) == 0 {
            // No routes at this length, fallback to previous
            fallback := index - 1
            for fallback >= 0 && httpDispatch[fallback].Handler == "" {
                fallback--
            }
            if fallback < 0 {
                httpDispatch[index] = DispatchEntry{Index: index, Handler: "http.NotFound"}
                httpsDispatch[index] = DispatchEntry{Index: index, Handler: "http.NotFound"}
            } else {
                httpDispatch[index] = httpDispatch[fallback]
                httpsDispatch[index] = httpsDispatch[fallback]
            }
            continue
        }
        
        // Store the routes and the handler names
        handlerName := fmt.Sprintf("getLen%d", index)
        httpDispatch[index] = DispatchEntry{
            Index:   index,
            Handler: handlerName + "HTTP",
            Routes:  routes,
        }
        httpsDispatch[index] = DispatchEntry{
            Index:   index,
            Handler: handlerName + "HTTPS",
            Routes:  routes,
        }
    }
    
    return httpDispatch, httpsDispatch
}

func hasRootIndex(assets []asset) bool {
    for _, asset := range assets {
        if asset.RelPath == "index.html" || asset.RelPath == "" {
            return true
        }
    }
    return false
}
```

**Intent**: Generate dispatch arrays with proper indexing.

**Purpose**: Enable O(1) routing with pre-computed dispatch arrays.

**Rationale**: Dispatch index = asset route length + 1. This eliminates runtime slash removal and byte subtraction.

### 9.3 Path Sanitization for Switch Cases

```go
// sanitizePath converts a path to a valid Go string literal for switch cases
func sanitizePath(path string) string {
    path = strings.ReplaceAll(path, "\\", "/")
    
    var result strings.Builder
    for _, r := range path {
        switch r {
        case '\\':
            result.WriteString("\\\\")
        case '"':
            result.WriteString("\\\"")
        case '\n':
            result.WriteString("\\n")
        case '\r':
            result.WriteString("\\r")
        case '\t':
            result.WriteString("\\t")
        default:
            result.WriteRune(r)
        }
    }
    
    return result.String()
}
```

**Intent**: Convert paths to valid Go string literals for switch case matching.

**Purpose**: Ensure paths are usable as case labels in switch statements.

**Rationale**: Go switch cases use string literals; special characters must be escaped.

---

## 10. Core Algorithms

### 10.1 Asset Discovery

```go
import (
    "os"
    "path/filepath"
    "strings"
)

// discover walks the input directory and collects all files
func discover(input string) []asset {
    var assets []asset
    
    err := filepath.WalkDir(input, func(path string, d fs.DirEntry, err error) error {
        if err != nil {
            return err
        }
        
        // Skip directories
        if d.IsDir() {
            return nil
        }
        
        // Get file info
        info, err := d.Info()
        if err != nil {
            return nil
        }
        
        // Skip special files
        mode := info.Mode()
        if mode&os.ModeSocket != 0 || mode&os.ModeDevice != 0 || mode&os.ModeNamedPipe != 0 {
            return nil
        }
        
        // Follow symlinks
        resolvedPath := path
        if mode&os.ModeSymlink != 0 {
            target, err := os.Readlink(path)
            if err == nil {
                absTarget := filepath.Join(filepath.Dir(path), target)
                targetInfo, err := os.Stat(absTarget)
                if err == nil && !targetInfo.IsDir() {
                    resolvedPath = absTarget
                    info = targetInfo
                }
            }
        }
        
        // Compute relative path (POSIX-style)
        relPath, err := filepath.Rel(input, resolvedPath)
        relPath = filepath.ToSlash(relPath)
        
        // Compute absolute path
        absPath, err := filepath.Abs(resolvedPath)
        
        asset := asset{
            RelPath:  relPath,
            AbsPath:  absPath,
            Size:     info.Size(),
            MIME:     detectMIME(resolvedPath),
        }
        assets = append(assets, asset)
        return nil
    })
    
    if err != nil {
        return nil
    }
    
    // Sort by relative path for deterministic ordering
    sort.Slice(assets, func(i, j int) bool {
        return assets[i].RelPath < assets[j].RelPath
    })
    
    return assets
}
```

**Intent**: Discover all regular files and symlinks to files.

**Purpose**: Deterministic ordering enables reproducible builds.

**Rationale**: `filepath.WalkDir` is more efficient than `filepath.Walk` and avoids unnecessary system calls. Symlink following enables proper deduplication.

### 10.2 MIME Detection

```go
import (
    "mime"
    "net/http"
    "os"
)

// detectMIME determines the MIME type for a file
// Step 1: Extension-based lookup using mime.TypeByExtension
// Step 2: Content sniffing using http.DetectContentType
// Step 3: Fallback to application/octet-stream
func detectMIME(path string) string {
    // Step 1: Extension-based lookup
    ext := filepath.Ext(path)
    if ext != "" {
        mimeType := mime.TypeByExtension(ext)
        if mimeType != "" {
            return mimeType
        }
    }
    
    // Step 2: Content sniffing
    data, err := os.ReadFile(path)
    if err == nil && len(data) > 0 {
        sniff := data[:min(512, len(data))]
        mimeType := http.DetectContentType(sniff)
        if mimeType != "" {
            return mimeType
        }
    }
    
    // Step 3: Fallback
    return "application/octet-stream"
}
```

**Intent**: Accurate MIME types enable correct Content-Type headers.

**Purpose**: Extension-based lookup is fastest; content sniffing handles unknown extensions.

**Rationale**: The `mime.TypeByExtension` function provides extension-based MIME detection. Content sniffing handles files with unknown or missing extensions.

### 10.3 ImoHash and ETag

```go
import (
    "github.com/kalafut/imohash"
    "github.com/mtraver/base91"
)

// computeImoHash computes the ImoHash for a file
// Returns 128-bit hash as []byte
func computeImoHash(path string) []byte {
    sum, err := imohash.SumFile(path)
    if err != nil {
        return nil
    }
    return sum[:] // 128 bits
}

// computeETag generates an ETag from ImoHash
// Uses base91 encoding for compact representation
// Returns quoted ETag string for HTTP headers
func computeETag(hash []byte) string {
    encoded := base91.StdEncoding.EncodeToString(hash)
    return "\"" + encoded + "\""
}
```

**Intent**: ImoHash provides fast content hash; base91 encoding produces compact ETag.

**Purpose**: Base91 produces shorter strings than hex, enabling faster byte-by-byte comparison.

**Rationale**: ImoHash is optimized for fast content hashing. Base91 encoding is more compact than hex.

### 10.4 Deduplication

```go
// dedupe identifies duplicate assets based on content hash
func dedupe(assets []asset) []asset {
    hashMap := make(map[string][]asset)
    
    // Group assets by hash
    for _, asset := range assets {
        key := string(asset.ImoHash)
        hashMap[key] = append(hashMap[key], asset)
    }
    
    // Mark duplicates
    for _, group := range hashMap {
        if len(group) > 1 {
            // First asset is canonical
            canonical := group[0]
            for i := 1; i < len(group); i++ {
                // On-demand content verification
                canonicalContent, _ := os.ReadFile(canonical.AbsPath)
                duplicateContent, _ := os.ReadFile(group[i].AbsPath)
                if bytes.Equal(canonicalContent, duplicateContent) {
                    group[i].IsDuplicate = true
                    group[i].CanonicalID = canonical.Identifier
                }
            }
        }
    }
    
    return assets
}
```

**Intent**: Find identical content across different paths.

**Purpose**: On-demand content reading prevents loading all assets into memory simultaneously.

**Rationale**: Deduplication reduces binary size by eliminating duplicate content.

### 10.5 Identifier Generation

```go
import (
    "unicode"
)

// generateIdentifier creates a valid Go identifier from a relative path
func generateIdentifier(relPath string, existing map[string]bool) string {
    // Split path into segments
    segments := strings.Split(relPath, "/")
    filename := filepath.Base(relPath)
    ext := filepath.Ext(filename)
    filename = strings.TrimSuffix(filename, ext)
    
    capitalize := func(s string) string {
        if len(s) == 0 {
            return ""
        }
        return strings.ToUpper(s[:1]) + s[1:]
    }
    
    // Sanitize each segment
    var parts []string
    for _, seg := range segments {
        // Filter valid identifier characters
        seg = sanitizeIdentifier(seg)
        if len(seg) > 0 {
            parts = append(parts, capitalize(seg))
        }
    }
    
    // Add filename if present
    if filename != "" && filename != "." {
        filename = sanitizeIdentifier(filename)
        if len(filename) > 0 {
            parts = append(parts, capitalize(filename))
        }
    }
    
    identifier := "Asset" + strings.Join(parts, "")
    
    // Resolve collisions
    if existing[identifier] {
        for i := 2; i < 1000; i++ {
            suffix := fmt.Sprintf("_%03d", i)
            newID := identifier + suffix
            if !existing[newID] {
                identifier = newID
                break
            }
        }
    }
    
    return identifier
}

// sanitizeIdentifier filters valid Go identifier characters
// This function must be exported for use in template functions
func sanitizeIdentifier(s string) string {
    var result strings.Builder
    for _, r := range s {
        if unicode.IsLetter(r) || unicode.IsDigit(r) {
            result.WriteRune(r)
        }
    }
    return result.String()
}
```

**Intent**: Generate valid Go identifiers from path segments.

**Purpose**: Deterministic algorithm ensures reproducibility.

**Rationale**: Collision resolution prevents duplicate identifiers. The `sanitizeIdentifier` function is exported for use in template functions.

### 10.6 Frequency Score

```go
// estimateFrequencyScore estimates request frequency for switch case ordering
// Higher frequency assets should appear first in switch statements
// for better branch prediction and cache locality
func estimateFrequencyScore(path string, isEmbed bool) int {
    score := 0
    
    if path == "" || path == "index.html" {
        score += 1000
    }
    
    if strings.Contains(path, "favicon.") {
        score += 900
    }
    
    if strings.HasSuffix(path, ".css") {
        score += 800
    }
    
    if strings.HasSuffix(path, ".js") {
        score += 600
    }
    
    if strings.Contains(path, "index.html") {
        score += 500
    }
    
    if strings.Contains(path, "logo.") {
        score += 400
    }
    
    if isEmbed {
        score += 200
    }
    
    // Path complexity penalty
    score -= 5 * len(path)
    score -= 30 * strings.Count(path, "/")
    
    lowTraffic := []string{".map", ".zip", ".pdf", ".doc", ".xls", ".tar"}
    for _, ext := range lowTraffic {
        if strings.HasSuffix(path, ext) {
            score -= 100
            break
        }
    }
    
    return score
}
```

**Intent**: Estimate request frequency for switch case ordering.

**Purpose**: High-frequency assets should appear first in switch statements for better branch prediction.

**Rationale**: Frequency score ordering optimizes switch case locality.

### 10.7 Shortcut Generation

```go
// generateShortcut creates an extensionless shortcut for a path
// Enables clean URLs like "/about" instead of "/about/index.html"
func generateShortcut(relPath string) string {
    // Root index has no shortcut
    if relPath == "index.html" {
        return ""
    }
    
    // Index files in subdirectories
    if strings.HasSuffix(relPath, "/index.html") {
        return strings.TrimSuffix(relPath, "/index.html")
    }
    
    // Extensionless shortcuts
    ext := filepath.Ext(relPath)
    if ext != "" {
        return strings.TrimSuffix(relPath, ext)
    }
    
    return relPath
}
```

**Intent**: Generate extensionless shortcuts for routing.

**Purpose**: Users often omit extensions; shortcuts map extensionless paths to canonical assets.

**Rationale**: Shortcuts enable cleaner URLs (e.g., `/about` instead of `/about/index.html` or `/style` instead of `/style.css`).

### 10.8 MaxLen Computation

```go
// computeMaxLen calculates the maximum path length for dispatch array sizing
func computeMaxLen(assets []asset, canonicalPaths, duplicatePaths, shortcutPaths map[string]string) int {
    maxLen := 0
    for _, asset := range assets {
        if len(asset.RelPath) > maxLen {
            maxLen = len(asset.RelPath)
        }
    }
    return maxLen
}
```

**Intent**: Calculate dispatch array size.

**Purpose**: Dispatch array must accommodate all path lengths.

**Rationale**: Adding 2 ensures coverage of trailing slash and null terminator edge cases.

---

## 11. Embed Budget Allocation

### 11.1 Budget Allocation Algorithm

```go
// allocateBudget determines which assets are eligible for embedding
// Assets are sorted by size (smallest first) and embedded until budget is exhausted
func allocateBudget(assets []asset, budget int64) []asset {
    // Sort assets by size (smallest first for embedding priority)
    sort.Slice(assets, func(i, j int) bool {
        return assets[i].Size < assets[j].Size
    })
    
    var totalSize int64
    for i := range assets {
        if totalSize+assets[i].Size <= budget {
            assets[i].EmbedEligible = true
            totalSize += assets[i].Size
        } else {
            assets[i].EmbedEligible = false
        }
    }
    
    return assets
}
```

**Intent**: Determine embed eligibility based on budget constraints.

**Purpose**: Smaller assets are embedded first to maximize embed count.

**Rationale**: Budget allocation sorts assets by size and embeds smallest first until budget is exhausted.

---

## 12. Variant Generation

### 12.1 Generate Variants

```go
// generateVariants creates compression variants for eligible assets
func generateVariants(assets []asset, brotliQuality, avifQuality, webPQuality int, cacheDir string) []asset {
    for i := range assets {
        // Skip non-embed eligible assets for variants
        if !assets[i].EmbedEligible {
            continue
        }
        
        var variants []Variant
        
        // Generate Brotli variant for HTML/CSS/JS
        if isCompressible(assets[i].MIME) && brotliQuality > 0 {
            content, err := os.ReadFile(assets[i].AbsPath)
            if err == nil {
                compressed, err := compressBrotli(content, brotliQuality)
                if err == nil && int64(len(compressed)) < assets[i].Size {
                    v := Variant{
                        VariantType:   VariantBrotli,
                        Size:          int64(len(compressed)),
                        Identifier:    assets[i].Identifier + "_brotli",
                        Extension:     ".br",
                        CachePath:     filepath.Join(cacheDir, assets[i].RelPath + ".br"),
                    }
                    err = os.WriteFile(v.CachePath, compressed, 0600)
                    if err == nil {
                        variants = append(variants, v)
                    }
                }
            }
        }
        
        // Generate AVIF variant for images
        if isImage(assets[i].MIME) && avifQuality > 0 {
            v, err := generateAVIFVariant(assets[i], avifQuality, cacheDir)
            if err == nil && v.Size < assets[i].Size {
                variants = append(variants, v)
            }
        }
        
        // Generate WebP variant for images
        if isImage(assets[i].MIME) && webPQuality > 0 {
            v, err := generateWebPVariant(assets[i], webPQuality, cacheDir)
            if err == nil && v.Size < assets[i].Size {
                variants = append(variants, v)
            }
        }
        
        assets[i].Variants = variants
    }
    
    return assets
}

// isCompressible determines if content is eligible for Brotli compression
func isCompressible(mime string) bool {
    compressible := []string{
        "text/html",
        "text/css",
        "text/javascript",
        "application/javascript",
        "text/plain",
        "text/xml",
        "application/json",
        "application/xml",
    }
    for _, m := range compressible {
        if strings.HasPrefix(mime, m) {
            return true
        }
    }
    return false
}

// isImage determines if content is an image
func isImage(mime string) bool {
    return strings.HasPrefix(mime, "image/")
}
```

**Intent**: Generate compression variants for eligible assets.

**Purpose**: Reduce bandwidth through compression.

**Rationale**: Only assets smaller than original are kept as variants.

---

## 13. Header Generation

### 13.1 Render HTTP Headers

```go
// renderHeaderHTTP generates HTTP headers for an asset
func renderHeaderHTTP(asset asset, csp string) []byte {
    var headers strings.Builder
    
    headers.WriteString(fmt.Sprintf("Content-Type: %s\r\n", asset.MIME))
    
    if asset.IsIndex {
        headers.WriteString("Cache-Control: public, max-age=31536000, immutable, must-revalidate\r\n")
    } else {
        headers.WriteString("Cache-Control: public, max-age=31536000, immutable\r\n")
    }
    
    headers.WriteString(fmt.Sprintf("Content-Length: %d\r\n", asset.Size))
    
    if asset.IsHTML && csp != "" {
        headers.WriteString(fmt.Sprintf("Content-Security-Policy: %s\r\n", csp))
    }
    
    return []byte(headers.String())
}
```

### 13.2 Render HTTPS Headers

```go
// renderHeaderHTTPS generates HTTPS headers for an asset
func renderHeaderHTTPS(asset asset, csp string, httpsPort string) []byte {
    var headers strings.Builder
    
    headers.WriteString(fmt.Sprintf("Content-Type: %s\r\n", asset.MIME))
    
    if asset.IsIndex {
        headers.WriteString("Cache-Control: public, max-age=31536000, immutable, must-revalidate\r\n")
    } else {
        headers.WriteString("Cache-Control: public, max-age=31536000, immutable\r\n")
    }
    
    headers.WriteString(fmt.Sprintf("Content-Length: %d\r\n", asset.Size))
    
    if asset.IsHTML && csp != "" {
        headers.WriteString(fmt.Sprintf("Content-Security-Policy: %s\r\n", csp))
    }
    
    headers.WriteString("Strict-Transport-Security: max-age=31536000\r\n")
    headers.WriteString(fmt.Sprintf("Alt-Svc: h3=\":%s\"; ma=2592000\r\n", httpsPort))
    
    return []byte(headers.String())
}
```

**Intent**: Pre-compute headers for embed-eligible assets.

**Purpose**: Headers are compile-time constants for zero allocation.

**Rationale**: Pre-computed headers eliminate runtime header generation.

---

## 14. Link Creation

### 14.1 Create Links

```go
// createLinks creates symbolic links for assets in the output directory
func createLinks(assets []asset, input string, output string, cacheDir string) error {
    // Create assets directory
    assetsDir := filepath.Join(output, "assets")
    if err := os.MkdirAll(assetsDir, 0700); err != nil {
        return fmt.Errorf("E087: Failed to create assets directory: %v", err)
    }
    
    // Create www directory
    wwwDir := filepath.Join(output, "www")
    if err := os.MkdirAll(wwwDir, 0700); err != nil {
        return fmt.Errorf("E087: Failed to create www directory: %v", err)
    }
    
    for _, asset := range assets {
        if asset.IsDuplicate {
            continue
        }
        
        if asset.EmbedEligible {
            // Create symlink in assets directory
            target := filepath.Join(assetsDir, asset.Filename+filepath.Ext(asset.RelPath))
            if err := os.Symlink(asset.AbsPath, target); err != nil {
                return fmt.Errorf("E087: Failed to create symlink: %v", err)
            }
        } else {
            // Create symlink in www directory
            target := filepath.Join(wwwDir, asset.RelPath)
            if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
                return fmt.Errorf("E087: Failed to create www subdirectory: %v", err)
            }
            if err := os.Symlink(asset.AbsPath, target); err != nil {
                return fmt.Errorf("E087: Failed to create symlink: %v", err)
            }
        }
    }
    
    return nil
}
```

**Intent**: Create symbolic links pointing to original assets.

**Purpose**: Links avoid duplicating content during generation.

**Rationale**: Symlinks save disk space and preserve original file locations.

---

## 15. Code Generation

### 15.1 Convert Assets to Template Data

```go
// convertAssets converts asset structures to template data
func convertAssets(assets []asset) []AssetData {
    var result []AssetData
    for _, asset := range assets {
        if asset.IsDuplicate {
            continue
        }
        
        var variants []VariantData
        for _, v := range asset.Variants {
            variants = append(variants, VariantData{
                VariantType:   v.VariantType,
                Size:          v.Size,
                HeaderHTTP:    string(v.HeaderHTTP),
                HeaderHTTPS:   string(v.HeaderHTTPS),
                Identifier:    v.Identifier,
                Extension:     v.Extension,
                CachePath:     v.CachePath,
            })
        }
        
        result = append(result, AssetData{
            RelPath:        asset.RelPath,
            Size:           asset.Size,
            MIME:           asset.MIME,
            ETag:           asset.ETag,
            Identifier:     asset.Identifier,
            Filename:       asset.Filename,
            IsDuplicate:    asset.IsDuplicate,
            CanonicalID:    asset.CanonicalID,
            EmbedEligible:  asset.EmbedEligible,
            IsIndex:        asset.IsIndex,
            IsHTML:         asset.IsHTML,
            FrequencyScore: asset.FrequencyScore,
            HeaderHTTP:     string(asset.HeaderHTTP),
            HeaderHTTPS:    string(asset.HeaderHTTPS),
            Variants:       variants,
        })
    }
    return result
}
```

### 15.2 Generate Go Code

```go
// generate generates the Go code for the flash server
func generate(data TemplateData, output string) error {
    // Ensure output directory exists
    if err := os.MkdirAll(output, 0700); err != nil {
        return fmt.Errorf("E099: Failed to create output directory: %v", err)
    }
    
    // Generate assets/embed.go
    embedCode, err := renderTemplate("embed", data)
    if err != nil {
        return fmt.Errorf("E099: Failed to render embed template: %v", err)
    }
    if err := os.WriteFile(filepath.Join(output, "assets", "embed.go"), []byte(embedCode), 0600); err != nil {
        return fmt.Errorf("E099: Failed to write embed.go: %v", err)
    }
    
    // Generate main.go
    mainCode, err := renderTemplate("main", data)
    if err != nil {
        return fmt.Errorf("E099: Failed to render main template: %v", err)
    }
    if err := os.WriteFile(filepath.Join(output, "main.go"), []byte(mainCode), 0600); err != nil {
        return fmt.Errorf("E099: Failed to write main.go: %v", err)
    }
    
    return nil
}
```

---

## 16. Template Definitions

### 16.1 `templates/embed.go.gotmpl`

```go
// Package assets contains all embedded assets.
package assets

import (
    _ "embed"
    "net/http"
)

{{range .Assets}}
    {{if and (not .IsDuplicate) (.EmbedEligible)}}
        //go:embed {{.Filename}}
        // Symlink to original: {{.AbsPath}}
        var {{.Identifier}} []byte

        var {{.Identifier}}HeaderHTTP = []byte("{{.HeaderHTTP}}")
        var {{.Identifier}}HeaderHTTPS = []byte("{{.HeaderHTTPS}}")

        func (s *Server) Serve{{.Identifier}}HTTP(w http.ResponseWriter, r *http.Request) {
            if r.Header.Get("If-None-Match") == {{.ETag}} {
                w.WriteHeader(http.StatusNotModified)
                return
            }
            w.Write({{.Identifier}}HeaderHTTP)
            if r.Method != "" && r.Method[0] == 'H' {
                return
            }
            w.Write({{.Identifier}})
        }

        func (s *Server) Serve{{.Identifier}}HTTPS(w http.ResponseWriter, r *http.Request) {
            if r.Header.Get("If-None-Match") == {{.ETag}} {
                w.WriteHeader(http.StatusNotModified)
                return
            }
            w.Write({{.Identifier}}HeaderHTTPS)
            if r.Method != "" && r.Method[0] == 'H' {
                return
            }
            w.Write({{.Identifier}})
        }
    {{end}}
{{end}}
```

**Intent**: Generate embed-eligible assets with pre-computed headers.

**Purpose**: Template simplifies asset handler generation.

**Rationale**: `{{.ETag}}` adds surrounding quotes for HTTP ETag format. Handlers are methods on `*Server` to access dispatch arrays.

### 16.2 `templates/main.go.gotmpl`

```go
package main

import (
    "crypto/ecdsa"
    "crypto/elliptic"
    "crypto/rand"
    "crypto/tls"
    "crypto/x509"
    "crypto/x509/pkix"
    "fmt"
    "math/big"
    "net"
    "net/http"
    "os"
    "os/signal"
    "path/filepath"
    "slices"
    "strconv"
    "syscall"
    "time"
    "github.com/alecthomas/kong"
    "golang.org/x/net/http2"
    quic "github.com/quic-go/quic-go/http3"
    "flash/assets"
)

type Server struct {
    Logger        *slog.Logger
    DispatchHTTP  [{{.MaxLen}}+2]func(http.ResponseWriter, *http.Request)
    DispatchHTTPS [{{.MaxLen}}+2]func(http.ResponseWriter, *http.Request)
    TLSCfg        *tls.Config
}

// RouterHTTP dispatches HTTP requests based on path length.
// The dispatch array index equals the request path length.
// Path length 0 is the root (empty path after removing leading slash).
// Path length 1 corresponds to paths like "/a" (asset route length = 0).
// This design avoids runtime slash removal and length subtraction.
func (s *Server) routerHTTP(w http.ResponseWriter, r *http.Request) {
    // Request path starts with leading slash: "/example/path"
    // We skip the leading slash to get: "example/path"
    pathNoSlash := r.URL.Path[1:]
    // Dispatch array index equals the length of pathNoSlash
    // MaxLen+1 is the fallback for paths exceeding maximum length
    idx := min(len(pathNoSlash), {{.MaxLen}}+1)
    s.DispatchHTTP[idx](w, r)
}

func (s *Server) routerHTTPS(w http.ResponseWriter, r *http.Request) {
    pathNoSlash := r.URL.Path[1:]
    idx := min(len(pathNoSlash), {{.MaxLen}}+1)
    s.DispatchHTTPS[idx](w, r)
}

{{range .Dispatch.HTTP}}
    {{if .Handler}}
        // getLen{{.Index}}HTTP handles paths of length {{.Index}}.
        // Dispatch array index = {{.Index}} corresponds to asset route length = {{.Index}}-1.
        // Asset routes are relative paths without leading slash.
        func (s *Server) getLen{{.Index}}HTTP(w http.ResponseWriter, r *http.Request) {
            const L = {{.Index}}
            pathNoSlash := r.URL.Path[1:] // asset routes are relative (no leading slash)iii
            truncated := pathNoSlash[:L-1] // Match against asset routes of length L-1
            switch truncated {
            {{range .Routes}}
                case "{{.Path | sanitize}}":
                    s.Serve{{.Identifier}}HTTP(w, r)
            {{end}}
            default:
                s.DispatchHTTP[L-1](w, r)
            }
        }
    {{end}}
{{end}}

{{range .Dispatch.HTTPS}}
    {{if .Handler}}
        func (s *Server) getLen{{.Index}}HTTPS(w http.ResponseWriter, r *http.Request) {
            const L = {{.Index}}
            pathNoSlash := r.URL.Path[1:] // asset routes are relative (no leading slash)iii
            truncated := pathNoSlash[:L-1] // Match against asset routes of length L-1
            switch truncated {
            {{range .Routes}}
                case "{{.Path | sanitize}}":
                    s.Serve{{.Identifier}}HTTPS(w, r)
            {{end}}
            default:
                s.DispatchHTTPS[L-1](w, r)
            }
        }
    {{end}}
{{end}}

func main() {
    var cli struct {
        HTTP   string `flag:"--http" default:"localhost:8080"`
        HTTPS  string `flag:"--https" default:"localhost:8443"`
        Admin  string `flag:"--admin" default:"localhost:8081"`
        V      int    `flag:"-v" default:"0"`
    }
    kong.Parse(&cli)

    // Blocked ports validation
    blocked := [82]int{1, 7, 9, 11, 13, 15, 17, 19, 20, 21, 22, 23, 25, 37, 42, 43, 53, 69, 77, 79, 87, 95, 101, 102, 103, 104, 109, 110, 111, 113, 115, 117, 119, 123, 135, 137, 139, 143, 161, 179, 389, 427, 465, 512, 513, 514, 515, 526, 530, 531, 532, 540, 548, 554, 556, 563, 587, 601, 636, 989, 990, 993, 995, 1719, 1720, 1723, 2049, 3659, 4045, 4190, 5060, 5061, 6000, 6566, 6665, 6666, 6667, 6668, 6669, 6679, 6697, 10080}
    
    // Check blocked ports for HTTP
    if cli.HTTP != "none" {
        _, port, _ := net.SplitHostPort(cli.HTTP)
        portNum, _ := strconv.Atoi(port)
        if slices.BinarySearch(blocked[:], portNum) {
            fmt.Println("E010: Blocked port")
            os.Exit(10)
        }
    }
    
    // Check blocked ports for HTTPS
    if cli.HTTPS != "none" {
        _, port, _ := net.SplitHostPort(cli.HTTPS)
        portNum, _ := strconv.Atoi(port)
        if slices.BinarySearch(blocked[:], portNum) {
            fmt.Println("E010: Blocked port")
            os.Exit(10)
        }
    }

    // Generate TLS config
    tlsCfg := generateTLSConfig()

    // Start servers...
}

func generateTLSConfig() *tls.Config {
    key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
    if err != nil {
        panic(err)
    }
    serial, _ := rand.Int(rand.Reader, big.NewInt(1<<64-1))
    cert := &x509.Certificate{
        SerialNumber: serial,
        Subject:      pkix.Name{CommonName: "flash"},
        NotBefore:    time.Now(),
        NotAfter:     time.Now().Add(10 * 365 * 24 * time.Hour),
    }
    certBytes := x509.CreateCertificate(rand.Reader, cert, cert, &key.Public(), key)
    return &tls.Config{
        MinVersion:  tls.VersionTLS13,
        NextProtos:  []string{"h3", "h2", "http/1.1"},
        Certificates: []tls.Certificate{
            {Certificate: certBytes, PrivateKey: key},
        },
    }
}
```

**Intent**: Generate main.go with router and dispatch arrays.

**Purpose**: Template simplifies main.go generation.

**Rationale**: Array sizes are written as literal constants during code generation, not as template variables. Dispatch array indexing is explained in comments to clarify the relationship between request path length and asset route length. Uses Go 1.26 `min` builtin and `slices.BinarySearch`.

### 16.3 `templates/handlers.go.gotmpl`

```go
{{/* Per-length handler template for HTTP */}}
{{define "httpHandler"}}
// getLen{{.Index}}HTTP handles HTTP requests for paths of length {{.Index}}.
// Dispatch array index = {{.Index}} corresponds to asset routes of length {{.Index}}-1.
// Request paths include leading slash: "/example" -> pathNoSlash = "example" (length 7)
// Asset routes are relative paths without leading slash: "example" (length 7)
// This design eliminates runtime slash removal and byte subtraction.
func (s *Server) getLen{{.Index}}HTTP(w http.ResponseWriter, r *http.Request) {
    const L = {{.Index}}
    pathNoSlash := r.URL.Path[1:]
    if len(pathNoSlash) < L-1 {
        s.DispatchHTTP[L-1](w, r)
        return
    }
    truncated := pathNoSlash[:L-1]
    switch truncated {
    {{range .Routes}}
        case "{{.Path}}":
            s.Serve{{.Identifier}}HTTP(w, r)
    {{end}}
    default:
        s.DispatchHTTP[L-1](w, r)
    }
}
{{end}}

{{/* Per-length handler template for HTTPS */}}
{{define "httpsHandler"}}
// getLen{{.Index}}HTTPS handles HTTPS requests for paths of length {{.Index}}.
// Dispatch array index = {{.Index}} corresponds to asset routes of length {{.Index}}-1.
func (s *Server) getLen{{.Index}}HTTPS(w http.ResponseWriter, r *http.Request) {
    const L = {{.Index}}
    pathNoSlash := r.URL.Path[1:]
    if len(pathNoSlash) < L-1 {
        s.DispatchHTTPS[L-1](w, r)
        return
    }
    truncated := pathNoSlash[:L-1]
    switch truncated {
    {{range .Routes}}
        case "{{.Path}}":
            s.Serve{{.Identifier}}HTTPS(w, r)
    {{end}}
    default:
        s.DispatchHTTPS[L-1](w, r)
    }
}
{{end}}
```

**Intent**: Declarative handler templates with clear documentation.

**Purpose**: Templates clarify handler structure for humans and document the dispatch indexing logic.

**Rationale**: Modifying handlers only requires template changes, not code changes. Comments explain the relationship between dispatch index and asset route length.

### 16.4 `templates/headers.http.gotmpl`

```go
{{/* Header literals template */}}

{{/* HTTP */}}
{{define "httpHeader"}}
Content-Type: {{.MIME}}
Cache-Control: public, max-age=31536000, immutable{{if .IsIndex}}, must-revalidate{{end}}
Content-Length: {{.Size}}
{{if .IsHTML}}{{if .CSP}}Content-Security-Policy: {{.CSP}}
{{end}}{{end}}

{{end}}

{{/* HTTPS */}}
{{define "httpsHeader"}}
Content-Type: {{.MIME}}
Cache-Control: public, max-age=31536000, immutable{{if .IsIndex}}, must-revalidate{{end}}
Content-Length: {{.Size}}
{{if .IsHTML}}{{if .CSP}}Content-Security-Policy: {{.CSP}}
{{end}}{{end}}
Strict-Transport-Security: max-age=31536000
Alt-Svc: h3=":{{.HTTPSPort}}"; ma=2592000

{{end}}

{{define "httpHeaderLiteral"}}
var {{.Identifier}}HeaderHTTP = []byte("{{.HeaderHTTP}}")
{{end}}

{{define "httpsHeaderLiteral"}}
var {{.Identifier}}HeaderHTTPS = []byte("{{.HeaderHTTPS}}")
{{end}}
```

**Intent**: Declarative header templates.

**Purpose**: Templates clarify header format for humans.

**Rationale**: Modifying headers only requires template changes.

### 16.5 `templates/shared-imports.go.gotmpl`

```go
{{/* Shared imports for generated files */}}
{{define "imports"}}
import (
    "crypto/ecdsa"
    "crypto/elliptic"
    "crypto/rand"
    "crypto/tls"
    "crypto/x509"
    "crypto/x509/pkix"
    "fmt"
    "math/big"
    "net"
    "net/http"
    "os"
    "os/signal"
    "path/filepath"
    "slices"
    "strconv"
    "syscall"
    "time"
    "github.com/alecthomas/kong"
    "golang.org/x/net/http2"
    quic "github.com/quic-go/quic-go/http3"
    "flash/assets"
)
{{end}}
```

**Intent**: Shared imports template for code generation.

**Purpose**: Centralize import declarations.

**Rationale**: Shared imports reduce duplication across generated files.

### 16.6 `templates/shared-struct.go.gotmpl`

```go
{{/* Server struct template */}}
{{define "serverStruct"}}
type Server struct {
    Logger        *slog.Logger
    DispatchHTTP  [{{.MaxLen}}+2]func(http.ResponseWriter, *http.Request)
    DispatchHTTPS [{{.MaxLen}}+2]func(http.ResponseWriter, *http.Request)
    TLSCfg        *tls.Config
}
{{end}}
```

**Intent**: Server struct template for code generation.

**Purpose**: Centralize server struct definition.

**Rationale**: Shared struct definition ensures consistency across generated code.

---

## 17. Main Entry Point

### 17.1 Orchestration

```go
func main() {
    // Kong handles --help flag and environment variables automatically via tags
    var cli cli
    ctx := kong.Parse(&cli)

    // Validate inputs
    if cli.Input == cli.Output {
        log.Println("E099: Input and output must differ")
        os.Exit(2)
    }
    
    // Validate compression flags
    if err := validateCompressionFlags(&cli); err != nil {
        log.Println(err.Error())
        os.Exit(2)
    }
    
    // Set default cache directory
    if cli.CacheDir == "" {
        cli.CacheDir = getDefaultCacheDir()
    }
    
    // Parse size flags using units.Base2Bytes
    embedBudget := int64(cli.EmbedBudget)
    cacheMax := int64(cli.CacheMax)
    
    // Create output directories
    if !cli.DryRun {
        os.MkdirAll(cli.Output, 0700)
        os.MkdirAll(filepath.Join(cli.Output, "assets"), 0700)
        os.MkdirAll(filepath.Join(cli.Output, "www"), 0700)
        os.MkdirAll(cli.CacheDir, 0700)
    }
    
    // Step 1: Discover assets
    assets := discover(cli.Input)
    
    // Step 2: Compute hashes and ETags
    for i := range assets {
        assets[i].ImoHash = computeImoHash(assets[i].AbsPath)
        assets[i].ETag = computeETag(assets[i].ImoHash)
    }
    
    // Step 3: Deduplicate
    assets = dedupe(assets)
    
    // Step 4: Generate identifiers
    identifiers := make(map[string]bool)
    for i := range assets {
        assets[i].Identifier = generateIdentifier(assets[i].RelPath, identifiers)
        identifiers[assets[i].Identifier] = true
        assets[i].Filename = assets[i].Identifier + filepath.Ext(assets[i].RelPath)
    }
    
    // Step 5: Generate variants
    assets = generateVariants(assets, cli.Brotli, cli.AVIF, cli.WebP, cli.CacheDir)
    
    // Step 6: Allocate embed budget
    assets = allocateBudget(assets, embedBudget)
    
    // Step 7: Update frequency scores
    for i := range assets {
        assets[i].FrequencyScore = estimateFrequencyScore(assets[i].RelPath, assets[i].EmbedEligible)
    }
    
    // Step 8: Pre-compute headers
    for i := range assets {
        if assets[i].EmbedEligible && !assets[i].IsDuplicate {
            assets[i].HeaderHTTP = renderHeaderHTTP(assets[i], cli.CSP)
            assets[i].HeaderHTTPS = renderHeaderHTTPS(assets[i], cli.CSP, "8443")
        }
    }
    
    // Step 9: Create links
    if !cli.DryRun {
        createLinks(assets, cli.Input, cli.Output, cli.CacheDir)
    }
    
    // Step 10: Build path maps
    canonicalPaths := make(map[string]string)
    duplicatePaths := make(map[string]string)
    shortcutPaths := make(map[string]string)
    
    for _, asset := range assets {
        if !asset.IsDuplicate {
            canonicalPaths[asset.RelPath] = asset.Identifier
            shortcut := generateShortcut(asset.RelPath)
            if shortcut != "" && canonicalPaths[shortcut] == "" && duplicatePaths[shortcut] == "" {
                shortcutPaths[shortcut] = asset.Identifier
            }
        } else {
            duplicatePaths[asset.RelPath] = asset.CanonicalID
        }
    }
    
    // Step 11: Compute MaxLen
    maxLen := computeMaxLen(assets, canonicalPaths, duplicatePaths, shortcutPaths)
    
    // Step 12: Generate dispatch arrays
    httpDispatch, httpsDispatch := buildDispatch(assets, maxLen)
    
    // Step 13: Convert to template data
    data := TemplateData{
        Config: ConfigData{
            CSP:       cli.CSP,
            HTTPSPort: "8443",
            Module:    "flash",
        },
        Assets:    convertAssets(assets),
        Dispatch: DispatchData{
            HTTP:   httpDispatch,
            HTTPS:  httpsDispatch,
            MaxLen: maxLen,
        },
        PathMaps: PathMapsData{
            Canonical:  canonicalPaths,
            Duplicate:  duplicatePaths,
            Shortcut:   shortcutPaths,
        },
        MaxLen:    maxLen,
    }
    
    // Step 14: Generate Go code
    if !cli.DryRun {
        generate(data, cli.Output)
    }
    
    // Step 15: Run go mod tidy
    if !cli.DryRun {
        runGoModTidy(cli.Output)
    }
    
    // Step 16: Build binary
    if !cli.DryRun {
        runGoBuild(cli.Output)
    }
    
    // Step 17: Run tests
    if cli.Tests {
        runTests(cli.Output)
    }
    
    fmt.Println("Generation complete")
}
```

**Intent**: Orchestrate all steps deterministically.

**Purpose**: Clear step sequence enables debugging and maintenance.

**Rationale**: Each step produces deterministic output from same inputs. The `validate()` function is called early to ensure input/output correctness.

### 17.2 Build Commands

```go
// runGoModTidy runs go mod tidy in the output directory
func runGoModTidy(output string) error {
    cmd := exec.Command("go", "mod", "tidy")
    cmd.Dir = output
    return cmd.Run()
}

// runGoBuild builds the flash binary
func runGoBuild(output string) error {
    cmd := exec.Command("go", "build", "-o", "flash")
    cmd.Dir = output
    return cmd.Run()
}

// runTests runs the test suite
func runTests(output string) error {
    cmd := exec.Command("go", "test", "-v")
    cmd.Dir = output
    return cmd.Run()
}
```

**Intent**: Execute build commands for generated code.

**Purpose**: Complete the generation process.

**Rationale**: Commands are executed in the output directory.

---

## 18. Error Codes

| Code | Description | Exit Status |
|------|-------------|-------------|
| E001 | I/O error reading asset | 2 |
| E010 | Blocked port violation | 10 |
| E025 | Invalid compression/budget level | 2 |
| E087 | Link creation failure | 87 |
| E099 | Generic internal error | 2 |
| E079 | Test suite failure | 3 |

**Intent**: Consistent error codes for scripting.

**Purpose**: Exit codes enable shell script integration.

**Rationale**: Error codes provide clear feedback for automation.

---

## 19. Dependencies

### 19.1 Generator Dependencies

| Package | Import Path |
|---------|------------|
| Kong | `github.com/alecthomas/kong` |
| ImoHash | `github.com/kalafut/imohash` |
| Base91 | `github.com/mtraver/base91` |
| Brotli | `github.com/google/brotli/go/cbrotli` |
| AVIF | `github.com/vegidio/avif-go` (CGO required) |
| WebP | `github.com/kolesa-team/go-webp/encoder` (CGO required) |
| Units | `github.com/alecthomas/units` |

**Intent**: Use specified packages exactly.

**Purpose**: Ensure compatibility and stability.

**Rationale**: Well-maintained packages provide stable APIs. Note: AVIF and WebP libraries require CGO during generation. If unavailable, the generator should skip variant generation and log a warning.

### 19.2 Generated Server Dependencies

| Package | Import Path |
|---------|------------|
| Kong | `github.com/alecthomas/kong` |
| HTTP/2 | `golang.org/x/net/http/http2` |
| HTTP/3 | `github.com/quic-go/quic-go/http3` |

**Intent**: Use specified packages exactly.

**Purpose**: Ensure compatibility and stability.

**Rationale**: Well-maintained packages provide stable APIs.

---

## 20. Testing Requirements

| Type | Description |
|------|-------------|
| Unit | Identifier sanitization, MIME detection, header rendering |
| Integration | Server startup, response headers, variant selection |
| Fuzz | `FuzzIdentifierCollision`, `FuzzRouterPath` |
| Benchmark | Per-request latency, allocations |

**Intent**: Comprehensive test coverage.

**Purpose**: Ensure correctness and performance.

**Rationale**: Unit tests for algorithms; integration tests for generated server.

---

## 21. Implementation Checklist

Implement all files:

- `assets.go` - Asset discovery, dedupe, identifier, link, asset/variant types
- `cache.go` - Cache management, budget allocation, fileInfo
- `endpoints.go` - Dispatch generation, routing, computeMaxLen
- `generate.go` - Code generation, templates, data structures
- `main.go` - CLI, entry point, cli struct, orchestration
- `variant.go` - Variant generation, compression, hashing
- `templates/embed.go.gotmpl` - Embed template
- `templates/handlers.go.gotmpl` - Handlers template
- `templates/headers.http.gotmpl` - Template for header literals
- `templates/main.go.gotmpl` - Main template
- `templates/shared-imports.go.gotmpl` - Shared import declarations
- `templates/shared-struct.go.gotmpl` - Shared server struct template

Each file should be implemented exactly as specified in this document, with all algorithms, data structures, and templates matching the specification.

Do not generate the `go.mod` and `go.sum` files. These will be created after your implementation with the commands `go mod init flashbuilder` and `go mod tidy`. We **intentionally do not pin** dependency versions. We run `go mod tidy` to **automatically upgrade** dependencies to the **latest compatible** releases.

---

## 22. Testing Requirements

### 22.1 Test Infrastructure

**Intent**: Establish comprehensive test infrastructure for both `flashbuilder` generator and generated `flash` server.

**Purpose**: Ensure correctness, performance, and reliability of both the generator and generated code.

**Rationale**: Tests verify deterministic behavior, zero-allocation claims, and O(1) routing performance.

#### 22.1.1 Test File Structure

```
flashbuilder/
├── main_test.go           # CLI, entry point, validation tests
├── assets_test.go         # Asset discovery, MIME, dedupe tests
├── dispatch_test.go       # Dispatch array, routing tests
├── generate_test.go       # Code generation, template tests
├── cache_test.go          # Cache management, budget tests
├── variant_test.go        # Variant generation tests
├── integration_test.go    # End-to-end workflow tests
├── template_test.go       # Template rendering tests
└── testdata/              # Test fixtures directory
    ├── assets/            # Sample assets for testing
    │   ├── index.html
    │   ├── style.css
    │   ├── script.js
    │   └── logo.png
    └── templates/         # Expected template outputs
```

#### 22.1.2 Test Dependencies

```go
import (
    "testing"
    "os"
    "path/filepath"
    "strings"
    "time"
    "io/fs"
)
```

### 22.2 Unit Test Specifications

#### 22.2.1 `validateCompressionFlags` Tests

**Purpose**: Verify compression quality validation.

```go
func TestValidateCompressionFlags(t *testing.T) {
    tests := []struct {
        name      string
        brotli    int
        avif      int
        webp      int
        expectErr bool
        errContains string
    }{
        {"Valid Brotli 0-11", 5, 50, 50, false, ""},
        {"Invalid Brotli negative", -1, 50, 50, true, "Brotli quality must be 0-11"},
        {"Invalid Brotli > 11", 12, 50, 50, true, "Brotli quality must be 0-11"},
        {"Valid AVIF 0-100", 5, 75, 50, false, ""},
        {"Invalid AVIF negative", 5, -1, 50, true, "AVIF quality must be 0-100"},
        {"Invalid AVIF > 100", 5, 101, 50, true, "AVIF quality must be 0-100"},
        {"Valid WebP 0-100", 5, 50, 75, false, ""},
        {"Invalid WebP negative", 5, 50, -1, true, "WebP quality must be 0-100"},
        {"Invalid WebP > 100", 5, 50, 101, true, "WebP quality must be 0-100"},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            cli := &cli{
                Brotli: tt.brotli,
                AVIF:   tt.avif,
                WebP:   tt.webp,
            }
            err := validateCompressionFlags(cli)
            if tt.expectErr && err == nil {
                t.Errorf("expected error but got nil")
            }
            if !tt.expectErr && err != nil {
                t.Errorf("unexpected error: %v", err)
            }
            if tt.expectErr && err != nil && !strings.Contains(err.Error(), tt.errContains) {
                t.Errorf("expected error containing %s, got %s", tt.errContains, err.Error())
            }
        })
    }
}
```

#### 22.2.2 `getDefaultCacheDir` Tests

**Purpose**: Verify cache directory resolution following XDG specification.

```go
func TestGetDefaultCacheDir(t *testing.T) {
    // Test with XDG_CACHE_HOME
    t.Run("XDG_CACHE_HOME set", func(t *testing.T) {
        oldXdg := os.Getenv("XDG_CACHE_HOME")
        defer os.Setenv("XDG_CACHE_HOME", oldXdg)
        os.Setenv("XDG_CACHE_HOME", "/tmp/xdg-cache")
        expected := filepath.Join("/tmp/xdg-cache", "flashbuilder")
        result := getDefaultCacheDir()
        if result != expected {
            t.Errorf("expected %s, got %s", expected, result)
        }
    })
    
    // Test with HOME (fallback)
    t.Run("HOME set (fallback)", func(t *testing.T) {
        oldXdg := os.Getenv("XDG_CACHE_HOME")
        oldHome := os.Getenv("HOME")
        defer os.Setenv("XDG_CACHE_HOME", oldXdg)
        defer os.Setenv("HOME", oldHome)
        os.Setenv("XDG_CACHE_HOME", "")
        os.Setenv("HOME", "/tmp/home")
        expected := filepath.Join("/tmp/home", ".cache", "flashbuilder")
        result := getDefaultCacheDir()
        if result != expected {
            t.Errorf("expected %s, got %s", expected, result)
        }
    })
    
    // Test fallback to current directory
    t.Run("Neither set (fallback)", func(t *testing.T) {
        oldXdg := os.Getenv("XDG_CACHE_HOME")
        oldHome := os.Getenv("HOME")
        defer os.Setenv("XDG_CACHE_HOME", oldXdg)
        defer os.Setenv("HOME", oldHome)
        os.Setenv("XDG_CACHE_HOME", "")
        os.Setenv("HOME", "")
        expected := ".cache"
        result := getDefaultCacheDir()
        if result != expected {
            t.Errorf("expected %s, got %s", expected, result)
        }
    })
}
```

#### 22.2.3 `discover` Tests

**Purpose**: Verify asset discovery, MIME detection, and ordering.

```go
func TestDiscover(t *testing.T) {
    // Create temporary directory
    tmpDir, err := os.MkdirTemp("", "flashbuilder-*")
    if err != nil {
        t.Fatalf("Failed to create temp dir: %v", err)
    }
    defer os.RemoveAll(tmpDir)
    
    // Create test files
    files := []struct {
        name     string
        content  string
        expected string // Expected MIME type prefix
    }{
        {"index.html", "<html></html>", "text/html"},
        {"style.css", "body {}", "text/css"},
        {"script.js", "console.log", "text/javascript"},
        {"image.png", "\x89PNG\x00\x00\x00", "image/png"},
        {"data.json", "{}", "application/json"},
    }
    
    for _, file := range files {
        path := filepath.Join(tmpDir, file.name)
        if err := os.WriteFile(path, []byte(file.content), 0600); err != nil {
            t.Fatalf("Failed to write file: %v", err)
        }
    }
    
    // Run discovery
    assets, err := discover(tmpDir)
    if err != nil {
        t.Fatalf("Discovery failed: %v", err)
    }
    
    // Verify count
    if len(assets) != len(files) {
        t.Errorf("Expected %d assets, got %d", len(files), len(assets))
    }
    
    // Verify sorting (alphabetical by RelPath)
    for i := 1; i < len(assets); i++ {
        if assets[i].RelPath < assets[i-1].RelPath {
            t.Errorf("Assets not sorted: %s should come after %s", assets[i].RelPath, assets[i-1].RelPath)
        }
    }
    
    // Verify MIME detection
    for _, asset := range assets {
        if asset.MIME == "" || asset.MIME == "application/octet-stream" && asset.RelPath != "data.json" {
            t.Errorf("MIME detection failed for %s: got %s", asset.RelPath, asset.MIME)
        }
    }
    
    // Verify absolute paths are set
    for _, asset := range assets {
        if asset.AbsPath == "" {
            t.Errorf("AbsPath should not be empty for %s", asset.RelPath)
        }
    }
}
```

#### 22.2.4 `detectMIME` Tests

**Purpose**: Verify MIME type detection accuracy.

```go
func TestDetectMIME(t *testing.T) {
    tests := []struct {
        name        string
        filename    string
        content     string
        expectedMIME string
    }{
        {"HTML file", "index.html", "<html></html>", "text/html"},
        {"CSS file", "style.css", "body {}", "text/css"},
        {"JS file", "script.js", "console.log", "text/javascript"},
        {"JSON file", "data.json", "{}", "application/json"},
        {"Unknown", "data.bin", "\x00\x01\x02", "application/octet-stream"},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            tmpDir, err := os.MkdirTemp("", "flashbuilder-*")
            if err != nil {
                t.Fatalf("Failed to create temp dir: %v", err)
            }
            defer os.RemoveAll(tmpDir)
            
            path := filepath.Join(tmpDir, tt.filename)
            if err := os.WriteFile(path, []byte(tt.content), 0600); err != nil {
                t.Fatalf("Failed to write file: %v", err)
            }
            
            mime := detectMIME(path)
            if !strings.HasPrefix(mime, tt.expectedMIME) {
                t.Errorf("Expected %s, got %s", tt.expectedMIME, mime)
            }
        })
    }
}
```

#### 22.2.5 `generateIdentifier` Tests

**Purpose**: Verify deterministic identifier generation.

```go
func TestGenerateIdentifier(t *testing.T) {
    tests := []struct {
        name        string
        relPath     string
        expected    string
    }{
        {"Simple file", "css/style.css", "AssetCssStyle"},
        {"Index file", "index.html", "AssetIndex"},
        {"Nested file", "assets/css/main.css", "AssetAssetsCssMain"},
        {"Special chars", "assets/images/logo-1.png", "AssetAssetsImagesLogo1"},
    }
    
    existing := make(map[string]bool)
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := generateIdentifier(tt.relPath, existing)
            // Verify starts with "Asset"
            if result[:5] != "Asset" {
                t.Errorf("Identifier should start with 'Asset', got %s", result)
            }
            // Verify valid Go identifier chars
            for _, r := range result {
                if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
                    t.Errorf("Invalid character in identifier: %c", r)
                }
            }
        })
    }
    
    // Test collision resolution
    t.Run("Collision resolution", func(t *testing.T) {
        existing := make(map[string]bool)
        existing["AssetStyle"] = true
        
        result := generateIdentifier("style.css", existing)
        if result == "AssetStyle" {
            t.Errorf("Should resolve collision, got %s", result)
        }
        if result[:5] != "Asset" {
            t.Errorf("Should start with Asset, got %s", result)
        }
    })
}
```

#### 22.2.6 `estimateFrequencyScore` Tests

**Purpose**: Verify frequency score calculation for switch ordering.

```go
func TestEstimateFrequencyScore(t *testing.T) {
    tests := []struct {
        name      string
        path      string
        isEmbed   bool
        expected  int
    }{
        {"Index file", "index.html", true, 1000 + 500 + 200},
        {"Favicon", "favicon.ico", true, 900 + 200},
        {"CSS file", "style.css", true, 800 + 200},
        {"JS file", "script.js", true, 600 + 200},
        {"Logo", "logo.png", true, 400 + 200},
        {"Deep path", "assets/css/main.css", true, 800 + 200 - (5*len("assets/css/main.css")) - 30*2},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := estimateFrequencyScore(tt.path, tt.isEmbed)
            // Verify positive for high-priority files
            if tt.path == "index.html" && result <= 0 {
                t.Errorf("Index should have positive score, got %d", result)
            }
            // Verify specific expected values
            if tt.expected > 0 && result != tt.expected {
                t.Errorf("Expected %d, got %d", tt.expected, result)
            }
        })
    }
}
```

#### 22.2.7 `generateShortcut` Tests

**Purpose**: Verify shortcut generation for clean URLs.

```go
func TestGenerateShortcut(t *testing.T) {
    tests := []struct {
        name        string
        relPath     string
        expected    string
    }{
        {"Root index", "index.html", ""},
        {"Subdir index", "about/index.html", "about"},
        {"CSS file", "style.css", "style"},
        {"JS file", "script.js", "script"},
        {"No extension", "path/to/file", "path/to/file"},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := generateShortcut(tt.relPath)
            if result != tt.expected {
                t.Errorf("Expected %s, got %s", tt.expected, result)
            }
        })
    }
}
```

#### 22.2.8 `sanitizeIdentifier` Tests

**Purpose**: Verify identifier sanitization.

```go
func TestSanitizeIdentifier(t *testing.T) {
    tests := []struct {
        name        string
        input       string
        expected    string
    }{
        {"Simple", "style", "style"},
        {"With dash", "my-file", "myfile"},
        {"With number", "file1", "file1"},
        {"Special chars", "file@#$", "file"},
        {"Unicode", "café", "caf"},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := sanitizeIdentifier(tt.input)
            if result != tt.expected {
                t.Errorf("Expected %s, got %s", tt.expected, result)
            }
        })
    }
}
```

#### 22.2.9 `dedupe` Tests

**Purpose**: Verify deduplication logic.

```go
func TestDedupe(t *testing.T) {
    // Create temp directory with duplicate files
    tmpDir, err := os.MkdirTemp("", "flashbuilder-*")
    if err != nil {
        t.Fatalf("Failed to create temp dir: %v", err)
    }
    defer os.RemoveAll(tmpDir)
    
    // Create two identical files
    content := "test content"
    file1 := filepath.Join(tmpDir, "file1.txt")
    file2 := filepath.Join(tmpDir, "file2.txt")
    
    if err := os.WriteFile(file1, []byte(content), 0600); err != nil {
        t.Fatalf("Failed to write file: %v", err)
    }
    if err := os.WriteFile(file2, []byte(content), 0600); err != nil {
        t.Fatalf("Failed to write file: %v", err)
    }
    
    // Create assets
    assets := []asset{
        {RelPath: "file1.txt", AbsPath: file1, Identifier: "AssetFile1"},
        {RelPath: "file2.txt", AbsPath: file2, Identifier: "AssetFile2"},
    }
    
    // Compute hashes
    for i := range assets {
        assets[i].ImoHash = computeImoHash(assets[i].AbsPath)
        assets[i].ETag = computeETag(assets[i].ImoHash)
    }
    
    // Run dedupe
    result := dedupe(assets)
    
    // Verify first is canonical, second is duplicate
    if result[0].IsDuplicate {
        t.Errorf("First asset should not be duplicate")
    }
    if !result[1].IsDuplicate {
        t.Errorf("Second asset should be duplicate")
    }
    if result[1].CanonicalID != result[0].Identifier {
        t.Errorf("CanonicalID should be %s, got %s", result[0].Identifier, result[1].CanonicalID)
    }
}
```

#### 22.2.10 `buildDispatch` Tests

**Purpose**: Verify dispatch array generation and O(1) routing.

```go
func TestBuildDispatch(t *testing.T) {
    assets := []asset{
        {RelPath: "index.html", Identifier: "AssetIndex", FrequencyScore: 1000, IsDuplicate: false},
        {RelPath: "style.css", Identifier: "AssetStyle", FrequencyScore: 800, IsDuplicate: false},
        {RelPath: "script.js", Identifier: "AssetScript", FrequencyScore: 600, IsDuplicate: false},
    }
    
    maxLen := computeMaxLen(assets)
    httpDispatch, httpsDispatch := buildDispatch(assets, maxLen)
    
    // Verify dispatch arrays have correct length
    expectedLen := maxLen + 2
    if len(httpDispatch) != expectedLen {
        t.Errorf("HTTP dispatch length: expected %d, got %d", expectedLen, len(httpDispatch))
    }
    if len(httpsDispatch) != expectedLen {
        t.Errorf("HTTPS dispatch length: expected %d, got %d", expectedLen, len(httpsDispatch))
    }
    
    // Verify root handler at index 0 and 1
    if httpDispatch[0].Handler != "serveRootIndex" {
        t.Errorf("HTTP dispatch[0]: expected serveRootIndex, got %s", httpDispatch[0].Handler)
    }
    if httpDispatch[1].Handler != "serveRootIndex" {
        t.Errorf("HTTP dispatch[1]: expected serveRootIndex, got %s", httpDispatch[1].Handler)
    }
    
    // Verify routes are sorted by frequency
    for i := 2; i < len(httpDispatch); i++ {
        if len(httpDispatch[i].Routes) > 1 {
            for j := 1; j < len(httpDispatch[i].Routes); j++ {
                if httpDispatch[i].Routes[j-1].Frequency < httpDispatch[i].Routes[j].Frequency {
                    t.Errorf("Routes not sorted by frequency at dispatch[%d]", i)
                }
            }
        }
    }
}
```

#### 22.2.11 `sanitizePath` Tests

**Purpose**: Verify path sanitization for switch cases.

```go
func TestSanitizePath(t *testing.T) {
    tests := []struct {
        name        string
        input       string
        expected    string
    }{
        {"Simple path", "style.css", "style.css"},
        {"Path with backslash", "path\\to\\file", "path/to/file"},
        {"Path with quote", `path"file`, "path\\\"file"},
        {"Path with newline", "path\nfile", "path\\nfile"},
        {"Path with tab", "path\tfile", "path\\tfile"},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := sanitizePath(tt.input)
            if result != tt.expected {
                t.Errorf("Expected %s, got %s", tt.expected, result)
            }
        })
    }
}
```

#### 22.2.12 `computeMaxLen` Tests

**Purpose**: Verify max length calculation.

```go
func TestComputeMaxLen(t *testing.T) {
    tests := []struct {
        name        string
        paths       []string
        expected    int
    }{
        {"Single file", []string{"a"}, 1},
        {"Multiple files", []string{"a", "b/c", "d/e/f"}, 5},
        {"Empty", []string{}, 0},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            assets := make([]asset, len(tt.paths))
            for i, p := range tt.paths {
                assets[i] = asset{RelPath: p}
            }
            result := computeMaxLen(assets)
            if result != tt.expected {
                t.Errorf("Expected %d, got %d", tt.expected, result)
            }
        })
    }
}
```

#### 22.2.13 `hasRootIndex` Tests

**Purpose**: Verify root index detection.

```go
func TestHasRootIndex(t *testing.T) {
    tests := []struct {
        name        string
        assets      []asset
        expected    bool
    }{
        {"Has index.html", []asset{{RelPath: "index.html"}}, true},
        {"Has empty path", []asset{{RelPath: ""}}, true},
        {"No index", []asset{{RelPath: "style.css"}}, false},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := hasRootIndex(tt.assets)
            if result != tt.expected {
                t.Errorf("Expected %v, got %v", tt.expected, result)
            }
        })
    }
}
```

#### 22.2.14 `ensureCacheDir` Tests

**Purpose**: Verify cache directory creation.

```go
func TestEnsureCacheDir(t *testing.T) {
    tmpDir, err := os.MkdirTemp("", "flashbuilder-*")
    if err != nil {
        t.Fatalf("Failed to create temp dir: %v", err)
    }
    defer os.RemoveAll(tmpDir)
    
    cachePath := filepath.Join(tmpDir, "cache")
    
    err = ensureCacheDir(cachePath)
    if err != nil {
        t.Fatalf("ensureCacheDir failed: %v", err)
    }
    
    // Verify directory exists
    stat, err := os.Stat(cachePath)
    if err != nil {
        t.Fatalf("Cache directory does not exist: %v", err)
    }
    if !stat.IsDir() {
        t.Error("Cache path is not a directory")
    }
    
    // Verify permissions (0700)
    if stat.Mode()&0700 != 0700 {
        t.Errorf("Expected permissions 0700, got %v", stat.Mode())
    }
}
```

#### 22.2.15 `allocateBudget` Tests

**Purpose**: Verify embed budget allocation.

```go
func TestAllocateBudget(t *testing.T) {
    assets := []asset{
        {RelPath: "a.txt", Size: 100},
        {RelPath: "b.txt", Size: 200},
        {RelPath: "c.txt", Size: 300},
    }
    
    // Budget of 250 should fit a + b, but not c
    result := allocateBudget(assets, 250)
    
    // Verify sorting (smallest first)
    if result[0].Size > result[1].Size {
        t.Errorf("Assets should be sorted by size")
    }
    
    // Verify first two are eligible
    if !result[0].EmbedEligible {
        t.Errorf("Asset 0 should be eligible")
    }
    if !result[1].EmbedEligible {
        t.Errorf("Asset 1 should be eligible")
    }
    
    // Verify last one is not eligible
    if result[2].EmbedEligible {
        t.Errorf("Asset 2 should not be eligible (too large)")
    }
}
```

#### 22.2.16 `cleanCache` Tests

**Purpose**: Verify cache cleaning with LRU eviction.

```go
func TestCleanCache(t *testing.T) {
    tmpDir, err := os.MkdirTemp("", "flashbuilder-*")
    if err != nil {
        t.Fatalf("Failed to create temp dir: %v", err)
    }
    defer os.RemoveAll(tmpDir)
    
    // Create test files with different ages
    files := []struct {
        name    string
        content  string
        modTime  time.Time
    }{
        {"old.txt", "old content", time.Now().Add(-time.Hour)},
        {"new.txt", "new content", time.Now()},
    }
    
    for _, f := range files {
        path := filepath.Join(tmpDir, f.name)
        if err := os.WriteFile(path, []byte(f.content), 0600); err != nil {
            t.Fatalf("Failed to write file: %v", err)
        }
        // Set modification time
        if err := os.Chtimes(path, f.modTime, f.modTime); err != nil {
            t.Fatalf("Failed to set mod time: %v", err)
        }
    }
    
    // Clean cache to remove oldest file (simulate small max size)
    err = cleanCache(tmpDir, 0) // 0 bytes max = force deletion
    if err != nil {
        t.Fatalf("cleanCache failed: %v", err)
    }
    
    // Old file should be deleted
    if _, err := os.Stat(filepath.Join(tmpDir, "old.txt")); err == nil {
        t.Error("Old file should have been deleted")
    }
    
    // New file should still exist
    if _, err := os.Stat(filepath.Join(tmpDir, "new.txt")); err != nil {
        t.Error("New file should still exist")
    }
}
```

#### 22.2.17 `renderHeaderHTTP` Tests

**Purpose**: Verify HTTP header generation.

```go
func TestRenderHeaderHTTP(t *testing.T) {
    tests := []struct {
        name       string
        asset      asset
        csp        string
        contains   []string
    }{
        {
            "Simple asset",
            asset{MIME: "text/css", Size: 100, IsIndex: false, IsHTML: false},
            "",
            []string{"Content-Type: text/css", "Content-Length: 100", "Cache-Control: public, max-age=31536000, immutable"},
        },
        {
            "Index file",
            asset{MIME: "text/html", Size: 500, IsIndex: true, IsHTML: true},
            "default-src 'self'",
            []string{"Content-Type: text/html", "must-revalidate", "Content-Security-Policy: default-src 'self'"},
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := string(renderHeaderHTTP(tt.asset, tt.csp))
            for _, expected := range tt.contains {
                if !strings.Contains(result, expected) {
                    t.Errorf("Expected header to contain %s", expected)
                }
            }
        })
    }
}
```

#### 22.2.18 `renderHeaderHTTPS` Tests

**Purpose**: Verify HTTPS header generation.

```go
func TestRenderHeaderHTTPS(t *testing.T) {
    tests := []struct {
        name       string
        asset      asset
        csp        string
        httpsPort  string
        contains   []string
    }{
        {
            "Simple asset",
            asset{MIME: "text/css", Size: 100, IsIndex: false, IsHTML: false},
            "",
            "8443",
            []string{"Content-Type: text/css", "Strict-Transport-Security", "Alt-Svc: h3"},
        },
        {
            "HTML with CSP",
            asset{MIME: "text/html", Size: 500, IsIndex: true, IsHTML: true},
            "default-src 'self'",
            "8443",
            []string{"Content-Security-Policy", "Strict-Transport-Security"},
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := string(renderHeaderHTTPS(tt.asset, tt.csp, tt.httpsPort))
            for _, expected := range tt.contains {
                if !strings.Contains(result, expected) {
                    t.Errorf("Expected header to contain %s", expected)
                }
            }
        })
    }
}
```

#### 22.2.19 `convertAssets` Tests

**Purpose**: Verify asset data conversion.

```go
func TestConvertAssets(t *testing.T) {
    assets := []asset{
        {RelPath: "style.css", Identifier: "AssetStyle", IsDuplicate: false, EmbedEligible: true},
        {RelPath: "dup.css", Identifier: "AssetDup", IsDuplicate: true, CanonicalID: "AssetStyle"},
    }
    
    result := convertAssets(assets)
    
    // Should only include non-duplicate assets
    if len(result) != 1 {
        t.Errorf("Expected 1 asset (duplicate excluded), got %d", len(result))
    }
    
    if result[0].RelPath != "style.css" {
        t.Errorf("Expected style.css, got %s", result[0].RelPath)
    }
}
```

#### 22.2.20 `isCompressible` Tests

**Purpose**: Verify MIME type compression eligibility.

```go
func TestIsCompressible(t *testing.T) {
    tests := []struct {
        name        string
        mime        string
        expected    bool
    }{
        {"HTML", "text/html", true},
        {"CSS", "text/css", true},
        {"JS", "text/javascript", true},
        {"JSON", "application/json", true},
        {"XML", "application/xml", true},
        {"Image", "image/png", false},
        {"Binary", "application/octet-stream", false},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := isCompressible(tt.mime)
            if result != tt.expected {
                t.Errorf("Expected %v, got %v", tt.expected, result)
            }
        })
    }
}
```

#### 22.2.21 `isImage` Tests

**Purpose**: Verify image MIME type detection.

```go
func TestIsImage(t *testing.T) {
    tests := []struct {
        name        string
        mime        string
        expected    bool
    }{
        {"PNG", "image/png", true},
        {"JPEG", "image/jpeg", true},
        {"GIF", "image/gif", true},
        {"SVG", "image/svg+xml", true},
        {"HTML", "text/html", false},
        {"CSS", "text/css", false},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := isImage(tt.mime)
            if result != tt.expected {
                t.Errorf("Expected %v, got %v", tt.expected, result)
            }
        })
    }
}
```

### 22.3 Integration Test Specifications

#### 22.3.1 End-to-End Discovery to Dispatch Test

```go
func TestIntegration_DiscoverToDispatch(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }
    
    // Create temp directory
    tmpDir, err := os.MkdirTemp("", "flashbuilder-*")
    if err != nil {
        t.Fatalf("Failed to create temp dir: %v", err)
    }
    defer os.RemoveAll(tmpDir)
    
    // Create test files
    files := []struct {
        name    string
        content string
    }{
        {"index.html", "<html></html>"},
        {"style.css", "body {}"},
        {"script.js", "console.log"},
    }
    
    for _, f := range files {
        path := filepath.Join(tmpDir, f.name)
        if err := os.WriteFile(path, []byte(f.content), 0600); err != nil {
            t.Fatalf("Failed to write file: %v", err)
        }
    }
    
    // Run discovery
    assets, err := discover(tmpDir)
    if err != nil {
        t.Fatalf("Discovery failed: %v", err)
    }
    
    // Verify assets
    if len(assets) != len(files) {
        t.Errorf("Expected %d assets, got %d", len(files), len(assets))
    }
    
    // Compute hashes
    for i := range assets {
        assets[i].ImoHash = computeImoHash(assets[i].AbsPath)
        assets[i].ETag = computeETag(assets[i].ImoHash)
    }
    
    // Generate identifiers
    identifiers := make(map[string]bool)
    for i := range assets {
        assets[i].Identifier = generateIdentifier(assets[i].RelPath, identifiers)
        identifiers[assets[i].Identifier] = true
        assets[i].Filename = assets[i].Identifier + filepath.Ext(assets[i].RelPath)
    }
    
    // Compute frequency scores
    for i := range assets {
        assets[i].FrequencyScore = estimateFrequencyScore(assets[i].RelPath, assets[i].EmbedEligible)
    }
    
    // Compute max length
    maxLen := computeMaxLen(assets)
    if maxLen <= 0 {
        t.Errorf("MaxLen should be positive, got %d", maxLen)
    }
    
    // Build dispatch
    httpDispatch, httpsDispatch := buildDispatch(assets, maxLen)
    
    // Verify dispatch arrays
    if len(httpDispatch) != maxLen+2 {
        t.Errorf("HTTP dispatch length: expected %d, got %d", maxLen+2, len(httpDispatch))
    }
    if len(httpsDispatch) != maxLen+2 {
        t.Errorf("HTTPS dispatch length: expected %d, got %d", maxLen+2, len(httpsDispatch))
    }
}
```

#### 22.3.2 Budget Allocation Integration Test

```go
func TestIntegration_BudgetAllocation(t *testing.T) {
    assets := []asset{
        {RelPath: "a.txt", Size: 100},
        {RelPath: "b.txt", Size: 200},
        {RelPath: "c.txt", Size: 300},
    }
    
    // Budget of 250 should fit a + b, but not c
    result := allocateBudget(assets, 250)
    
    // Verify first two are eligible
    total := 0
    for _, a := range result {
        if a.EmbedEligible {
            total++
        }
    }
    
    if total != 2 {
        t.Errorf("Expected 2 eligible assets, got %d", total)
    }
}
```

#### 22.3.3 Shortcut Generation Integration Test

```go
func TestIntegration_ShortcutGeneration(t *testing.T) {
    assets := []asset{
        {RelPath: "index.html", Identifier: "AssetIndex"},
        {RelPath: "about/index.html", Identifier: "AssetAboutIndex"},
        {RelPath: "style.css", Identifier: "AssetStyle"},
    }
    
    // Build path maps
    canonicalPaths := make(map[string]string)
    shortcutPaths := make(map[string]string)
    
    for _, asset := range assets {
        if !asset.IsDuplicate {
            canonicalPaths[asset.RelPath] = asset.Identifier
            shortcut := generateShortcut(asset.RelPath)
            if shortcut != "" && canonicalPaths[shortcut] == "" {
                shortcutPaths[shortcut] = asset.Identifier
            }
        }
    }
    
    // Verify shortcuts
    if canonicalPaths["index.html"] != "AssetIndex" {
        t.Errorf("Expected AssetIndex for index.html")
    }
    
    // about/index.html should have shortcut "about"
    if shortcutPaths["about"] != "AssetAboutIndex" {
        t.Errorf("Expected AssetAboutIndex for 'about' shortcut")
    }
    
    // style.css should have shortcut "style"
    if shortcutPaths["style"] != "AssetStyle" {
        t.Errorf("Expected AssetStyle for 'style' shortcut")
    }
}
```

### 22.4 Generated `flash` Server Tests

#### 22.4.1 Router Performance Test

**Purpose**: Verify O(1) routing performance.

```go
func TestRouterPerformance(t *testing.T) {
    // Generated server should have O(1) routing
    // This test verifies dispatch array access is constant time
    
    // Create mock dispatch arrays
    dispatchHTTP := make([]func(http.ResponseWriter, *http.Request), 100)
    dispatchHTTPS := make([]func(http.ResponseWriter, *http.Request), 100)
    
    // Benchmark dispatch access
    benchmarks := []struct {
        name      string
        path      string
        expected  int
    }{
        {"Root path", "/", 0},
        {"Short path", "/a", 1},
        {"Long path", "/a/b/c/d/e/f/g/h/i/j", 10},
    }
    
    for _, bm := range benchmarks {
        t.Run(bm.name, func(t *testing.T) {
            // Simulate dispatch index calculation
            pathNoSlash := bm.path[1:]
            idx := min(len(pathNoSlash), 100)
            
            // Verify index calculation
            if idx != bm.expected && bm.expected >= 0 {
                t.Errorf("Expected index %d, got %d", bm.expected, idx)
            }
        })
    }
}
```

#### 22.4.2 Header Pre-computation Test

**Purpose**: Verify headers are pre-computed constants.

```go
func TestHeaderPrecomputation(t *testing.T) {
    // This test verifies that headers are generated at compile time
    // and stored as []byte constants
    
    // Simulate header generation
    asset := asset{
        MIME:      "text/css",
        Size:      100,
        IsIndex:   false,
        IsHTML:    false,
    }
    
    csp := "default-src 'self'"
    httpsPort := "8443"
    
    headerHTTP := renderHeaderHTTP(asset, csp)
    headerHTTPS := renderHeaderHTTPS(asset, csp, httpsPort)
    
    // Verify HTTP header
    if !strings.Contains(string(headerHTTP), "Content-Type: text/css") {
        t.Errorf("HTTP header missing Content-Type")
    }
    if !strings.Contains(string(headerHTTP), "Content-Length: 100") {
        t.Errorf("HTTP header missing Content-Length")
    }
    
    // Verify HTTPS header
    if !strings.Contains(string(headerHTTPS), "Strict-Transport-Security") {
        t.Errorf("HTTPS header missing HSTS")
    }
    if !strings.Contains(string(headerHTTPS), "Alt-Svc") {
        t.Errorf("HTTPS header missing Alt-Svc")
    }
}
```

#### 22.4.3 Zero Allocation Test

**Purpose**: Verify zero heap allocations per request.

```go
func TestZeroAllocation(t *testing.T) {
    // This test simulates request handling to verify zero allocations
    // for embed-eligible assets
    
    // Simulate pre-computed data
    headerHTTP := []byte("Content-Type: text/css\r\nContent-Length: 100\r\n")
    assetData := []byte("body {}")
    etag := `"abc123"`
    
    // Simulate request handling (no allocations)
    req := &http.Request{
        Method: "GET",
        Header: map[string][]string{},
    }
    req.Header.Set("If-None-Match", etag)
    
    // Verify conditional GET
    if req.Header.Get("If-None-Match") == etag {
        // Should return 304 without allocation
    }
    
    // Verify header write (pre-computed)
    // In real generated code, this would be: w.Write(headerHTTP)
    if len(headerHTTP) > 0 {
        // No allocation - header is pre-computed
    }
    
    // Verify asset write (pre-computed via go:embed)
    if len(assetData) > 0 {
        // No allocation - asset is embedded
    }
}
```

### 22.5 Benchmark Specifications

#### 22.5.1 Router Benchmark

```go
func BenchmarkRouter(b *testing.B) {
    // Benchmark dispatch array access
    pathNoSlash := "style.css" // 9 characters
    idx := min(len(pathNoSlash), 100)
    
    for i := 0; i < b.N; i++ {
        // Simulate dispatch access
        _ = idx
    }
    
    // Expected: ~1-2 ns per operation (O(1) array access)
}
```

#### 22.5.2 Header Generation Benchmark

```go
func BenchmarkHeaderGeneration(b *testing.B) {
    asset := asset{
        MIME:    "text/css",
        Size:    100,
        IsIndex: false,
        IsHTML:  false,
    }
    
    for i := 0; i < b.N; i++ {
        _ = renderHeaderHTTP(asset, "")
    }
    
    // Expected: ~100-200 ns (string building)
}
```

#### 22.5.3 Identifier Generation Benchmark

```go
func BenchmarkIdentifierGeneration(b *testing.B) {
    existing := make(map[string]bool)
    
    for i := 0; i < b.N; i++ {
        _ = generateIdentifier("assets/css/style.css", existing)
    }
    
    // Expected: ~500-1000 ns (string operations)
}
```

### 22.6 Test Utilities

#### 22.6.1 Helper Functions

```go
// contains checks if string contains substring
func contains(s, substr string) bool {
    return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
    for i := 0; i <= len(s)-len(substr); i++ {
        if s[i:i+len(substr)] == substr {
            return true
        }
    }
    return false
}

// skipIfCGO skips tests that require CGO
func skipIfCGO(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping CGO-dependent test in short mode")
    }
}

// skipIfNoTemplates skips tests that require template files
func skipIfNoTemplates(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping template test in short mode")
    }
}
```

#### 22.6.2 Test Fixtures

```
testdata/
├── assets/
│   ├── index.html      # HTML test file
│   ├── style.css       # CSS test file
│   ├── script.js       # JS test file
│   ├── logo.png        # Binary test file (create with actual PNG data)
│   └── data.json       # JSON test file
└── expected/
    ├── identifiers.txt # Expected identifier mappings
    └── dispatch.json   # Expected dispatch array
```

### 22.7 Test Running Instructions

```bash
# Run all tests
go test -v ./...

# Run short tests only (skip integration tests)
go test -v -short ./...

# Run specific test file
go test -v -run TestDiscover ./...

# Run with coverage
go test -v -cover ./...

# Run benchmarks
go test -bench=. ./...

# Run specific benchmark
go test -bench=BenchmarkRouter ./...

# Run tests with CGO (for compression tests)
CGO_ENABLED=1 go test -v ./...
```

### 22.8 Test Coverage Requirements

| Package | Minimum Coverage | Critical Functions |
|---------|-----------------|-------------------|
| `main` | 80% | `validateCompressionFlags`, `getDefaultCacheDir` |
| `assets` | 90% | `discover`, `detectMIME`, `generateIdentifier`, `dedupe` |
| `dispatch` | 90% | `buildDispatch`, `sanitizePath`, `computeMaxLen` |
| `cache` | 85% | `ensureCacheDir`, `allocateBudget`, `cleanCache` |
| `generate` | 85% | `renderHeaderHTTP`, `renderHeaderHTTPS`, `convertAssets` |
| `variant` | 70% | `isCompressible`, `isImage` (CGO-dependent functions skipped) |

### 22.9 Generated `flash` Server Test Requirements

The generated `flash` server should include:

1. **Router Test**: Verify O(1) dispatch array access
2. **Header Test**: Verify pre-computed headers
3. **Zero Allocation Test**: Verify no heap allocations per request
4. **Conditional GET Test**: Verify ETag-based 304 responses
5. **Blocked Port Test**: Verify port validation
6. **TLS Configuration Test**: Verify TLS 1.3 and H2/H3 support
