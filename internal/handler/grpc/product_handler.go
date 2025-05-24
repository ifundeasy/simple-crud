package grpc

import (
	"context"
	"errors"

	pb "simple-crud/internal/handler/grpc/pb"
	"simple-crud/internal/model"
	"simple-crud/internal/service"
	"simple-crud/pkg/logger"

	"log/slog"

	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

type ProductGRPCHandler struct {
	pb.UnimplementedProductServiceServer
	Service *service.ProductService
	Logger  *slog.Logger
}

func NewProductGRPCHandler(svc *service.ProductService, log *slog.Logger) *ProductGRPCHandler {
	return &ProductGRPCHandler{
		Service: svc,
		Logger:  log,
	}
}

func (h *ProductGRPCHandler) log(ctx context.Context, method string, payload any) {
	h.Logger.Info("gRPC Request",
		slog.String("method", method),
		slog.String("hostname", logger.Hostname),
		slog.Any("payload", payload),
	)
}

func (h *ProductGRPCHandler) GetAll(ctx context.Context, _ *emptypb.Empty) (*pb.ProductList, error) {
	h.log(ctx, "GetAll", nil)

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

	return &pb.ProductList{Products: protoProducts}, nil
}

func (h *ProductGRPCHandler) GetByID(ctx context.Context, req *pb.ProductId) (*pb.Product, error) {
	h.log(ctx, "GetByID", req)

	product, err := h.Service.GetByID(ctx, req.GetId())
	if err != nil {
		return nil, err
	}

	return &pb.Product{
		Id:    product.ID.Hex(),
		Name:  product.Name,
		Price: product.Price,
		Stock: int32(product.Stock),
	}, nil
}

func (h *ProductGRPCHandler) Create(ctx context.Context, req *pb.Product) (*pb.Product, error) {
	h.log(ctx, "Create", req)

	product := &model.Product{
		Name:  req.GetName(),
		Price: req.GetPrice(),
		Stock: int(req.GetStock()),
	}

	created, err := h.Service.Create(ctx, product)
	if err != nil {
		return nil, err
	}

	return &pb.Product{
		Id:    created.ID.Hex(),
		Name:  created.Name,
		Price: created.Price,
		Stock: int32(created.Stock),
	}, nil
}

func (h *ProductGRPCHandler) Update(ctx context.Context, req *pb.Product) (*pb.Product, error) {
	h.log(ctx, "Update", req)

	if req.GetId() == "" {
		return nil, errors.New("id is required")
	}

	p := model.Product{
		Name:  req.GetName(),
		Price: req.GetPrice(),
		Stock: int(req.GetStock()),
	}

	err := h.Service.Update(ctx, req.GetId(), p)
	if err != nil {
		return nil, err
	}

	return req, nil
}

func (h *ProductGRPCHandler) Delete(ctx context.Context, req *pb.ProductId) (*emptypb.Empty, error) {
	h.log(ctx, "Delete", req)

	if err := h.Service.Delete(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
