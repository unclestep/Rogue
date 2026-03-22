package main

import (
	"github.com/unclestep/Rogue/internal/bootstrap"
	"io"
	"log"
)

func init() {
	log.SetOutput(io.Discard)
}

func main() {
	bootstrap.InitializeApp(bootstrap.Config{
		PlaythroughsDir: "saves/playthroughs",
		RulesDir:        "saves/rules",
	}).Run()
}
