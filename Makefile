BIN      ?= bin
SERVER   ?= http://localhost:8080
GOFLAGS  := -trimpath -ldflags="-s -w"

.PHONY: build test vet fmt keygen bootstrap serve agent-register agent-run docker clean

build:
	mkdir -p $(BIN)
	CGO_ENABLED=0 go build $(GOFLAGS) -o $(BIN)/goran-server ./webapp
	CGO_ENABLED=0 go build $(GOFLAGS) -o $(BIN)/goran-agent ./agent

test:
	go test -timeout 300s ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

## one-time setup ---------------------------------------------------------

keygen:            ## print a master key; export it as GORAN_MASTER_KEY before `make serve`
	go run ./webapp keygen

bootstrap:         ## create admin user, workspace and the first API token
	go run ./webapp bootstrap --user $(or $(USER_NAME),admin) --email $(or $(EMAIL),admin@example.com) --workspace $(or $(WORKSPACE),default)

## run ------------------------------------------------------------------------

serve:             ## needs GORAN_MASTER_KEY in the environment
	go run ./webapp serve

agent-register:    ## make agent-register TOKEN=gr_... NAME=runner-1 LABELS=eu
	go run ./agent register --server $(SERVER) --token $(TOKEN) --name $(or $(NAME),$(shell hostname)) --labels "$(LABELS)"

agent-run:
	go run ./agent run

## packaging ------------------------------------------------------------------

docker:
	docker build -f build/Dockerfile.server -t goran-server .
	docker build -f build/Dockerfile.agent -t goran-agent .

clean:
	rm -rf $(BIN) work
