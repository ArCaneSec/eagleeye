package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ArCaneSec/eagleeye/internal/server"
	"github.com/joho/godotenv"
)

func main() {
	ex, _ := os.Executable()
	exPath := filepath.Dir(ex)

	godotenv.Load(fmt.Sprintf("%s/../.env", exPath))
	server.InitializeEagleEye()
}
