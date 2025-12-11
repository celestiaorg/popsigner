# BanhBaoRing Makefile
# Build all components

.PHONY: all build test lint docker-build docker-push help

# Versions
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
REGISTRY ?= ghcr.io/bidon15

# Image names
OPERATOR_IMAGE := $(REGISTRY)/banhbaoring-operator:$(VERSION)
CONTROL_PLANE_IMAGE := $(REGISTRY)/banhbaoring-control-plane:$(VERSION)
PLUGIN_IMAGE := $(REGISTRY)/banhbaoring-secp256k1:$(VERSION)

##@ General

help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Build

build: ## Build all Go binaries
	go build ./...
	cd plugin && go build ./...
	cd control-plane && go build ./...
	cd operator && go build ./...
	cd sdk-go && go build ./...

test: ## Run all tests
	go test ./...
	cd plugin && go test ./...
	cd control-plane && go test ./...
	cd operator && go test ./internal/... 
	cd sdk-go && go test ./...
	cd sdk-rust && cargo test

lint: ## Run linters
	golangci-lint run ./...
	cd plugin && golangci-lint run ./...
	cd control-plane && golangci-lint run ./...
	cd operator && golangci-lint run ./...

##@ Docker

docker-build: docker-build-operator docker-build-control-plane docker-build-plugin ## Build all Docker images

docker-build-operator: ## Build operator image
	docker build -t $(OPERATOR_IMAGE) ./operator

docker-build-control-plane: ## Build control-plane image
	docker build -t $(CONTROL_PLANE_IMAGE) -f ./control-plane/docker/Dockerfile ./control-plane

docker-build-plugin: ## Build plugin image
	docker build -t $(PLUGIN_IMAGE) ./plugin

docker-push: ## Push all images to registry
	docker push $(OPERATOR_IMAGE)
	docker push $(CONTROL_PLANE_IMAGE)
	docker push $(PLUGIN_IMAGE)

##@ Kind (Local K8s)

kind-create: ## Create kind cluster
	kind create cluster --name banhbaoring

kind-delete: ## Delete kind cluster
	kind delete cluster --name banhbaoring

kind-load: docker-build ## Load images into kind
	kind load docker-image $(OPERATOR_IMAGE) --name banhbaoring
	kind load docker-image $(CONTROL_PLANE_IMAGE) --name banhbaoring
	kind load docker-image $(PLUGIN_IMAGE) --name banhbaoring

##@ Helm

helm-install: ## Install operator via Helm
	helm install banhbaoring-operator ./operator/charts/banhbaoring-operator \
		-n banhbaoring-system --create-namespace \
		--set image.tag=$(VERSION)

helm-upgrade: ## Upgrade operator
	helm upgrade banhbaoring-operator ./operator/charts/banhbaoring-operator \
		-n banhbaoring-system \
		--set image.tag=$(VERSION)

helm-uninstall: ## Uninstall operator
	helm uninstall banhbaoring-operator -n banhbaoring-system

helm-lint: ## Lint Helm chart
	helm lint ./operator/charts/banhbaoring-operator

##@ Deploy

deploy-minimal: ## Deploy minimal cluster
	kubectl apply -f ./operator/config/samples/cluster_minimal.yaml

deploy-production: ## Deploy production cluster
	kubectl apply -f ./operator/config/samples/cluster_production.yaml

##@ Development

dev-setup: ## Setup development environment
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/a-h/templ/cmd/templ@latest
	cd operator && make envtest

generate: ## Generate code (CRDs, templ, etc.)
	cd operator && make generate manifests
	cd control-plane && templ generate

##@ Release

release: docker-build docker-push ## Build and push release
	@echo "Released $(VERSION)"
	@echo "  - $(OPERATOR_IMAGE)"
	@echo "  - $(CONTROL_PLANE_IMAGE)"
	@echo "  - $(PLUGIN_IMAGE)"

