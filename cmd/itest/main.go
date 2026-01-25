package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"a2a-simple/pkg/interactions"
	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatal("GEMINI_API_KEY not set in .env")
	}

	client := interactions.NewClient(apiKey)
	ctx := context.Background()

	input := "Research the history of the Go language"
	if len(os.Args) > 1 {
		input = os.Args[1]
	}

	var interactionID string
	var currentStatus string

	// If input looks like an ID, use it directly
	if strings.HasPrefix(input, "v1_") {
		interactionID = input
		fmt.Printf("Checking existing interaction: %s\n", interactionID)
	} else {
		fmt.Printf("Starting Research: %q\n", input)
		
		req := &interactions.InteractionRequest{
			Input:      input,
			Agent:      "deep-research-pro-preview-12-2025",
			Background: true,
		}

		resp, err := client.Create(ctx, req)
		if err != nil {
			log.Fatalf("Create Error: %v", err)
		}
		interactionID = resp.ID
		currentStatus = resp.Status
		fmt.Printf("Interaction Created! ID: %s, Status: %s\n", interactionID, currentStatus)
	}

	// Poll
	for {
		fmt.Printf("[%s] Checking status...\n", time.Now().Format("15:04:05"))
		current, err := client.Get(ctx, interactionID)
		if err != nil {
			log.Printf("Get Error: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		fmt.Printf("Status: %s\n", current.Status)
		status := strings.ToLower(current.Status)
		if status != "working" && status != "in_progress" && status != "pending" {
			fmt.Println("\n--- FINAL RESPONSE ---")
			
			// Debug: print raw JSON
			rawJSON, _ := json.MarshalIndent(current, "", "  ")
			fmt.Printf("Raw JSON:\n%s\n", string(rawJSON))

			if len(current.Outputs) > 0 {
				for i, content := range current.Outputs {
					fmt.Printf("Output %d (Role: %s):\n", i, content.Role)
					if content.Text != "" {
						fmt.Printf("  Text: %s\n", content.Text)
					}
					for j, part := range content.Parts {
						fmt.Printf("  Part %d Text: %s\n", j, part.Text)
					}
				}
			} else {
				fmt.Println("No output found in final response.")
				// Debug: print raw struct content
				fmt.Printf("Raw Response Data: %+v\n", current)
			}
			break
		}

		time.Sleep(10 * time.Second)
	}
}
