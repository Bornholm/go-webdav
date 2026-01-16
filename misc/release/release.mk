GORELEASER_ARGS ?= release --auto-snapshot --clean

release: tools/goreleaser/bin/goreleaser
	REPO_OWNER=$(shell whoami) tools/goreleaser/bin/goreleaser $(GORELEASER_ARGS)

tools/goreleaser/bin/goreleaser:
	mkdir -p tools/goreleaser/bin
	GOBIN=$(PWD)/tools/goreleaser/bin go install github.com/goreleaser/goreleaser/v2@latest