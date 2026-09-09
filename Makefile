.PHONY: fmt test vet build terraform-fmt terraform-validate verify

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

terraform-validate:
	terraform -chdir=deployments/terraform init -backend=false -input=false
	terraform -chdir=deployments/terraform validate

verify: test vet terraform-fmt
