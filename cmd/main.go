package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/lucap9056/go-lifecycle/lifecycle"
	"github.com/lucap9056/mail-template-sender/internal/grpclistener"
	"github.com/lucap9056/mail-template-sender/internal/httplistener"
	"github.com/lucap9056/mail-template-sender/internal/smtp"
	"github.com/lucap9056/mail-template-sender/internal/template"
)

type ENV struct {
	SMTP_USERNAME               string
	SMTP_PASSWORD               string
	SMTP_SERVER_ADDRESS         string
	EMAIL_TEMPLATES_DIRECTORY   string
	TLS_CA_CERTIFICATE_PATH     string
	TLS_SERVER_CERTIFICATE_PATH string
	TLS_SERVER_KEY_PATH         string
	ENABLED_LISTENERS           map[string]struct{}
	GRPC_LISTENER_ADDRESS       string
	HTTP_LISTENER_ADDRESS       string
}

func getEnabledListeners(enabledListeners string) map[string]struct{} {
	listeners := strings.ToLower(enabledListeners)
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
		SMTP_USERNAME:               os.Getenv("SMTP_USERNAME"),
		SMTP_PASSWORD:               os.Getenv("SMTP_PASSWORD"),
		SMTP_SERVER_ADDRESS:         os.Getenv("SMTP_SERVER_ADDRESS"),
		EMAIL_TEMPLATES_DIRECTORY:   os.Getenv("EMAIL_TEMPLATES_DIRECTORY"),
		TLS_CA_CERTIFICATE_PATH:     os.Getenv("TLS_CA_CERTIFICATE_PATH"),
		TLS_SERVER_CERTIFICATE_PATH: os.Getenv("TLS_SERVER_CERTIFICATE_PATH"),
		TLS_SERVER_KEY_PATH:         os.Getenv("TLS_SERVER_KEY_PATH"),
		ENABLED_LISTENERS:           getEnabledListeners(os.Getenv("ENABLED_LISTENERS")),
		GRPC_LISTENER_ADDRESS:       os.Getenv("GRPC_LISTENER_ADDRESS"),
		HTTP_LISTENER_ADDRESS:       os.Getenv("HTTP_LISTENER_ADDRESS"),
	}

	var tlsConfig *tls.Config

	if isTLSConfigured(env.TLS_SERVER_CERTIFICATE_PATH, env.TLS_SERVER_KEY_PATH) {
		config, err := readTLSConfig(
			env.TLS_CA_CERTIFICATE_PATH,
			env.TLS_SERVER_CERTIFICATE_PATH,
			env.TLS_SERVER_KEY_PATH,
		)

		if err != nil {
			log.Fatalln(err.Error())
		}
		tlsConfig = config
	}

	if env.EMAIL_TEMPLATES_DIRECTORY == "" {
		env.EMAIL_TEMPLATES_DIRECTORY = "./templates"
		log.Printf("TEMPLATES_PATH not set. Using default: %s\n", env.EMAIL_TEMPLATES_DIRECTORY)
	} else {
		log.Printf("Using templates path: %s\n", env.EMAIL_TEMPLATES_DIRECTORY)
	}

	life := lifecycle.New()

	log.Println("Loading templates...")
	templates, err := template.New(env.EMAIL_TEMPLATES_DIRECTORY)
	if err != nil {
		log.Fatalf("Failed to load templates: %s\n", err.Error())
	}

	log.Println("Setting up SMTP configuration...")
	smtpConfig := &smtp.SMTPConfig{
		Username: env.SMTP_USERNAME,
		Password: env.SMTP_PASSWORD,
		Address:  env.SMTP_SERVER_ADDRESS,
	}

	log.Println("Initializing SMTP client...")
	client, err := smtp.New(smtpConfig)
	if err != nil {
		log.Fatalf("Failed to initialize SMTP client: %s\n", err.Error())
	}
	defer client.Close()

	if _, ok := env.ENABLED_LISTENERS["grpc"]; ok {

		log.Println("Creating gRPC listener service...")

		app, err := grpclistener.New(client, templates, tlsConfig)
		if err != nil {
			log.Fatalln(err.Error())
		}
		defer app.Stop()

		go func() {
			if env.GRPC_LISTENER_ADDRESS == "" {
				env.GRPC_LISTENER_ADDRESS = ":50051"
			}

			log.Printf("Starting gRPC listener on %s...\n", env.GRPC_LISTENER_ADDRESS)
			err := app.Run(env.GRPC_LISTENER_ADDRESS)
			if err != nil {
				life.Exitf("gRPC listener exited with error: %s\n", err.Error())
			}
		}()

	}

	if _, ok := env.ENABLED_LISTENERS["http"]; ok {

		log.Println("Creating HTTPS listener service...")

		app := httplistener.New(client, templates)
		defer app.Stop()

		go func() {

			if tlsConfig != nil {

				if env.HTTP_LISTENER_ADDRESS == "" {
					env.HTTP_LISTENER_ADDRESS = ":443"
				}

				log.Printf("Starting HTTPS listener on %s...\n", env.HTTP_LISTENER_ADDRESS)

				err := app.Run(env.HTTP_LISTENER_ADDRESS, tlsConfig)
				if err != nil {
					life.Exitf("HTTPS listener exited with error: %s\n", err.Error())
				}

			} else {

				if env.HTTP_LISTENER_ADDRESS == "" {
					env.HTTP_LISTENER_ADDRESS = ":80"
				}

				log.Printf("Starting HTTP listener on %s...\n", env.HTTP_LISTENER_ADDRESS)

				err := app.Run(env.HTTP_LISTENER_ADDRESS, nil)
				if err != nil {
					life.Exitf("HTTP listener exited with error: %s\n", err.Error())
				}

			}

		}()

	}

	log.Println("Waiting for shutdown signal...")
	life.Wait(2 * time.Second)

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

		return config, nil
	}
}
