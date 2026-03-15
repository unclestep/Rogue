// Server file
package app

import (
	"embed"
	"encoding/json"
	"log"
	"os"

	"github.com/google/uuid"
	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/dto"
	"github.com/unclestep/Rogue/internal/engine"
)

//go:embed assets/*
var assetsFS embed.FS

type Server struct {
	Engine *engine.GameEngine

	GameRules   *model.GameRules
	DungManager *model.DifficultyCurve
}

//
//
// --- CONSTRUCTORS ---
//
//

const (
	rules              string = "assets/game_rules/default_rules.json"
	genParamsStartPath string = "assets/dung_gen_params/dung_gen_params_start.json"
	genParamsEndPath   string = "assets/dung_gen_params/dung_gen_params_end.json"
	sessionPath        string = "saves/session.json"
)

func NewServer(fromClient <-chan dto.Command, toClient chan<- dto.WorldInfo) *Server {
	gr := createDefaultGameRules()
	dm := createDefaultDungGenManager()

	saveStruct(rules, gr)
	saveStruct(genParamsStartPath, dm.Start)
	saveStruct(genParamsEndPath, dm.End)

	session := loadSession(sessionPath)
	if session == nil {
		session = model.NewGameSession(gr, dm)
	} else {
		session.GameRules = gr
		session.DungeonGenManager = dm
		session.CurDungeonGenParams = dm.GetParamsForLevel(session)
	}

	s := &Server{
		GameRules:   createDefaultGameRules(),
		DungManager: createDefaultDungGenManager(),
	}
	s.Engine = engine.NewGameEngine(fromClient, toClient, session, s.Save, s.DeleteSave, 0)

	return s
}

//
//
// --- START SERVER METHOD ---
//
//

func (s *Server) Listen() {
	s.Engine.Run()
}

//
//
// --- SAVE&LOAD METHODS ---
//
//

func (s *Server) Save() {
	saveStruct(sessionPath, s.Engine.Session)
}

func (s *Server) DeleteSave() {
	os.Remove(sessionPath)
}

func loadSession(path string) *model.Playthrough {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	session := &model.Playthrough{}
	if err := json.Unmarshal(data, session); err != nil {
		return nil
	}

	return session
}

func saveStruct(path string, data any) {
	jsonData, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		log.Printf("Marshaling error: %v", err)
	}
	saveFile(path, jsonData)
}
