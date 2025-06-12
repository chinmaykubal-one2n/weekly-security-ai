package main

import (
	"os"
	"os/exec"
	"weeklysec/internal/api"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
)

func main() {
	// Load env variables if .env file exists
	_ = godotenv.Load()

	// Check if Trivy is available
	if _, err := exec.LookPath("trivy"); err != nil {
		log.Fatal().Msg("Trivy CLI not found in PATH. Please install Trivy to continue.")
	}

	// Create Gin engine
	r := gin.Default()

	// Setup routes
	routes := api.SetupRoutes()
	routes(r)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Info().Msgf("Starting server on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal().Err(err).Msg("Failed to start server")
	}
}
