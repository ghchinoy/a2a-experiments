package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/a2aproject/a2a-go/a2a"
	"github.com/a2aproject/a2a-go/a2aclient"
	"github.com/a2aproject/a2a-go/a2aclient/agentcard"
	"github.com/spf13/cobra"
)

var (
	serviceURL   string
	skillID      string
	authToken    string
	targetTaskID string
	refTaskID    string
)

type tokenInterceptor struct {
	a2aclient.PassthroughInterceptor
	token string
}

func (i *tokenInterceptor) Before(ctx context.Context, req *a2aclient.Request) (context.Context, error) {
	if i.token != "" {
		req.Meta["Authorization"] = []string{"Bearer " + i.token}
	}
	return ctx, nil
}

func main() {
	var rootCmd = &cobra.Command{
		Use:   "a2acli",
		Short: "A2A CLI Client",
	}

	rootCmd.PersistentFlags().StringVarP(&serviceURL, "service-url", "u", "http://127.0.0.1:9001", "Base URL of the A2A service")
	rootCmd.PersistentFlags().StringVarP(&authToken, "token", "t", "", "Auth token")
	rootCmd.PersistentFlags().StringVarP(&targetTaskID, "task", "k", "", "Existing Task ID to continue (must be non-terminal)")
	rootCmd.PersistentFlags().StringVarP(&refTaskID, "ref", "r", "", "Task ID to reference as context (works for completed tasks)")

	var describeCmd = &cobra.Command{
		Use:   "describe",
		Short: "Describe the agent card",
		Run: func(cmd *cobra.Command, args []string) {
			card, err := agentcard.DefaultResolver.Resolve(context.Background(), serviceURL)
			if err != nil {
				log.Fatalf("Error: %v", err)
			}
			fmt.Printf("Agent: %s\nSkills:\n", card.Name)
			for _, s := range card.Skills {
				fmt.Printf("  - [%s] %s\n", s.ID, s.Name)
			}
		},
	}

	var invokeCmd = &cobra.Command{
		Use:   "invoke [message]",
		Short: "Invoke a skill (streaming)",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			messageText := args[0]
			ctx := context.Background()

			card, err := agentcard.DefaultResolver.Resolve(ctx, serviceURL)
			if err != nil {
				log.Fatalf("Error: %v", err)
			}

			// 2. Create Client
		httpClient := &http.Client{Timeout: 15 * time.Minute}
			opts := []a2aclient.FactoryOption{a2aclient.WithJSONRPCTransport(httpClient)}
			if authToken != "" {
				opts = append(opts, a2aclient.WithInterceptors(&tokenInterceptor{token: authToken}))
			}

			client, err := a2aclient.NewFromCard(ctx, card, opts...)
			if err != nil {
				log.Fatalf("Error: %v", err)
			}

			// 3. Prepare Message
			msg := a2a.NewMessage(a2a.MessageRoleUser, a2a.TextPart{Text: messageText})
			if targetTaskID != "" {
				msg.TaskID = a2a.TaskID(targetTaskID)
				fmt.Printf("Continuing Task: %s\n", targetTaskID)
			}
			if refTaskID != "" {
				msg.ReferenceTasks = []a2a.TaskID{a2a.TaskID(refTaskID)}
				fmt.Printf("Referencing Task: %s\n", refTaskID)
			}

			params := &a2a.MessageSendParams{
				Message: msg,
			}
			if skillID != "" {
				params.Metadata = map[string]any{"skillId": skillID}
			}

			fmt.Printf("Invoking A2A Service (Streaming)...\n\n")

			var lastTaskID string

			for event, err := range client.SendStreamingMessage(ctx, params) {
				if err != nil {
					log.Fatalf("\nStream Error: %v", err)
				}

				if event.TaskInfo().TaskID != "" {
					lastTaskID = string(event.TaskInfo().TaskID)
				}

				switch v := event.(type) {
				case *a2a.Message:
					for _, p := range v.Parts {
						if tp, ok := p.(a2a.TextPart); ok {
							fmt.Printf("Agent: %s\n", tp.Text)
						}
					}
				case *a2a.TaskStatusUpdateEvent:
					state := v.Status.State
					msg := ""
					if v.Status.Message != nil && len(v.Status.Message.Parts) > 0 {
						if tp, ok := v.Status.Message.Parts[0].(a2a.TextPart); ok {
							msg = " - " + tp.Text
						}
					}
					fmt.Printf("[%s]%s\n", state, msg)
				case *a2a.TaskArtifactUpdateEvent:
					fmt.Printf("\n--- ARTIFACT RECEIVED: %s ---\n", v.Artifact.Name)
					fmt.Printf("Description: %s\n", v.Artifact.Description)
					for _, p := range v.Artifact.Parts {
						if dp, ok := p.(a2a.DataPart); ok {
							prettyJSON, _ := json.MarshalIndent(dp.Data, "", "  ")
							fmt.Printf("Data:\n%s\n", string(prettyJSON))
						} else if tp, ok := p.(a2a.TextPart); ok {
							fmt.Printf("Content: %s\n", tp.Text)
						}
					}
					fmt.Println("------------------------------")
				}
			}
			fmt.Printf("\nTask ID: %s (use --task %s to continue, or --ref %s to reference)\n", lastTaskID, lastTaskID, lastTaskID)
		},
	}

	invokeCmd.Flags().StringVarP(&skillID, "skill", "s", "", "Skill ID")
	rootCmd.AddCommand(describeCmd, invokeCmd)
	rootCmd.Execute()
}
