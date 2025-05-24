# # Build image
# docker build -t ifundeasy/simple-crud:latest .
# docker push ifundeasy/simple-crud:latest

# # Run HTTP mode (default)
# docker run --env-file .env.docker -p 3000:3000 ifundeasy/simple-crud

# Run gRPC mode
# docker run --env-file .env.docker -e APP_MODE=grpc -p 50051:50051 ifundeasy/simple-crud

# Run HTTP Client mode
# docker run --env-file .env.docker -e APP_MODE=http-client ifundeasy/simple-crud

# Run gRPC Client mode
# docker run --env-file .env.docker -e APP_MODE=grpc-client ifundeasy/simple-crud

# ===== Builder stage =====
FROM golang:1.23.4-alpine AS builder

ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /app

COPY . .

# Download modules
RUN go mod tidy && go mod download

# Build both binaries
RUN go build -o http-app ./cmd/http/main.go
RUN go build -o grpc-app ./cmd/grpc/main.go
RUN go build -o http-client-app ./cmd/http-client/main.go
RUN go build -o grpc-client-app ./cmd/grpc-client/main.go

# ===== Runtime stage =====
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy both apps
COPY --from=builder /app/http-app .
COPY --from=builder /app/grpc-app .
COPY --from=builder /app/http-client-app .
COPY --from=builder /app/grpc-client-app .

EXPOSE 3000 50051

# Runtime mode: http or grpc
ENV APP_MODE=http

CMD ["/bin/sh", "-c", "\
  if [ \"$APP_MODE\" = \"grpc\" ]; then ./grpc-app; \
  elif [ \"$APP_MODE\" = \"http-client\" ]; then ./http-client-app; \
  elif [ \"$APP_MODE\" = \"grpc-client\" ]; then ./grpc-client-app; \
  else ./http-app; fi"]
