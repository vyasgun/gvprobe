package ai

import (
	"context"
	"fmt"
	"os"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/vertex"
)

func AnalyseDhcpTrace(prompt string) {
	ctx := context.Background()

	region := os.Getenv("GCP_REGION")
	project := os.Getenv("GCP_PROJECT")
	if region == "" || project == "" {
		panic("GCP_REGION and GCP_PROJECT must be set")
	}
	client := anthropic.NewClient(
		vertex.WithGoogleAuth(ctx, region, project),
	)

	message, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeSonnet4_6,
		MaxTokens: 1024,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(message.Content[0].Text)
}
