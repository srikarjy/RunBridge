.PHONY: fmt test vet build terraform-fmt verify

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

test:
	go test -race ./...

vet:
	go vet ./...

build:
	CGO_ENABLED=0 go build -trimpath -o ./runbridge ./cmd/runbridge

terraform-fmt:
	terraform fmt -check -recursive deployments/terraform

verify: test vet terraform-fmt
