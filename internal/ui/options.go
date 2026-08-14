package ui

// NotificationMode controls out-of-focus terminal notifications. Escape
// sequence notifications are opt-in because unsupported terminals may print
// them literally.
type NotificationMode string

const (
	NotificationDisabled NotificationMode = "disabled"
	NotificationBell     NotificationMode = "bell"
	NotificationOSC777   NotificationMode = "osc777"
)

// Options contains terminal-specific behavior that should be explicit rather
// than guessed from environment variables inside the UI model.
type Options struct {
	Notifications NotificationMode
	ReducedMotion bool
	// Endpoint is display-only connection context. Authentication still belongs
	// to the CLI/config layer until Mango exposes a real identity flow.
	Endpoint string
}
