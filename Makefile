# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GO111MODULE=on

all: test build

build:
	cd ./cmd/terraform-runner/setup-terraform-repo && GOOS=linux GOARCH=amd64 $(GOBUILD) && cd -
	docker build . -f Dockerfile.terraform-runner -t terraform-runner:latest

	GOOS=linux GOARCH=amd64 $(GOBUILD) ./
	docker build . -f Dockerfile.controller -t repo-pull-controller:latest

test:
	$(GOTEST) ./

clean:
	$(GOCLEAN)

fmt:
	$(GOCMD) fmt ./pkg/

gen:
	./hack/update-codegen.sh

deploy:
	kubectl delete -f deployment/repo-pull-controller-deployment.yaml --ignore-not-found
	kubectl apply -f deployment/repo-pull-controller-deployment.yaml
