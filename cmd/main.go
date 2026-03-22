package main

import "github.com/unclestep/Rogue/internal/bootstrap"

func main() {
	bootstrap.InitializeApp(bootstrap.Config{
		PlaythroughsDir: "saves/playthroughs",
		RulesDir:        "saves/rules",
	}).Run()
}
