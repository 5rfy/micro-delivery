package main

import (
	"api-gateway/structs"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	deliverypb "github.com/5rfy/micro-delivery/proto/generated/delivery"
	orderpb "github.com/5rfy/micro-delivery/proto/generated/order"
	paymentpb "github.com/5rfy/micro-delivery/proto/generated/payment"

	"github.com/gorilla/mux"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Gateway struct {
	orderClient    orderpb.OrderServiceClient
	paymentClient  paymentpb.PaymentServiceClient
	deliveryClient deliverypb.DeliveryServiceClient
}

// POST /api/orders
func (g *Gateway) CreateOrder(writer http.ResponseWriter, request *http.Request) {
	var req structs.Order
	if err := json.NewDecoder(request.Body).Decode(&req); err != nil {
		respondError(writer, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.UserID == "" || len(req.Items) == 0 {
		respondError(writer, http.StatusBadRequest, "UserId and Items are required")
		return
	}

	pbItems := make([]*orderpb.OrderItem, len(req.Items))
	for i, item := range req.Items {
		pbItems[i] = &orderpb.OrderItem{
			ProductId: item.ProductId,
			Quantity:  item.Quantity,
			Price:     item.Price.InexactFloat64(),
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	order, err := g.orderClient.CreateOrder(ctx, &orderpb.CreateOrderRequest{
		UserId:          req.UserID,
		Items:           pbItems,
		DeliveryAddress: req.DeliveryAddress,
	})
	if err != nil {
		log.Printf("CreateOrder error: %v", err)
		respondError(writer, http.StatusInternalServerError, err.Error())
		return
	}
	respondJson(writer, http.StatusOK, order)
}

// GET /api/orders/{id}
func (g *Gateway) GetOrder(writer http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)
	orderID := vars["id"]

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	order, err := g.orderClient.GetOrder(ctx, &orderpb.GetOrderRequest{
		OrderId: orderID,
	})

	if err != nil {
		log.Printf("GetOrder error: %v", err)
		respondError(writer, http.StatusInternalServerError, "order not found")
		return
	}
	respondJson(writer, http.StatusOK, order)
}

// GET /api/orders/{id}/delivery
func (g *Gateway) GetDeliveryStatus(writer http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)
	orderID := vars["id"]

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	order, err := g.deliveryClient.GetDeliveryStatus(ctx, &deliverypb.GetDeliveryStatusRequest{
		OrderId: orderID,
	})

	if err != nil {
		log.Printf("GetDeliveryStatus error: %v", err)
		respondError(writer, http.StatusInternalServerError, "delivery status not found")
		return
	}

	respondJson(writer, http.StatusOK, order)
}

// GET /api/orders/{id}/payment
func (g *Gateway) GetPaymentStatus(writer http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)
	orderID := vars["id"]

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	status, err := g.paymentClient.GetPaymentStatus(ctx, &paymentpb.GetPaymentStatusRequest{
		OrderId: orderID,
	})
	if err != nil {
		log.Printf("GetPaymentStatus error: %v", err)
		respondError(writer, http.StatusInternalServerError, "payment status not found")
		return
	}

	respondJson(writer, http.StatusOK, status)
}

func respondJson(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		return
	}
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJson(w, status, map[string]string{"error": message})
}

func connectGrpc(addr string) *grpc.ClientConn {
	for i := range 10 {
		conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err == nil {
			return conn
		}
		log.Printf("Waiting for gRPC service at %s... attempt %d/10", addr, i+1)
		time.Sleep(3 * time.Second)
	}
	log.Fatalf("Failed to connect to gRPC service at %s", addr)
	return nil
}

func main() {
	orderAddr := os.Getenv("ORDER_SERVICE_ADDR")
	if orderAddr == "" {
		orderAddr = "localhost:50051"
	}

	paymentAddr := os.Getenv("PAYMENT_SERVICE_ADDR")
	if paymentAddr == "" {
		paymentAddr = "localhost:50052"
	}

	deliveryAddr := os.Getenv("DELIVERY_SERVICE_ADDR")
	if deliveryAddr == "" {
		deliveryAddr = "localhost:50053"
	}

	orderGrpc := connectGrpc(orderAddr)
	defer orderGrpc.Close()

	paymentGrpc := connectGrpc(paymentAddr)
	defer paymentGrpc.Close()

	deliveryGrpc := connectGrpc(deliveryAddr)
	defer deliveryGrpc.Close()

	gw := &Gateway{
		orderClient:    orderpb.NewOrderServiceClient(orderGrpc),
		paymentClient:  paymentpb.NewPaymentServiceClient(paymentGrpc),
		deliveryClient: deliverypb.NewDeliveryServiceClient(deliveryGrpc),
	}

	router := mux.NewRouter()

	router.Use(loggingMiddleware)
	router.Use(corsMiddleware)

	router.HandleFunc("/health", healthCheck).Methods("GET")
	router.HandleFunc("/api/orders", gw.CreateOrder).Methods("POST")
	router.HandleFunc("/api/orders/{id}", gw.GetOrder).Methods("GET")
	router.HandleFunc("/api/orders/{id}/delivery", gw.GetDeliveryStatus).Methods("GET")
	router.HandleFunc("/api/orders/{id}/payment", gw.GetPaymentStatus).Methods("GET")

	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("API Gateway listening on :%s", port)
	if err := http.ListenAndServe(":"+port, router); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func healthCheck(writer http.ResponseWriter, request *http.Request) {
	respondJson(writer, http.StatusOK, map[string]string{
		"status":    "UP",
		"service":   "apiGateway",
		"timestamp": time.Now().Format(time.RFC3339)})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.RequestURI, time.Since(start))
	})
}
