package main

import (
	"context"
	"log"
	"net/http"
	"text/template"

	"github.com/aminkamal/lol/internal/service"
	"github.com/aminkamal/lol/pkg/logger"
	"github.com/aws/aws-sdk-go-v2/config"
)

const (
	apiKey    = "<API_KEY>"
	awsRegion = "eu-west-1"
	useTLS    = false // set to true for prod
)

func main() {
	// Initialize logger
	if err := logger.Init("rito.log"); err != nil {
		panic("Failed to initialize logger: " + err.Error())
	}
	defer logger.Close()

	ctx := context.Background()

	var templates = template.Must(template.ParseFiles("templates/index.html"))

	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(awsRegion))
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	svc := service.New(cfg, apiKey)

	fs := http.FileServer(http.Dir("templates/static"))
	http.Handle("GET /static/", http.StripPrefix("/static/", fs))

	http.HandleFunc("GET /", svc.HandleLanding(templates))
	http.HandleFunc("GET /lol", svc.HandleIndex(templates))
	http.HandleFunc("POST /session", svc.HandleSession)
	http.HandleFunc("GET /chat", svc.EventHandler)
	http.HandleFunc("POST /chat", svc.ChatHandler)

	logger.Info("Server starting; useTLS=%v", useTLS)

	if useTLS {
		if err := http.ListenAndServeTLS(
			":443", "fullchain.pem", "privkey.pem", nil); err != nil {
			log.Fatal("TLS Server failed to start:", err)
		}
		return
	}

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
