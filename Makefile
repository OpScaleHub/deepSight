IMG ?= ghcr.io/opscalehub/deepsight/workload:latest
PLATFORMS ?= linux/amd64,linux/arm64

.PHONY: help docker-buildx

help:
	@echo "Available targets:"
	@echo "  make docker-buildx IMG=ghcr.io/org/repo/name:tag PLATFORMS=linux/amd64,linux/arm64"

docker-buildx:
	@echo "Building multi-arch image: ${IMG} for ${PLATFORMS}"
	@docker buildx build --platform ${PLATFORMS} --tag ${IMG} --push .
