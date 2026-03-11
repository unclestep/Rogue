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

func loadSession(path string) *model.GameSession {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	session := &model.GameSession{}
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

func createDefaultDungGenManager() *model.DifficultyCurve {
	return &model.DifficultyCurve{
		Start: createDefaultStartDungGenParams(),
		End:   createDefaultEndDungGenParams(),
	}
}

func createDefaultStartDungGenParams() *model.DifficultyCurve {
	return &model.DifficultyCurve{
		MaxMonsters: 3,
		MinMonsters: 1,
		MonsterWeights: map[model.ActorLabel]int{
			model.ActorLabelZombieCommon:    40,
			model.ActorLabelVampireCommon:   15,
			model.ActorLabelGhostCommon:     10,
			model.ActorLabelOgreCommon:      15,
			model.ActorLabelSnakeMageCommon: 15,
			model.ActorLabelMimicCommon:     5,
		},
		MonsterStatsMultiplier: 1.0,
		MaxItems:               7,
		MinItems:               3,
		ItemWeights: map[model.ItemLabel]int{
			model.ItemLabelDefaultFood:     40,
			model.ItemLabelDexterityElixir: 10,
			model.ItemLabelStrengthElixir:  10,
			model.ItemLabelMaxHpElixir:     10,
			model.ItemLabelDexterityScroll: 5,
			model.ItemLabelStrengthScroll:  5,
			model.ItemLabelMaxHpScroll:     5,
			model.ItemLabelDefaultWeapon:   15,
		},
		TreasureValueMultiplier: 1.0,
		LockedDoorsStartDepth:   5,
		MaxLockedDoors:          2,
		MinLockedDoors:          1,
	}
}

func createDefaultEndDungGenParams() *model.DifficultyCurve {
	return &model.DifficultyCurve{
		MaxMonsters: 15,
		MinMonsters: 12,
		MonsterWeights: map[model.ActorLabel]int{
			model.ActorLabelZombieCommon:    10,
			model.ActorLabelVampireCommon:   25,
			model.ActorLabelGhostCommon:     15,
			model.ActorLabelOgreCommon:      25,
			model.ActorLabelSnakeMageCommon: 20,
			model.ActorLabelMimicCommon:     5,
		},
		MonsterStatsMultiplier: 2.0,
		MaxItems:               3,
		MinItems:               0,
		ItemWeights: map[model.ItemLabel]int{
			model.ItemLabelDefaultFood:     10,
			model.ItemLabelDexterityElixir: 20,
			model.ItemLabelStrengthElixir:  20,
			model.ItemLabelMaxHpElixir:     20,
			model.ItemLabelDexterityScroll: 5,
			model.ItemLabelStrengthScroll:  5,
			model.ItemLabelMaxHpScroll:     5,
			model.ItemLabelDefaultWeapon:   15,
		},
		TreasureValueMultiplier: 2.0,
		LockedDoorsStartDepth:   5,
		MaxLockedDoors:          9,
		MinLockedDoors:          7,
	}
}

func createDefaultGameRules() *model.GameRules {
	return &model.GameRules{
		MaxDungeonCount:        21,
		MaxHorizontalRoomCount: 3,
		MaxVerticalRoomCount:   3,
		ActorsConf:             createDefaultActorsConf(),
		ItemsConf:              createDefaultItemsConf(),
		GameDifficulty:         1.0,
		HpRestore:              0.25,
		TimeForMove:            0,
	}
}

func createDefaultActorsConf() map[model.ActorLabel]*model.Actor {
	return map[model.ActorLabel]*model.Actor{
		model.ActorLabelPlayerCommon:    model.NewDefaultPlayer(0, model.NewInvalidPoint(), 9),
		model.ActorLabelZombieCommon:    model.NewDefaultZombie(0, model.NewInvalidPoint()),
		model.ActorLabelVampireCommon:   model.NewDefaultVampire(0, model.NewInvalidPoint()),
		model.ActorLabelGhostCommon:     model.NewDefaultGhost(0, model.NewInvalidPoint()),
		model.ActorLabelOgreCommon:      model.NewDefaultOgre(0, model.NewInvalidPoint()),
		model.ActorLabelSnakeMageCommon: model.NewDefaultSnakeMage(0, model.NewInvalidPoint()),
		model.ActorLabelMimicCommon:     model.NewDefaultMimic(0, model.NewInvalidPoint()),
	}
}

func createDefaultItemsConf() map[model.ItemLabel]*model.Item {
	return map[model.ItemLabel]*model.Item{
		model.ItemLabelDefaultFood:     model.NewDefaultFood(0, model.NewInvalidPoint()),
		model.ItemLabelDexterityElixir: model.NewDefaultDexterityElixir(0, model.NewInvalidPoint()),
		model.ItemLabelStrengthElixir:  model.NewDefaultStrengthElixir(0, model.NewInvalidPoint()),
		model.ItemLabelMaxHpElixir:     model.NewDefaultMaxHpElixir(0, model.NewInvalidPoint()),
		model.ItemLabelDexterityScroll: model.NewDefaultDexterityScroll(0, model.NewInvalidPoint()),
		model.ItemLabelStrengthScroll:  model.NewDefaultStrengthScroll(0, model.NewInvalidPoint()),
		model.ItemLabelMaxHpScroll:     model.NewDefaultMaxHpScroll(0, model.NewInvalidPoint()),
		model.ItemLabelDefaultWeapon:   model.NewDefaultWeapon(0, model.NewInvalidPoint()),
	}
}
