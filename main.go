package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.opentelemetry.io/contrib/instrumentation/go.mongodb.org/mongo-driver/mongo/otelmongo"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

// Global variables
var (
	// The .env variables
	APP_PORT      string
	APP_NAME      string
	MONGO_URI     string
	MONGO_DB_NAME string
	EXTERNAL_API  string
	OTEL_RPC_URI  string

	// Non .env variables
	MongoClient    *mongo.Client
	Logger         *slog.Logger
	TracerProvider *trace.TracerProvider
)

func initConfig() {
	// Initialize Logger
	Logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Load .env file
	err := godotenv.Load()
	if err != nil {
		Logger.Warn("No .env file found, using system environment variables")
	}

	MONGO_URI = os.Getenv("MONGO_URI")
	if MONGO_URI == "" {
		Logger.Error("Missing required environment variables",
			slog.String("MONGO_URI", MONGO_URI),
		)
		os.Exit(1)
	}

	MONGO_DB_NAME = os.Getenv("MONGO_DB_NAME")
	if MONGO_DB_NAME == "" {
		Logger.Error("Missing required environment variables",
			slog.String("MONGO_DB_NAME", MONGO_DB_NAME),
		)
		os.Exit(1)
	}

	APP_PORT = os.Getenv("APP_PORT")
	if APP_PORT == "" {
		Logger.Error("Missing required environment variables",
			slog.String("APP_PORT", APP_PORT),
		)
		os.Exit(1)
	}

	APP_NAME = os.Getenv("APP_NAME")
	if APP_NAME == "" {
		Logger.Error("Missing required environment variables",
			slog.String("APP_NAME", APP_NAME),
		)
		os.Exit(1)
	}

	OTEL_RPC_URI = os.Getenv("OTEL_RPC_URI")
	if OTEL_RPC_URI == "" {
		Logger.Error("Missing required environment variables",
			slog.String("OTEL_RPC_URI", OTEL_RPC_URI),
		)
		os.Exit(1)
	}

	EXTERNAL_API = os.Getenv("EXTERNAL_API")
	if EXTERNAL_API == "" {
		Logger.Error("Missing required environment variables",
			slog.String("EXTERNAL_API", EXTERNAL_API),
		)
		os.Exit(1)
	}

	Logger.Info("Configuration loaded successfully",
		slog.String("APP_PORT", APP_PORT),
		slog.String("APP_NAME", APP_NAME),
		slog.String("MONGO_URI", MONGO_URI),
		slog.String("MONGO_DB_NAME", MONGO_DB_NAME),
		slog.String("OTEL_RPC_URI", OTEL_RPC_URI),
		slog.String("EXTERNAL_API", EXTERNAL_API),
	)
}

// initTracer: Setup OpenTelemetry tracing
func initTracer() {
	Logger.Info("Initializing OpenTelemetry Tracer")

	ctx := context.Background()

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithEndpoint(OTEL_RPC_URI),
		// otlptracegrpc.WithDialOption(grpc.WithBlock()),
		otlptracegrpc.WithCompressor("gzip"),
	)
	if err != nil {
		Logger.Error("Failed to create OTLP gRPC exporter", slog.String("error", err.Error()))
		os.Exit(1)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(APP_NAME),
			attribute.String("environment", "production"),
		),
	)
	if err != nil {
		Logger.Error("Failed to create OpenTelemetry resource", slog.String("error", err.Error()))
		os.Exit(1)
	}

	TracerProvider = trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(res),
	)

	// Set global Tracer Provider
	otel.SetTracerProvider(TracerProvider)

	Logger.Info("OpenTelemetry Tracer initialized successfully")
}

func initMongo() {
	var err error

	clientOptions := options.Client().ApplyURI(MONGO_URI).SetMonitor(otelmongo.NewMonitor())
	MongoClient, err = mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		Logger.Error("MongoDB connection error", slog.String("error", err.Error()))
		os.Exit(1)
	}

	err = MongoClient.Ping(context.TODO(), nil)
	if err != nil {
		Logger.Error("MongoDB ping failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	Logger.Info("Connected to MongoDB", slog.String("status", "success"))
}

type Product struct {
	ID    primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Name  string             `json:"name" bson:"name"`
	Price float64            `json:"price" bson:"price"`
	Stock int                `json:"stock" bson:"stock"`
}

func createProduct(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	tracer := otel.Tracer(APP_NAME)
	ctx, span := tracer.Start(ctx, "createProduct")
	defer span.End()

	var product Product
	err := json.NewDecoder(r.Body).Decode(&product)
	if err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	product.ID = primitive.NewObjectID()
	collection := MongoClient.Database(MONGO_DB_NAME).Collection("product")
	_, err = collection.InsertOne(context.TODO(), product)
	if err != nil {
		http.Error(w, "Failed to insert product", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(product)
}

func getProducts(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	tracer := otel.Tracer(APP_NAME)
	ctx, span := tracer.Start(ctx, "getProducts")
	defer span.End()

	collection := MongoClient.Database(MONGO_DB_NAME).Collection("product")
	cursor, err := collection.Find(ctx, bson.M{})
	if err != nil {
		http.Error(w, "Failed to fetch products", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var products []Product
	for cursor.Next(ctx) {
		var product Product
		err := cursor.Decode(&product)
		if err != nil {
			http.Error(w, "Error decoding product", http.StatusInternalServerError)
			return
		}
		products = append(products, product)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(products)
}

func getProductByID(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	tracer := otel.Tracer(APP_NAME)
	ctx, span := tracer.Start(ctx, "getProductByID")
	defer span.End()

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		http.Error(w, "Invalid ID format", http.StatusBadRequest)
		return
	}

	collection := MongoClient.Database(MONGO_DB_NAME).Collection("product")
	var product Product
	err = collection.FindOne(context.TODO(), bson.M{"_id": objID}).Decode(&product)
	if err != nil {
		http.Error(w, "Product not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(product)
}

func updateProduct(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	tracer := otel.Tracer(APP_NAME)
	ctx, span := tracer.Start(ctx, "updateProduct")
	defer span.End()

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		http.Error(w, "Invalid ID format", http.StatusBadRequest)
		return
	}

	var updatedProduct Product
	err = json.NewDecoder(r.Body).Decode(&updatedProduct)
	if err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	collection := MongoClient.Database(MONGO_DB_NAME).Collection("product")
	update := bson.M{
		"$set": bson.M{
			"name":  updatedProduct.Name,
			"price": updatedProduct.Price,
			"stock": updatedProduct.Stock,
		},
	}

	_, err = collection.UpdateOne(context.TODO(), bson.M{"_id": objID}, update)
	if err != nil {
		http.Error(w, "Failed to update product", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(bson.M{"message": "Product updated successfully"})
}

func deleteProduct(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	tracer := otel.Tracer(APP_NAME)
	ctx, span := tracer.Start(ctx, "deleteProduct")
	defer span.End()

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		http.Error(w, "Invalid ID format", http.StatusBadRequest)
		return
	}

	collection := MongoClient.Database(MONGO_DB_NAME).Collection("product")
	_, err = collection.DeleteOne(context.TODO(), bson.M{"_id": objID})
	if err != nil {
		http.Error(w, "Failed to delete product", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(bson.M{"message": "Product deleted successfully"})
}

// fetchExternalData: Call an external API
func fetchExternalData(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	tracer := otel.Tracer(APP_NAME)
	_, span := tracer.Start(ctx, "fetchExternalData")
	defer span.End()

	Logger.Info("Fetching data from external API", slog.String("url", EXTERNAL_API))

	resp, err := http.Get(EXTERNAL_API)
	if err != nil {
		Logger.Error("Failed to fetch external data", slog.String("error", err.Error()))
		http.Error(w, "Failed to fetch external data", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		Logger.Error("External API returned non-200 response", slog.Int("status_code", resp.StatusCode))
		http.Error(w, "External API error", resp.StatusCode)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		Logger.Error("Failed to read response body", slog.String("error", err.Error()))
		http.Error(w, "Failed to read response", http.StatusInternalServerError)
		return
	}

	Logger.Info("Successfully fetched external data")
	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}

func main() {
	initConfig()
	initTracer()
	initMongo()

	mux := http.NewServeMux()
	mux.HandleFunc("/products", func(w http.ResponseWriter, r *http.Request) {
		tracer := otel.Tracer(APP_NAME)
		ctx, span := tracer.Start(r.Context(), "handlerProducts")
		defer span.End()

		getProducts(ctx, w, r)
	})

	mux.HandleFunc("/product", func(w http.ResponseWriter, r *http.Request) {
		tracer := otel.Tracer(APP_NAME)
		ctx, span := tracer.Start(r.Context(), "handlerProduct")
		defer span.End()

		switch r.Method {
		case http.MethodGet:
			getProductByID(ctx, w, r)
		case http.MethodPost:
			createProduct(ctx, w, r)
		case http.MethodPut:
			updateProduct(ctx, w, r)
		case http.MethodDelete:
			deleteProduct(ctx, w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/external", func(w http.ResponseWriter, r *http.Request) {
		tracer := otel.Tracer(APP_NAME)
		ctx, span := tracer.Start(r.Context(), "handlerExternal")
		defer span.End()

		switch r.Method {
		case http.MethodGet:
			fetchExternalData(ctx, w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	server := &http.Server{
		Addr:         ":" + APP_PORT,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	Logger.Info("Server running", slog.String("address", server.Addr))
	err := server.ListenAndServe()
	if err != nil {
		Logger.Error("Server failed to start", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
