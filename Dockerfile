# # Build image
# docker buildx build --platform linux/amd64,linux/arm64 -t ifundeasy/simple-crud:latest --push .
# docker buildx imagetools inspect ifundeasy/simple-crud:latest

# # Run HTTP mode (default)
# docker run --env-file .env.docker -p 3000:3000 ifundeasy/simple-crud

# Run gRPC mode
# docker run --env-file .env.docker -e APP_MODE=grpc -p 50051:50051 ifundeasy/simple-crud

# Run HTTP Client mode
# docker run --env-file .env.docker -e APP_MODE=http-client ifundeasy/simple-crud

# Run gRPC Client mode
# docker run --env-file .env.docker -e APP_MODE=grpc-client ifundeasy/simple-crud

# syntax=docker/dockerfile:1.4

########################
# Build stage
########################
FROM --platform=$BUILDPLATFORM golang:1.23.4 AS builder

ARG TARGETOS
ARG TARGETARCH

ENV CGO_ENABLED=0

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod tidy && go mod download

COPY . .

RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o http-app ./cmd/http/main.go
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o grpc-app ./cmd/grpc/main.go
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o http-client-app ./cmd/http-client/main.go
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o grpc-client-app ./cmd/grpc-client/main.go
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o grpc-client-balanced-app ./cmd/grpc-client-balanced/main.go

########################
# Runtime stage (debian)
########################
FROM debian:bullseye-slim

RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*

WORKDIR /root/

COPY --from=builder /app/http-app .
COPY --from=builder /app/grpc-app .
COPY --from=builder /app/http-client-app .
COPY --from=builder /app/grpc-client-app .
COPY --from=builder /app/grpc-client-balanced-app .

EXPOSE 3000 50051

ENV APP_MODE=http

CMD ["/bin/sh", "-c", "\
  if [ \"$APP_MODE\" = \"grpc\" ]; then ./grpc-app; \
  elif [ \"$APP_MODE\" = \"http-client\" ]; then ./http-client-app; \
  elif [ \"$APP_MODE\" = \"grpc-client\" ]; then ./grpc-client-app; \
  elif [ \"$APP_MODE\" = \"grpc-client-balanced\" ]; then ./grpc-client-balanced-app; \
  else ./http-app; fi"]