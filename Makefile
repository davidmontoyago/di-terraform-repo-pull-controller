# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GO111MODULE=on

all: test build

build:
	$(GOBUILD) ./
	docker build . -t repo-pull-controller:latest

test:
	$(GOTEST) ./

clean:
	$(GOCLEAN)

fmt:
	$(GOCMD) fmt ./pkg/

gen:
	./hack/update-codegen.sh
