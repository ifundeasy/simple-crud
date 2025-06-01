package grpc

import (
	"context"
	"errors"
	"log/slog"

	"simple-crud/internal/config"
	pb "simple-crud/internal/handler/grpc/pb"
	"simple-crud/internal/logger"
	"simple-crud/internal/model"
	"simple-crud/internal/service"
	"simple-crud/internal/utils"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

type ProductGRPCHandler struct {
	pb.UnimplementedProductServiceServer
	Service *service.ProductService
}

func NewProductGRPCHandler(svc *service.ProductService) *ProductGRPCHandler {
	return &ProductGRPCHandler{
		Service: svc,
	}
}

func (h *ProductGRPCHandler) logRequest(span trace.Span, function string, method string, payload any) {
	log := logger.Instance()
	log.Info("gRPC Request",
		slog.String("trace_id", span.SpanContext().TraceID().String()),
		slog.String("span_id", span.SpanContext().SpanID().String()),
		slog.String("function", function),
		slog.String("method", method),
		slog.String("hostname", utils.GetHost()),
		slog.Any("payload", payload),
	)
}

func (h *ProductGRPCHandler) GetAll(ctx context.Context, _ *emptypb.Empty) (*pb.ProductResN, error) {
	cfg := config.Instance()
	tracer := otel.Tracer(cfg.AppName)
	_, span := tracer.Start(ctx, "getProductsHandlerRpc")
	defer span.End()

	h.logRequest(span, "getProductsHandlerRpc", "GetAll", nil)

	products, err := h.Service.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	var protoProducts []*pb.Product
	for _, p := range products {
		protoProducts = append(protoProducts, &pb.Product{
			Id:    p.ID.Hex(),
			Name:  p.Name,
			Price: p.Price,
			Stock: int32(p.Stock),
		})
	}

	return &pb.ProductResN{
		Resolver: utils.GetHost(),
		Products: protoProducts,
	}, nil
}

func (h *ProductGRPCHandler) GetByID(ctx context.Context, req *pb.ProductId) (*pb.ProductRes1, error) {
	cfg := config.Instance()
	tracer := otel.Tracer(cfg.AppName)
	_, span := tracer.Start(ctx, "getProductByIdHandlerRpc")
	defer span.End()

	h.logRequest(span, "getProductByIdHandlerRpc", "GetByID", req)

	product, err := h.Service.GetByID(ctx, req.GetId())
	if err != nil {
		return nil, err
	}

	return &pb.ProductRes1{
		Resolver: utils.GetHost(),
		Product: &pb.Product{
			Id:    product.ID.Hex(),
			Name:  product.Name,
			Price: product.Price,
			Stock: int32(product.Stock),
		},
	}, nil
}

func (h *ProductGRPCHandler) Create(ctx context.Context, req *pb.Product) (*pb.ProductRes1, error) {
	cfg := config.Instance()
	tracer := otel.Tracer(cfg.AppName)
	_, span := tracer.Start(ctx, "createProductHandlerRpc")
	defer span.End()

	h.logRequest(span, "createProductHandlerRpc", "Create", req)

	product := &model.Product{
		Name:  req.GetName(),
		Price: req.GetPrice(),
		Stock: int(req.GetStock()),
	}

	created, err := h.Service.Create(ctx, product)
	if err != nil {
		return nil, err
	}

	return &pb.ProductRes1{
		Resolver: utils.GetHost(),
		Product: &pb.Product{
			Id:    created.ID.Hex(),
			Name:  created.Name,
			Price: created.Price,
			Stock: int32(created.Stock),
		},
	}, nil
}

func (h *ProductGRPCHandler) Update(ctx context.Context, req *pb.Product) (*pb.ProductRes1, error) {
	cfg := config.Instance()
	tracer := otel.Tracer(cfg.AppName)
	_, span := tracer.Start(ctx, "updateProductByIdHandlerRpc")
	defer span.End()

	h.logRequest(span, "updateProductByIdHandlerRpc", "Update", req)

	if req.GetId() == "" {
		return nil, errors.New("id is required")
	}

	p := model.Product{
		Name:  req.GetName(),
		Price: req.GetPrice(),
		Stock: int(req.GetStock()),
	}

	err := h.Service.Update(ctx, req.GetId(), &p)
	if err != nil {
		return nil, err
	}

	return &pb.ProductRes1{
		Resolver: utils.GetHost(),
		Product:  req,
	}, nil
}

func (h *ProductGRPCHandler) Delete(ctx context.Context, req *pb.ProductId) (*emptypb.Empty, error) {
	cfg := config.Instance()
	tracer := otel.Tracer(cfg.AppName)
	_, span := tracer.Start(ctx, "deleteProductByIdHandlerRpc")
	defer span.End()

	h.logRequest(span, "deleteProductByIdHandlerRpc", "Delete", req)

	if err := h.Service.Delete(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
