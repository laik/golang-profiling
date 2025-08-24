#!/bin/bash

# Build script for golang-profiling Docker image

set -e

# Default values
IMAGE_NAME="golang-profiling"
IMAGE_TAG="latest"
REGISTRY=""
PUSH=false
NO_CACHE=false

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -n|--name)
            IMAGE_NAME="$2"
            shift 2
            ;;
        -t|--tag)
            IMAGE_TAG="$2"
            shift 2
            ;;
        -r|--registry)
            REGISTRY="$2"
            shift 2
            ;;
        -p|--push)
            PUSH=true
            shift
            ;;
        --no-cache)
            NO_CACHE=true
            shift
            ;;
        -h|--help)
            echo "Usage: $0 [OPTIONS]"
            echo "Options:"
            echo "  -n, --name NAME       Image name (default: golang-profiling)"
            echo "  -t, --tag TAG         Image tag (default: latest)"
            echo "  -r, --registry REG    Registry prefix (optional)"
            echo "  -p, --push            Push image to registry after build"
            echo "  --no-cache            Build without cache"
            echo "  -h, --help            Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0                                    # Build golang-profiling:latest"
            echo "  $0 -t v1.0.0                        # Build golang-profiling:v1.0.0"
            echo "  $0 -r myregistry.com -t v1.0.0 -p   # Build and push myregistry.com/golang-profiling:v1.0.0"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use -h or --help for usage information"
            exit 1
            ;;
    esac
done

# Construct full image name
if [[ -n "$REGISTRY" ]]; then
    FULL_IMAGE_NAME="$REGISTRY/$IMAGE_NAME:$IMAGE_TAG"
else
    FULL_IMAGE_NAME="$IMAGE_NAME:$IMAGE_TAG"
fi

echo "Building Docker image: $FULL_IMAGE_NAME"

# Build arguments
BUILD_ARGS=()
if [[ "$NO_CACHE" == "true" ]]; then
    BUILD_ARGS+=("--no-cache")
fi

# Build the image
echo "Running: docker build ${BUILD_ARGS[*]} -t $FULL_IMAGE_NAME ."
docker build "${BUILD_ARGS[@]}" -t "$FULL_IMAGE_NAME" .

echo "✅ Successfully built: $FULL_IMAGE_NAME"

# Push if requested
if [[ "$PUSH" == "true" ]]; then
    echo "Pushing image to registry..."
    docker push "$FULL_IMAGE_NAME"
    echo "✅ Successfully pushed: $FULL_IMAGE_NAME"
fi

# Show image info
echo ""
echo "Image information:"
docker images "$FULL_IMAGE_NAME" --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}\t{{.CreatedAt}}"

echo ""
echo "To run the image:"
echo "  docker run --rm $FULL_IMAGE_NAME --help"
echo "  docker run --rm --privileged --pid=host -v /proc:/proc:ro -v /sys:/sys:ro $FULL_IMAGE_NAME --pid 1 --duration 10"