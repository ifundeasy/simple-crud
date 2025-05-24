protoc \
  --go_out=internal/handler/grpc/pb \
  --go-grpc_out=internal/handler/grpc/pb \
  --go_opt=paths=source_relative \
  --go-grpc_opt=paths=source_relative \
  -I proto \
  proto/product.proto
