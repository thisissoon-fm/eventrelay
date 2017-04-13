#
#  Make binaries and run test suite
#

GOOUTDIR 		?= .
GOOS 			?=
GOARCH 			?=
TIME 			?= $(shell date +%s)
VERSION 		?= $(shell git rev-parse HEAD)
DOCKER_IMAGE	?= gcr.io/soon-fm-production/eventrelay
DOCKER_TAG		?= latest

.PHONY: linux darwin

# All Targets
all: test linux darwin

# Build Linux
linux%: GOOS = linux
linux: linux64

# Build Darwin
darwin%: GOOS = darwin
darwin: darwin64

# 64bit Archetecture
%64: GOARCH = amd64

# Common Build Target
linux64 darwin64:
	GOOS=$(GOOS) \
	GOARCH=$(GOARCH) \
	CGO_ENABLED=0 \
	go build -v \
		-ldflags "-X eventrelay/build.timestamp=$(TIME) -X eventrelay/build.version=$(VERSION) -X eventrelay/build.arch=$(GOARCH) -X eventrelay/build.os=$(GOOS)" \
		-o "$(GOOUTDIR)/eventrelay.$(GOOS)-$(GOARCH)"

# Run Test Suite
test:
	go test -v -cover $(shell go list ./... | grep -v ./vendor/)

# Docker Image
image: linux64
	docker build --force-rm -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

#
# Kubernetes
#

k8s:
	cat k8s.yml | sed 's#'\$$TAG'#$(DOCKER_TAG)#g' | kubectl apply -f -
