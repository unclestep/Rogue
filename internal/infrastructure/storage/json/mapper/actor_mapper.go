package mapper

import (
	"log"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/infrastructure/storage/json/dto"
)

func ActorToDTO(a *model.Actor) *dto.ActorDTO {
	if a == nil {
		return nil
	}

	d := &dto.ActorDTO{
		Id:           int64(a.Id),
		Kind:         string(a.Kind),
		Label:        string(a.Label),
		MovePattern:  a.MovePattern.String(),
		MoveDir:      pointToDTO(a.MoveDir),
		Pos:          pointToDTO(a.Pos),
		Vitals:       vitalsToDTO(a.Vitals),
		BaseAttrs:    attrsToDTO(a.BaseAttrs),
		DerivedAttrs: attrsToDTO(a.DerivedAttrs),
		Statuses:     statusesToDTO(a.Statuses),
		State:        a.State.String(),
		Backpack:     BackpackToDTO(a.Backpack),
	}

	d.Effects = effectsMapToDTO(a.Effects)

	if a.EquippedGear != nil {
		d.EquippedGear = make(map[string]*dto.ItemDTO, len(a.EquippedGear))
		for kind, item := range a.EquippedGear {
			d.EquippedGear[string(kind)] = ItemToDTO(item)
		}
	}

	if a.Traits != nil {
		d.Traits = make(map[string][]*dto.ReactionDTO, len(a.Traits))
		for trigger, reactions := range a.Traits {
			dtos := make([]*dto.ReactionDTO, len(reactions))
			for i, r := range reactions {
				dtos[i] = reactionToDTO(r)
			}
			d.Traits[trigger.String()] = dtos
		}
	}

	return d
}

func ActorFromDTO(d *dto.ActorDTO) *model.Actor {
	if d == nil {
		return nil
	}

	movePattern, err := model.MovePatternTypeString(d.MovePattern)
	if err != nil {
		log.Printf("[WARN] mapper: unknown move pattern %q\n", d.MovePattern)
	}
	state, err := model.BehaviorTypeString(d.State)
	if err != nil {
		log.Printf("[WARN] mapper: unknown behavior type %q\n", d.State)
	}

	a := &model.Actor{
		Id:           model.ActorId(d.Id),
		Kind:         model.ActorType(d.Kind),
		Label:        model.ActorLabel(d.Label),
		MovePattern:  movePattern,
		MoveDir:      pointFromDTO(d.MoveDir),
		Pos:          pointFromDTO(d.Pos),
		Vitals:       vitalsFromDTO(d.Vitals),
		BaseAttrs:    attrsFromDTO(d.BaseAttrs),
		DerivedAttrs: attrsFromDTO(d.DerivedAttrs),
		Statuses:     statusesFromDTO(d.Statuses),
		State:        state,
		Effects:      effectsMapFromDTO(d.Effects),
		Backpack:     BackpackFromDTO(d.Backpack),
	}

	if d.EquippedGear != nil {
		a.EquippedGear = make(map[model.ItemType]*model.Item, len(d.EquippedGear))
		for kind, item := range d.EquippedGear {
			a.EquippedGear[model.ItemType(kind)] = ItemFromDTO(item)
		}
	} else {
		a.EquippedGear = make(map[model.ItemType]*model.Item)
	}

	if d.Traits != nil {
		a.Traits = make(map[model.TriggerType][]*model.Reaction, len(d.Traits))
		for k, reactions := range d.Traits {
			trigger, err := model.TriggerTypeString(k)
			if err != nil {
				log.Printf("[WARN] mapper: unknown trigger type %q, skipping\n", k)
				continue
			}
			models := make([]*model.Reaction, len(reactions))
			for i, r := range reactions {
				models[i] = reactionFromDTO(r)
			}
			a.Traits[trigger] = models
		}
	} else {
		a.Traits = make(map[model.TriggerType][]*model.Reaction)
	}

	return a
}

// --- Actor map helpers ---

func actorsMapToDTO(src map[model.ActorId]*model.Actor) map[int64]*dto.ActorDTO {
	if src == nil {
		return nil
	}
	res := make(map[int64]*dto.ActorDTO, len(src))
	for k, v := range src {
		res[int64(k)] = ActorToDTO(v)
	}
	return res
}

func actorsMapFromDTO(src map[int64]*dto.ActorDTO) map[model.ActorId]*model.Actor {
	if src == nil {
		return make(map[model.ActorId]*model.Actor)
	}
	res := make(map[model.ActorId]*model.Actor, len(src))
	for k, v := range src {
		res[model.ActorId(k)] = ActorFromDTO(v)
	}
	return res
}
