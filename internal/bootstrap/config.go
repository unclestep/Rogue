package bootstrap

// Config holds filesystem paths needed to initialize the application.
// Each binary (standalone, server, client) passes its own Config to its injector.
type Config struct {
	PlaythroughsDir string
	RulesDir        string

	// PursuerModelPath points at a trained ONNX policy for the Pursuer
	// monster. Empty string disables ONNX and uses FallbackPolicy (the
	// scent-gradient chase). See providePursuerPolicy in providers.go.
	PursuerModelPath string
}
