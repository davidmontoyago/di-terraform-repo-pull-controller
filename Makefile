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
	GOOS=linux GOARCH=amd64 $(GOBUILD) ./

test:
	$(GOTEST) ./
	$(GOTEST) ./pkg/poller

clean:
	$(GOCLEAN)
	$(GOCLEAN) ./cmd/...
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
	docker build . -f Dockerfile.terraform-runner -t terraform-runner:latest
	docker build . -f Dockerfile.controller -t repo-pull-controller:latest
	make delete
	kubectl apply -f deployment/
