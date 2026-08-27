package main

import (
	"os"

	"github.com/Nyukimin/RenCrow_ASSISTANT/internal/verify"
)

func main() {
	os.Exit(verify.Run(os.Args[1:], os.Stdout, os.Stderr))
}
