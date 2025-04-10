package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/lucap9056/mail-template-sender/internal/grpclistener"
	"github.com/lucap9056/mail-template-sender/internal/httplistener"
	"github.com/lucap9056/mail-template-sender/internal/lifecycle"
	"github.com/lucap9056/mail-template-sender/internal/smtp"
	"github.com/lucap9056/mail-template-sender/internal/template"
)

type ENV struct {
	username      string
	password      string
	host          string
	port          string
	templatesPath string
	caCertPath    string
	tlsCertPath   string
	tlsKeyPath    string
	listeners     map[string]struct{}
	grpcAddr      string
	httpAddr      string
}

func getEnabledListeners() map[string]struct{} {
	listeners := strings.ToLower(os.Getenv("LISTENERS"))
	listenerMap := make(map[string]struct{})

	if listeners == "" {
		listenerMap["grpc"] = struct{}{}
		listenerMap["http"] = struct{}{}
		return listenerMap
	}

	for _, listener := range strings.Split(listeners, ",") {
		listener = strings.TrimSpace(listener)
		if listener != "" {
			listenerMap[listener] = struct{}{}
		}
	}

	return listenerMap
}

func isTLSConfigured(cert, key string) bool {
	return cert != "" && key != ""
}

func main() {

	log.Println("Starting mail-template-sender service...")

	env := &ENV{
		username:      os.Getenv("SMTP_USERNAME"),
		password:      os.Getenv("SMTP_PASSWORD"),
		host:          os.Getenv("SMTP_HOST"),
		port:          os.Getenv("SMTP_PORT"),
		templatesPath: os.Getenv("TEMPLATES_PATH"),
		caCertPath:    os.Getenv("CA_CERT_PATH"),
		tlsCertPath:   os.Getenv("TLS_CERT_PATH"),
		tlsKeyPath:    os.Getenv("TLS_KEY_PATH"),
		listeners:     getEnabledListeners(),
		grpcAddr:      os.Getenv("GRPC_ADDR"),
		httpAddr:      os.Getenv("HTTP_ADDR"),
	}

	var tlsConfig *tls.Config

	if isTLSConfigured(env.tlsCertPath, env.tlsKeyPath) {
		config, err := readTLSConfig(
			env.caCertPath,
			env.tlsCertPath,
			env.tlsKeyPath,
		)

		if err != nil {
			log.Fatalln(err.Error())
		}
		tlsConfig = config
	}

	if env.templatesPath == "" {
		env.templatesPath = "./templates"
		log.Printf("TEMPLATES_PATH not set. Using default: %s\n", env.templatesPath)
	} else {
		log.Printf("Using templates path: %s\n", env.templatesPath)
	}

	log.Println("Initializing shutdown lifecycle handler...")
	shutdown := lifecycle.New()

	log.Println("Loading templates...")
	templates, err := template.New(env.templatesPath)
	if err != nil {
		log.Fatalf("Failed to load templates: %s\n", err.Error())
	}

	log.Println("Setting up SMTP configuration...")
	smtpConfig := &smtp.SMTPConfig{
		Username: env.username,
		Password: env.password,
		Host:     env.host,
		Port:     env.port,
		Address:  fmt.Sprintf("%s:%s", env.host, env.port),
	}

	log.Println("Initializing SMTP client...")
	client, err := smtp.New(smtpConfig)
	if err != nil {
		log.Fatalf("Failed to initialize SMTP client: %s\n", err.Error())
	}

	if _, ok := env.listeners["grpc"]; ok {

		log.Println("Creating gRPC listener service...")

		app, err := grpclistener.New(client, templates, tlsConfig)
		if err != nil {
			log.Fatalln(err.Error())
		}
		defer app.Stop()

		go func() {
			if env.grpcAddr == "" {
				env.grpcAddr = ":50051"
			}

			log.Printf("Starting gRPC listener on %s...\n", env.grpcAddr)
			err := app.Run(env.grpcAddr)
			if err != nil {
				log.Printf("gRPC listener exited with error: %s\n", err.Error())
				shutdown.Shutdown(err.Error())
			}
		}()

	}

	if _, ok := env.listeners["http"]; ok {

		log.Println("Creating HTTPS listener service...")

		app := httplistener.New(client, templates)
		defer app.Stop()

		go func() {

			if tlsConfig != nil {

				if env.httpAddr == "" {
					env.httpAddr = ":443"
				}

				log.Printf("Starting HTTPS listener on %s...\n", env.httpAddr)

				err := app.Run(env.httpAddr, tlsConfig)
				if err != nil {
					log.Printf("HTTPS listener exited with error: %s\n", err.Error())
					shutdown.Shutdown(err.Error())
				}

			} else {

				if env.httpAddr == "" {
					env.httpAddr = ":80"
				}

				log.Printf("Starting HTTP listener on %s...\n", env.httpAddr)

				err := app.Run(env.httpAddr, nil)
				if err != nil {
					log.Printf("HTTP listener exited with error: %s\n", err.Error())
					shutdown.Shutdown(err.Error())
				}

			}

		}()

	}

	log.Println("Waiting for shutdown signal...")
	shutdown.Wait()

	log.Println("Shutdown signal received. Cleaning up...")

	time.Sleep(2 * time.Second)

	log.Println("Service stopped.")
}

func readTLSConfig(caPath string, certPath string, keyPath string) (*tls.Config, error) {

	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load key pair: %v", err)
	}

	if caPath == "" {
		config := &tls.Config{
			Certificates: []tls.Certificate{cert},
			ClientAuth:   tls.NoClientCert,
		}
		log.Println("Server: Enabled One-way TLS")
		return config, nil
	} else {

		caCert, err := os.ReadFile(caPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA cert: %v", err)
		}
		caCertPool := x509.NewCertPool()
		if ok := caCertPool.AppendCertsFromPEM(caCert); !ok {
			return nil, fmt.Errorf("failed to append CA certs")
		}

		config := &tls.Config{
			ClientCAs:    caCertPool,
			Certificates: []tls.Certificate{cert},
			ClientAuth:   tls.RequireAndVerifyClientCert,
		}
		log.Println("Server: Enabled Mutual TLS")

		return config, nil
	}
}
