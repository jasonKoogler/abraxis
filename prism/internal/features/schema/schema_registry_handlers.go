package schema

// import (
// 	"context"
// 	"fmt"
// 	"log"
// 	"net"
// 	"net/http"
// 	"os"
// 	"os/signal"
// 	"syscall"
// 	"time"

// 	"github.com/go-chi/chi/v5"
// 	"github.com/go-chi/chi/v5/middleware"
// 	"google.golang.org/grpc"
// 	"google.golang.org/grpc/reflection"

// 	"github.com/jasonKoogler/gauth/internal/config"
// 	"github.com/jasonKoogler/gauth/internal/repository"
// 	"github.com/jasonKoogler/gauth/internal/service"
// 	pb "github.com/jasonKoogler/gauth/proto/schema"
// )

// func main() {
// 	// Load configuration
// 	cfg, err := config.Load()
// 	if err != nil {
// 		log.Fatalf("Failed to load configuration: %v", err)
// 	}

// 	// Initialize repository
// 	repo, err := repository.NewRepository(cfg.Database)
// 	if err != nil {
// 		log.Fatalf("Failed to initialize repository: %v", err)
// 	}

// 	// Initialize services
// 	schemaService := service.NewSchemaService(repo)
// 	notificationService := service.NewNotificationService(cfg.Notification)
// 	validationService := service.NewValidationService()
// 	protoService := proto.NewProtoService()

// 	// Create gRPC server
// 	grpcServer := grpc.NewServer()
// 	schemaServer := service.NewSchemaRegistryServer(schemaService, notificationService, validationService)
// 	pb.RegisterSchemaRegistryServer(grpcServer, schemaServer)
// 	reflection.Register(grpcServer)

// 	// Create HTTP server with Chi router
// 	router := chi.NewRouter()
// 	router.Use(middleware.RequestID)
// 	router.Use(middleware.RealIP)
// 	router.Use(middleware.Logger)
// 	router.Use(middleware.Recoverer)
// 	router.Use(middleware.Timeout(30 * time.Second))

// 	// Register HTTP endpoints
// 	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
// 		w.Write([]byte("OK"))
// 	})

// 	// API routes
// 	router.Route("/api/v1", func(r chi.Router) {
// 		// Schema endpoints
// 		r.Route("/schemas", func(r chi.Router) {
// 			r.Get("/", service.HandleListSchemas(schemaService))
// 			r.Post("/", service.HandleCreateSchema(schemaService, validationService, notificationService))
// 			r.Get("/{service}/{name}/{version}", service.HandleGetSchema(schemaService))
// 			r.Put("/{service}/{name}/{version}", service.HandleUpdateSchema(schemaService, validationService, notificationService))
// 			r.Delete("/{service}/{name}/{version}", service.HandleDeleteSchema(schemaService, notificationService))
// 		})

// 		// Bundle endpoints
// 		r.Route("/bundles", func(r chi.Router) {
// 			r.Get("/", service.HandleListBundles(schemaService))
// 			r.Get("/{service}/{version}", service.HandleGetBundle(schemaService))
// 			r.Post("/{service}/{version}", service.HandleCreateBundle(schemaService, notificationService))
// 		})

// 		// Service endpoints
// 		r.Route("/services", func(r chi.Router) {
// 			r.Get("/", service.HandleListServices(schemaService))
// 			r.Post("/register", service.HandleRegisterService(schemaService, notificationService))
// 			r.Post("/heartbeat", service.HandleServiceHeartbeat(schemaService))
// 			r.Delete("/{serviceId}", service.HandleDeregisterService(schemaService, notificationService))
// 		})

// 		// Compatibility endpoints
// 		r.Route("/compatibility", func(r chi.Router) {
// 			r.Post("/check", service.HandleCheckCompatibility(validationService, schemaService))
// 			r.Get("/history/{service}/{name}", service.HandleGetCompatibilityHistory(schemaService))
// 		})

// 		// Proto compilation endpoints
// 		r.Route("/compile", func(r chi.Router) {
// 			r.Post("/", service.HandleCompileProto(protoService, schemaService))
// 		})
// 	})

// 	// Start gRPC server
// 	grpcAddr := fmt.Sprintf(":%d", cfg.GRPC.Port)
// 	grpcListener, err := net.Listen("tcp", grpcAddr)
// 	if err != nil {
// 		log.Fatalf("Failed to listen for gRPC: %v", err)
// 	}

// 	log.Printf("Starting gRPC server on %s", grpcAddr)
// 	go func() {
// 		if err := grpcServer.Serve(grpcListener); err != nil {
// 			log.Fatalf("Failed to serve gRPC: %v", err)
// 		}
// 	}()

// 	// Start HTTP server
// 	httpServer := &http.Server{
// 		Addr:    fmt.Sprintf(":%d", cfg.HTTP.Port),
// 		Handler: router,
// 	}

// 	log.Printf("Starting HTTP server on %s", httpServer.Addr)
// 	go func() {
// 		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
// 			log.Fatalf("Failed to serve HTTP: %v", err)
// 		}
// 	}()

// 	// Wait for termination signal
// 	signalChan := make(chan os.Signal, 1)
// 	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
// 	<-signalChan

// 	log.Println("Received termination signal, shutting down...")

// 	// Graceful shutdown
// 	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
// 	defer cancel()

// 	if err := httpServer.Shutdown(ctx); err != nil {
// 		log.Printf("HTTP server shutdown error: %v", err)
// 	}

// 	grpcServer.GracefulStop()
// 	log.Println("Server shutdown complete")
// }
