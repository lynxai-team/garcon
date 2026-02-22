# Elaborate a prompt that generates a specification

You are a prompt engineer expert in writing prompts for models specialized in Go.  Your task is to elaborate a prompt in English to request a model to specify a command-line tool in Go that does:

1. walk the `www` folder to list all static web files
2. generate a Go file in each folder with a variable for each file using  `//go:embed <filename>` (example: `//go:embed index.html`)
3. generate a Go functions `func (mux *server) serve<filename>(w http.ResponseWriter, r *http.Request)` for the corresponding web static files (the function returns the brotli-compressed version of the file if the client support it, for example, it returns `index.html.br` instead of `index.html`)
4. generate a `match` function in Go having all filenames hard-coded within a switch as the following:

```go
func (mux *server) handle(path string, w http.ResponseWriter, r *http.Request) {
    switch path {
        case "index.html": mux.serveIndexHtml(w,r)
        case "favicon.svg": mux.serveFaviconIco(w,r)
        case "css/style.css": mux.serveCssStyleCss(w,r)
        case "js/script.js": mux.serveJsScriptJs(w,r)
        case "images/logo.png": mux.serveImagesLogoPng(w,r)
        ...
        default: mux.cannotFindPage(path,w,r)
    }
}
```

Define pertinent and clear task and role. The prompt asks the specification, not the implementation. The ultimate goal is a Go command that search for web static files and generate a Go function. This first step is about elaborating the prompt that request a model to specify that generator command. Use the best practices in prompt engineering. Request to analyze carefully the prompt to grasp the intent, purpose, rationale and motivation, so the model can generate a specification according these intent, purpose, rationale and motivation. The model should feel free to complete the specification with other non-specified aspects and using the best practices about Go specification writing.

Output only the elaborated prompt.
