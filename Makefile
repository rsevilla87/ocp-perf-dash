.PHONY: build binary container push clean help

# Variables
BINARY_NAME := ocp-perf-dash
IMAGE_REPO := quay.io
ORG := rsevilla
IMAGE_NAME := $(IMAGE_REPO)/$(ORG)/$(BINARY_NAME)
IMAGE_TAG := latest
CONTAINER_CMD := podman

# Build the binary
binary: clean
	@echo "Building $(BINARY_NAME)..."
	mkdir -p _output
	go build -o _output/$(BINARY_NAME) main.go

# Alias for binary
build: binary

# Build the container image
container: binary
	@echo "Building container image $(IMAGE_NAME):$(IMAGE_TAG)..."
	$(CONTAINER_CMD) build -t $(IMAGE_NAME):$(IMAGE_TAG) -f Containerfile .
	@echo "Container image built: $(IMAGE_NAME):$(IMAGE_TAG)"

# Build and push the container image
container-push: container
	@echo "Pushing container image $(IMAGE_NAME):$(IMAGE_TAG)..."
	$(CONTAINER_CMD) push $(IMAGE_NAME):$(IMAGE_TAG)

# Clean build artifacts
clean:
	rm -rf _output

# Show help
help:
	@echo "Available targets:"
	@echo "  build     - Build the binary (alias: binary)"
	@echo "  binary    - Build the binary"
	@echo "  container - Build the container image"
	@echo "  push      - Build and push the container image to quay.io"
	@echo "  clean     - Remove build artifacts"
	@echo "  help      - Show this help message"
	@echo ""
	@echo "Variables:"
	@echo "  BINARY_NAME   - Name of the binary (default: ocp-perf-dash)"
	@echo "  IMAGE_NAME    - Container image name (default: quay.io/rsevilla/ocp-perf-dash)"
	@echo "  IMAGE_TAG     - Container image tag (default: latest)"
	@echo "  CONTAINER_CMD - Container command (default: podman)"
