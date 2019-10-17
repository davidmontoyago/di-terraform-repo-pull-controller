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
	make delete

fmt:
	$(GOCMD) fmt ./pkg/

gen:
	go mod vendor
	./hack/update-codegen.sh

delete:
	kubectl delete repos --all
	kubectl delete -f deployment/repo-pull-controller-deployment.yaml --ignore-not-found

deploy:
	make build
	make delete
	kubectl apply -f deployment/
