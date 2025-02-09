# docker build -t simple-crud .
# docker run --env-file .env.docker -p 8080:3000 simple-crud


FROM golang:1.23.4-alpine AS builder

ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /app

COPY . .

# Download dependencies & build binary
RUN go mod tidy && go mod download
RUN go build -o app ./main.go

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/app .

EXPOSE 3000

CMD ["./app"]
