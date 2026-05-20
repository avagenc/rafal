// Package main is the entry point for the Calendar Agent AWS Lambda function.
//
// Build for Lambda deployment:
//
//	GOOS=linux GOARCH=amd64 go build -tags lambda.norpc -o bootstrap cmd/agent/main.go
//
// Or for arm64 (Graviton):
//
//	GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o bootstrap cmd/agent/main.go
package main

import (
	"log"

	"avagenc-gmail/internal/handler"
	"avagenc-gmail/pkg/config"

	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	// Validate configuration at cold start — fail fast if env vars are missing.
	// This runs once per Lambda execution context (warm starts skip this).
	if _, err := config.Load(); err != nil {
		log.Fatalf("[main] failed to load configuration: %v", err)
	}

	log.Println("[main] Calendar Agent Lambda starting")

	// Start the Lambda handler.
	// Uses the newer lambda.Start which supports the response streaming model.
	lambda.Start(handler.HandleRequest)
}
