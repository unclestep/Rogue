package service

import (
	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/pkg/geometry"
)

// Action indices returned by Policy.Predict.
// Order matches geometry.GetCardinalDirs(): Up, Right, Down, Left.
const (
	ActionUp    = 0
	ActionRight = 1
	ActionDown  = 2
	ActionLeft  = 3
	ActionWait  = 4

	ActionCount = 5
)

// Policy chooses the Pursuer's next action. Implementations fall into three
// categories and each uses a different subset of the inputs:
//
//   - ONNXPolicy (PR 3)   — consumes `obs` only; ignores ctx/actor.
//   - FallbackPolicy      — consumes ctx/actor only; ignores `obs`.
//     Used when the trained model is not available (no file, load error, etc.).
//   - ScriptedPolicy      — returns a fixed sequence; ignores everything. Test double.
//
// Keeping all three inputs on the interface lets the Pursuer-side call site
// stay the same regardless of which implementation is wired in.
type Policy interface {
	Predict(ctx *model.SessionContext, actor *model.Actor, obs []float32) int
}

// FallbackPolicy routes the decision through the same Dijkstra-chase gradient
// used by ChaseBehavior. It exists so the game keeps running when no ONNX
// model is present.
type FallbackPolicy struct {
	chase *ChaseBehavior
}

func NewFallbackPolicy(pathfinder *Pathfinder) *FallbackPolicy {
	return &FallbackPolicy{chase: NewChaseBehavior(pathfinder)}
}

func (fp *FallbackPolicy) Predict(ctx *model.SessionContext, actor *model.Actor, _ []float32) int {
	_, decision := fp.chase.Update(ctx, actor)
	if decision == nil {
		return ActionWait
	}
	return VectorToAction(decision.MoveVector)
}

// ScriptedPolicy replays a fixed sequence of actions. When the sequence is
// exhausted, it returns ActionWait. Used by unit tests to make Pursuer
// behaviour deterministic without loading a model.
type ScriptedPolicy struct {
	actions []int
	idx     int
}

func NewScriptedPolicy(actions []int) *ScriptedPolicy {
	return &ScriptedPolicy{actions: actions}
}

func (sp *ScriptedPolicy) Predict(_ *model.SessionContext, _ *model.Actor, _ []float32) int {
	if sp.idx >= len(sp.actions) {
		return ActionWait
	}
	a := sp.actions[sp.idx]
	sp.idx++
	return a
}

// ActionToVector maps an action index to a move vector (dx, dy).
// Unknown actions resolve to (0, 0) — treated as Wait.
func ActionToVector(action int) geometry.Point {
	switch action {
	case ActionUp:
		return geometry.GetDirUp()
	case ActionRight:
		return geometry.GetDirRight()
	case ActionDown:
		return geometry.GetDirDown()
	case ActionLeft:
		return geometry.GetDirLeft()
	default:
		return geometry.Point{}
	}
}

// VectorToAction is the inverse of ActionToVector for cardinal-only vectors.
// Diagonal or multi-step vectors are collapsed to their dominant axis so
// FallbackPolicy (which pulls from an 8-way pathfinder) always yields a valid
// 5-way action.
func VectorToAction(v geometry.Point) int {
	if v.X == 0 && v.Y == 0 {
		return ActionWait
	}
	if abs(v.X) >= abs(v.Y) {
		if v.X > 0 {
			return ActionRight
		}
		return ActionLeft
	}
	if v.Y > 0 {
		return ActionDown
	}
	return ActionUp
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
