help:
	# make all    Upgrade, Generate, Format, Lint, Tests
	# make v00X   Upgrade the patch version of the dependencies
	# make v0XX   Upgrade the minor version of the dependencies
	# make fmt    Generate code and Format code
	# make fix    Modernize and lint auto-fix
	# make test   Check build and Test
	# make cov    Browse test coverage
	# make int    Run integration-tests (use T=timeout)
	# make clean  Remove code-coverage.out

.PHONY: all
all: v0XX fmt fix cov

go.mod:
	go mod init github.com/lynxai-team/emo
	go mod tidy

go.sum: go.mod
	go mod tidy

.PHONY: v00X
v00X: go.sum
	GOPROXY=direct go get -t -u=patch all
	go mod tidy

.PHONY: v0XX
v0XX: go.sum
	go get -u -t all
	go mod tidy

.PHONY: fmt
fmt: go.sum
	go generate ./...
	go run mvdan.cc/gofumpt@latest -w -extra -l .

.PHONY: test
test: code-coverage.out
	go build ./...

.PHONY: cov
cov: code-coverage.out
	go tool cover -html code-coverage.out

code-coverage.out: go.sum *.go */*.go Makefile
	go test -race -vet all -tags=emo -coverprofile=code-coverage.out ./...

.PHONY: fix
fix:
	go fix ./... || true
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest run --fix

.PHONY: clean
clean:
	rm -vf code-coverage.out

# Allow using a different timeout value, examples:
#    T=30s make vet
#    make vet T=1m
T ?= 10s

.PHONY: int
int: go.sum
	pkill -fe [/]exe/complete   || true
	pkill -fe [/]exe/low-level  || true
	pkill -fe [/]exe/keystore   || true
	pkill -fe [/]exe/httprouter || true
	pkill -fe [/]exe/chi        || true
	timeout $T go run -race ./examples/complete   ; [ 124 -le $$? ] && echo Terminated by timeout $T ; true
	timeout $T go run -race ./examples/low-level  ; [ 124 -le $$? ] && echo Terminated by timeout $T ; true
	timeout $T go run -race ./examples/keystore   ; [ 124 -le $$? ] && echo Terminated by timeout $T ; true
	timeout $T go run -race ./examples/httprouter ; [ 124 -le $$? ] && echo Terminated by timeout $T ; true
	timeout $T go run -race ./examples/chi        ; [ 124 -le $$? ] && echo Terminated by timeout $T ; true
