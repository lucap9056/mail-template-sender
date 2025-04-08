package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/lucap9056/mail-template-sender/internal/api"
	"github.com/lucap9056/mail-template-sender/internal/lifecycle"
	"github.com/lucap9056/mail-template-sender/internal/smtp"
	"github.com/lucap9056/mail-template-sender/internal/template"
)

func main() {

	log.Println("Starting mail-template-sender service...")

	username := os.Getenv("SMTP_USERNAME")
	password := os.Getenv("SMTP_PASSWORD")
	host := os.Getenv("SMTP_HOST")
	port := os.Getenv("SMTP_PORT")
	templatesPath := os.Getenv("TEMPLATES_PATH")
	if templatesPath == "" {
		templatesPath = "./templates"
		log.Printf("TEMPLATES_PATH not set. Using default: %s\n", templatesPath)
	} else {
		log.Printf("Using templates path: %s\n", templatesPath)
	}

	log.Println("Initializing shutdown lifecycle handler...")
	shutdown := lifecycle.New()
	defer shutdown.Shutdown("")

	log.Println("Loading templates...")
	templates, err := template.New(templatesPath)
	if err != nil {
		log.Fatalf("Failed to load templates: %s\n", err.Error())
	}

	log.Println("Setting up SMTP configuration...")
	smtpConfig := &smtp.SMTPConfig{
		Username: username,
		Password: password,
		Host:     host,
		Port:     port,
		Address:  fmt.Sprintf("%s:%s", host, port),
	}

	log.Println("Initializing SMTP client...")
	client, err := smtp.New(smtpConfig)
	if err != nil {
		log.Fatalf("Failed to initialize SMTP client: %s\n", err.Error())
	}

	log.Println("Creating API service...")

	options := &api.Options{
		CertFile: os.Getenv("GRPC_CERT"),
		KeyFile:  os.Getenv("GRPC_KEY"),
	}
	app, err := api.New(client, templates, options)
	if err != nil {
		log.Fatalln(err.Error())
	}
	defer app.Stop()

	go func() {
		log.Println("Starting API server on port 50051...")
		err := app.Run(":50051")
		if err != nil {
			log.Printf("API server exited with error: %s\n", err.Error())
			shutdown.Shutdown(err.Error())
		}
	}()

	log.Println("Waiting for shutdown signal...")
	shutdown.Wait()

	log.Println("Shutdown signal received. Cleaning up...")

	time.Sleep(2 * time.Second)

	log.Println("Service stopped.")
}
