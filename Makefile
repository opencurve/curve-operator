
# Image URL to use all building/pushing image targets
IMG ?= harbor.cloud.netease.com/curve/curve-operator
# Image tag to use all building/pushing image targets
# TAG ?= $(shell git rev-parse --short HEAD)
TAG ?= v1.0.4
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# 1. make generator : generate hack/boilerplate.go.txt
# 2. make manfifests: generator crd/base and manager and rbac resources to manifests yaml
# 3. make install: install crd into cluster
# 4. make deploy: deploy all resource of manifests.yaml into cluster.
# 5. make run: generate fmt vet manifests then go run ./main.go

all: curve-operator

# Run tests
test: generate fmt vet manifests
	go test ./... -coverprofile cover.out -v

# Build curve-operator binary
curve-operator: generate fmt vet
	go build -o bin/curve-operator main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet
	go run ./main.go

# Install CRDs into a cluster
install: manifests
	kustomize build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests
	kustomize build config/crd | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	kubectl apply -f config/deploy

# Generate manifests e.g. CRD, RBAC etc.
manifests: generate
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=curve-operator-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases
	cd config/manager && kustomize edit set image harbor.cloud.netease.com/curve/curve-operator=${IMG}:${TAG}
	kustomize build config/default > config/deploy/manifests.yaml

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Build the docker image
docker-build: curve-operator test
	sudo docker build . -t ${IMG}:${TAG} --network host

# Push the docker image
docker-push:
	sudo docker push ${IMG}:${TAG}

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.5 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

KUSTOMIZE = $(GOBIN)/kustomize
kustomize: ## Download kustomize locally if necessary.
	$(call go-get-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v4@v4.0.5)

# go-get-tool will 'go get' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
go get $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef