IMG ?= quay.io/eformat/c2o-plugin:latest

.PHONY: compile
compile:
	yarn build

.PHONY: go-build
go-build:
	CGO_ENABLED=0 go build -o backend ./cmd/backend/

.PHONY: podman-build
podman-build: compile
	podman build -t $(IMG) .

.PHONY: podman-build-nocompile
podman-build-nocompile:
	podman build -t $(IMG) .

.PHONY: podman-push
podman-push: podman-build
	podman push $(IMG)

.PHONY: podman-login
podman-login:
	podman login quay.io

.PHONY: helm-install
helm-install:
	helm upgrade --install c2o-plugin chart/c2o-plugin -n c2o-plugin --create-namespace

.PHONY: helm-uninstall
helm-uninstall:
	helm uninstall c2o-plugin -n c2o-plugin
