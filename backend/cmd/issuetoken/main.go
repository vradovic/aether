package main

import (
	"fmt"
	"os"

	"github.com/vradovic/aether/services/api/internal/core"
)

// For quickly issuing tokens
func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "Usage: issuetoken <userID> <signingKey>")
		os.Exit(1)
	}
	userID := os.Args[1]
	signingKey := os.Args[2]

	token, err := core.IssueToken(signingKey, userID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Issue Token: %s", err)
		os.Exit(1)
	}

	fmt.Printf("Token: %s\n", token)
}
