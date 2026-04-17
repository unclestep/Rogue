package model_test

import (
	"testing"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/pkg/geometry"
)

func TestNewDefaultPursuerMatchesAttrConf(t *testing.T) {
	pos := geometry.Point{X: 3, Y: 4}
	p := model.NewDefaultPursuer(42, pos)

	if p.Id != 42 {
		t.Errorf("Expected Id=42, got %d", p.Id)
	}
	if p.Kind != model.ActorPursuer {
		t.Errorf("Expected Kind=ActorPursuer, got %q", p.Kind)
	}
	if p.Pos != pos {
		t.Errorf("Expected Pos=%v, got %v", pos, p.Pos)
	}
	if p.MovePattern != model.MovePatternDefault {
		t.Errorf("Expected MovePatternDefault, got %v", p.MovePattern)
	}
	if p.State != model.BehaviorPursuer {
		t.Errorf("Expected State=BehaviorPursuer, got %v", p.State)
	}

	if got := p.Vitals[model.VitalHP]; got != model.PursuerDefault.MaxHealth {
		t.Errorf("Expected VitalHP=%d, got %d", model.PursuerDefault.MaxHealth, got)
	}
	if got := p.Vitals[model.VitalStamina]; got != model.PursuerDefault.MaxStamina {
		t.Errorf("Expected VitalStamina=%d, got %d", model.PursuerDefault.MaxStamina, got)
	}

	if got := p.BaseAttrs[model.AttrDexterity]; got != model.PursuerDefault.Dexterity {
		t.Errorf("Expected Dexterity=%d, got %d", model.PursuerDefault.Dexterity, got)
	}
	if got := p.BaseAttrs[model.AttrHostility]; got != model.PursuerDefault.Hostility {
		t.Errorf("Expected Hostility=%d, got %d", model.PursuerDefault.Hostility, got)
	}
	if got := p.DerivedAttrs[model.AttrStrength]; got != model.PursuerDefault.Strength {
		t.Errorf("Expected DerivedAttr Strength=%d, got %d", model.PursuerDefault.Strength, got)
	}

	if p.Effects == nil || p.Statuses == nil || p.Traits == nil {
		t.Errorf("Expected Effects/Statuses/Traits maps to be initialized")
	}
}

func TestNewCustomPursuerScalesStats(t *testing.T) {
	pos := geometry.NewDefaultPoint()
	base := model.NewDefaultPursuer(1, pos)
	scaled := model.NewCustomPursuer(2, pos, 2.0)

	if scaled.Vitals[model.VitalHP] != base.Vitals[model.VitalHP]*2 {
		t.Errorf("Expected HP to double with difficulty=2.0, got %d (base %d)",
			scaled.Vitals[model.VitalHP], base.Vitals[model.VitalHP])
	}
	if scaled.BaseAttrs[model.AttrStrength] != base.BaseAttrs[model.AttrStrength]*2 {
		t.Errorf("Expected Strength to double with difficulty=2.0, got %d (base %d)",
			scaled.BaseAttrs[model.AttrStrength], base.BaseAttrs[model.AttrStrength])
	}
	// Stamina-related attrs must NOT scale (see AdjustStatsByDifficulty).
	if scaled.BaseAttrs[model.AttrMaxStamina] != base.BaseAttrs[model.AttrMaxStamina] {
		t.Errorf("Expected MaxStamina to stay unchanged under difficulty scaling")
	}
}

func TestPursuerRegisteredInDefaultRules(t *testing.T) {
	rules := model.NewDefaultGameRules()

	template, ok := rules.ActorsConf[model.ActorLabelPursuerCommon]
	if !ok {
		t.Fatalf("Expected ActorLabelPursuerCommon in default ActorsConf")
	}
	if template.Kind != model.ActorPursuer {
		t.Errorf("Expected template.Kind=ActorPursuer, got %q", template.Kind)
	}

	startWeights := rules.DiffCurve.Start.MonsterWeights
	if w, ok := startWeights[model.ActorLabelPursuerCommon]; !ok || w <= 0 {
		t.Errorf("Expected positive start-weight for Pursuer, got %d (ok=%v)", w, ok)
	}
	endWeights := rules.DiffCurve.End.MonsterWeights
	if w, ok := endWeights[model.ActorLabelPursuerCommon]; !ok || w <= 0 {
		t.Errorf("Expected positive end-weight for Pursuer, got %d (ok=%v)", w, ok)
	}
}
