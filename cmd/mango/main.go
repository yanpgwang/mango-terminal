package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/yanpgwang/mango-terminal/internal/api"
	"github.com/yanpgwang/mango-terminal/internal/ui"
)

func main() {
	flags := flag.NewFlagSet("mango", flag.ExitOnError)
	baseURL := flags.String("url", envOr("MANGO_URL", "http://127.0.0.1:8080"), "Mango API base URL")
	apiKey := flags.String("api-key", os.Getenv("MANGO_API_KEY"), "Mango API key")
	_ = flags.Parse(os.Args[1:])

	attachID := ""
	arguments := flags.Args()
	if len(arguments) > 0 {
		if arguments[0] != "attach" || len(arguments) != 2 {
			fmt.Fprintln(os.Stderr, "usage: mango [flags] [attach SESSION_ID]")
			os.Exit(2)
		}
		attachID = arguments[1]
	}

	client, err := api.New(api.Config{BaseURL: *baseURL, APIKey: *apiKey})
	if err != nil {
		fmt.Fprintln(os.Stderr, "mango:", err)
		os.Exit(2)
	}
	program := tea.NewProgram(ui.New(client, attachID))
	if _, err := program.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "mango:", err)
		os.Exit(1)
	}
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
