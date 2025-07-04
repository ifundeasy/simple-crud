# # Build image
# docker buildx build --platform linux/amd64,linux/arm64 -t ifundeasy/simple-crud:latest --push .
# docker buildx imagetools inspect ifundeasy/simple-crud:latest

# # Run HTTP mode (default)
# docker run --env-file .env.docker -p 3000:3000 ifundeasy/simple-crud

# Run gRPC mode
# docker run --env-file .env.docker -e DEPLOYMENT_MODE=grpc -p 50051:50051 ifundeasy/simple-crud

# Run HTTP Client mode
# docker run --env-file .env.docker -e DEPLOYMENT_MODE=http-client ifundeasy/simple-crud

# Run gRPC Client mode
# docker run --env-file .env.docker -e DEPLOYMENT_MODE=grpc-client ifundeasy/simple-crud

# syntax=docker/dockerfile:1.4

########################
# Build stage
########################
FROM --platform=$BUILDPLATFORM golang:1.23.4 AS builder

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_TIME=unknown

ENV CGO_ENABLED=0

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod tidy && go mod download

COPY . .

RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags "-X 'simple-crud/internal/version.Version=${VERSION}' -X 'simple-crud/internal/version.Commit=${COMMIT}' -X 'simple-crud/internal/version.BuildTime=${BUILD_TIME}'" -o http-server-app ./cmd/http-server/main.go
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags "-X 'simple-crud/internal/version.Version=${VERSION}' -X 'simple-crud/internal/version.Commit=${COMMIT}' -X 'simple-crud/internal/version.BuildTime=${BUILD_TIME}'" -o grpc-server-app ./cmd/grpc-server/main.go
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags "-X 'simple-crud/internal/version.Version=${VERSION}' -X 'simple-crud/internal/version.Commit=${COMMIT}' -X 'simple-crud/internal/version.BuildTime=${BUILD_TIME}'" -o http-client-app ./cmd/http-client/main.go
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags "-X 'simple-crud/internal/version.Version=${VERSION}' -X 'simple-crud/internal/version.Commit=${COMMIT}' -X 'simple-crud/internal/version.BuildTime=${BUILD_TIME}'" -o grpc-client-app ./cmd/grpc-client/main.go
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags "-X 'simple-crud/internal/version.Version=${VERSION}' -X 'simple-crud/internal/version.Commit=${COMMIT}' -X 'simple-crud/internal/version.BuildTime=${BUILD_TIME}'" -o grpc-client-balanced-app ./cmd/grpc-client-balanced/main.go

########################
# Runtime stage (debian)
########################
FROM debian:bullseye-slim

RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*

WORKDIR /root/

COPY --from=builder /app/http-server-app .
COPY --from=builder /app/grpc-server-app .
COPY --from=builder /app/http-client-app .
COPY --from=builder /app/grpc-client-app .
COPY --from=builder /app/grpc-client-balanced-app .

EXPOSE 3000 50051

ENV DEPLOYMENT_MODE=http

CMD ["/bin/sh", "-c", "\
  if [ \"$DEPLOYMENT_MODE\" = \"http-server\" ]; then ./http-server-app; \
  elif [ \"$DEPLOYMENT_MODE\" = \"grpc-server\" ]; then ./grpc-server-app; \
  elif [ \"$DEPLOYMENT_MODE\" = \"http-client\" ]; then ./http-client-app; \
  elif [ \"$DEPLOYMENT_MODE\" = \"grpc-client\" ]; then ./grpc-client-app; \
  elif [ \"$DEPLOYMENT_MODE\" = \"grpc-client-balanced\" ]; then ./grpc-client-balanced-app; \
  else ./http-server-app; fi"]