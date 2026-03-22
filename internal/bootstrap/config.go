package bootstrap

// Config holds filesystem paths needed to initialize the application.
// Each binary (standalone, server, client) passes its own Config to its injector.
type Config struct {
	PlaythroughsDir string
	RulesDir        string
}
