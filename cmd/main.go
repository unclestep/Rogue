package main

import (
	"io"
	"log"
	"os"

	"github.com/unclestep/Rogue/internal/bootstrap"
)

// PursuerModelEnv names the env var that, when set, points at a trained
// ONNX policy for the Pursuer monster. Leaving it unset keeps the game on
// the scent-gradient fallback.
const PursuerModelEnv = "ROGUE_PURSUER_MODEL_PATH"

func init() {
	log.SetOutput(io.Discard)
}

func main() {
	bootstrap.InitializeApp(bootstrap.Config{
		PlaythroughsDir:  "saves/playthroughs",
		RulesDir:         "saves/rules",
		PursuerModelPath: os.Getenv(PursuerModelEnv),
	}).Run()
}
