APP_NAME=ifundeasy/simple-crud
GIT_TAG=$(shell git describe --tags --abbrev=0 2>/dev/null || echo "dev")
COMMIT=$(shell git rev-parse --short HEAD)
BUILD_TIME=$(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

# multi-arch platform target
PLATFORMS=linux/amd64,linux/arm64

docker-build:
	docker buildx build \
		--platform $(PLATFORMS) \
		--build-arg VERSION=$(GIT_TAG) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		-t $(APP_NAME):$(GIT_TAG) \
		-t $(APP_NAME):latest \
		.

docker-push:
	docker push $(APP_NAME):$(GIT_TAG)
	docker push $(APP_NAME):latest