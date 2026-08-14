package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/yanpgwang/mango-terminal/internal/demo"
	"github.com/yanpgwang/mango-terminal/internal/mango"
	"github.com/yanpgwang/mango-terminal/internal/ui"
)

var (
	version   = "dev"
	commit    = "none"
	buildDate = "unknown"
)

func main() {
	flags := flag.NewFlagSet("mango", flag.ExitOnError)
	baseURL := flags.String("url", envOr("MANGO_URL", "http://127.0.0.1:8080"), "Mango API base URL")
	apiKey := flags.String("api-key", os.Getenv("MANGO_API_KEY"), "Mango API key")
	demoMode := flags.Bool("demo", false, "Explore the interface without a Mango server")
	notify := flags.String("notify", envOr("MANGO_NOTIFY", "disabled"), "Background notifications: disabled, bell, or osc777")
	noMotion := flags.Bool("no-motion", envBool("MANGO_NO_MOTION"), "Disable decorative animation")
	showVersion := flags.Bool("version", false, "Print the Mango version")
	_ = flags.Parse(os.Args[1:])

	attachID := ""
	arguments := flags.Args()
	if *showVersion || (len(arguments) == 1 && arguments[0] == "version") {
		fmt.Printf("mango %s (%s, %s)\n", version, commit, buildDate)
		return
	}
	if len(arguments) > 0 {
		if arguments[0] != "attach" || len(arguments) != 2 {
			usage()
			os.Exit(2)
		}
		attachID = arguments[1]
	}

	var backend mango.Backend
	endpointLabel := *baseURL
	if *demoMode {
		backend = demo.New()
		endpointLabel = "Local demo · no server required"
	} else {
		client, err := mango.New(mango.Config{BaseURL: *baseURL, APIKey: *apiKey})
		if err != nil {
			fmt.Fprintln(os.Stderr, "mango:", err)
			os.Exit(2)
		}
		backend = client
	}

	notificationMode := ui.NotificationMode(*notify)
	switch notificationMode {
	case ui.NotificationDisabled, ui.NotificationBell, ui.NotificationOSC777:
	default:
		fmt.Fprintln(os.Stderr, "mango: --notify must be disabled, bell, or osc777")
		os.Exit(2)
	}
	program := tea.NewProgram(ui.NewWithOptions(backend, attachID, ui.Options{
		Notifications: notificationMode,
		ReducedMotion: *noMotion,
		Endpoint:      endpointLabel,
	}))
	if _, err := program.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "mango:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: mango [--demo] [--url URL] [--api-key KEY] [--notify MODE] [--no-motion] [attach SESSION_ID]")
}

func envBool(name string) bool {
	switch os.Getenv(name) {
	case "1", "true", "TRUE", "yes", "YES", "on", "ON":
		return true
	default:
		return false
	}
}
func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
