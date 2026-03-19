package mapper

import (
	"log"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/infrastructure/storage/json/dto"
)

// --- Change ---

func changeToDTO(c model.Change) dto.ChangeDTO {
	return dto.ChangeDTO{
		Holder: c.Holder.String(),
		Vital:  c.Vital.String(),
		Attr:   c.Attr.String(),
		Amount: c.Amount,
		Scale:  c.Scale,
	}
}

func changeFromDTO(d dto.ChangeDTO) model.Change {
	holder, err := model.TargetTypeString(d.Holder)
	if err != nil {
		log.Printf("[WARN] mapper: unknown target type %q\n", d.Holder)
	}
	vital, err := model.VitalTypeString(d.Vital)
	if err != nil {
		log.Printf("[WARN] mapper: unknown vital type %q\n", d.Vital)
	}
	attr, err := model.AttrTypeString(d.Attr)
	if err != nil {
		log.Printf("[WARN] mapper: unknown attr type %q\n", d.Attr)
	}
	return model.Change{
		Holder: holder,
		Vital:  vital,
		Attr:   attr,
		Amount: d.Amount,
		Scale:  d.Scale,
	}
}

// --- Reaction ---

func reactionToDTO(r *model.Reaction) *dto.ReactionDTO {
	if r == nil {
		return nil
	}

	d := &dto.ReactionDTO{
		Trigger:        r.Trigger.String(),
		Target:         r.Target.String(),
		StatusesChange: statusesToDTO(r.StatusesChange),
		Chance:         r.Chance,
	}

	if r.VitalsChange != nil {
		d.VitalsChange = make(map[string]dto.ChangeDTO, len(r.VitalsChange))
		for k, v := range r.VitalsChange {
			d.VitalsChange[k.String()] = changeToDTO(v)
		}
	}

	if r.BaseAttrsChange != nil {
		d.BaseAttrsChange = make(map[string]dto.ChangeDTO, len(r.BaseAttrsChange))
		for k, v := range r.BaseAttrsChange {
			d.BaseAttrsChange[k.String()] = changeToDTO(v)
		}
	}

	d.EffectsToApply = effectsMapToDTO(r.EffectsToApply)

	return d
}

func reactionFromDTO(d *dto.ReactionDTO) *model.Reaction {
	if d == nil {
		return nil
	}

	trigger, err := model.TriggerTypeString(d.Trigger)
	if err != nil {
		log.Printf("[WARN] mapper: unknown trigger type %q\n", d.Trigger)
	}
	target, err := model.TargetTypeString(d.Target)
	if err != nil {
		log.Printf("[WARN] mapper: unknown target type %q\n", d.Target)
	}

	r := &model.Reaction{
		Trigger:        trigger,
		Target:         target,
		StatusesChange: statusesFromDTO(d.StatusesChange),
		Chance:         d.Chance,
	}

	if d.VitalsChange != nil {
		r.VitalsChange = make(map[model.VitalType]model.Change, len(d.VitalsChange))
		for k, v := range d.VitalsChange {
			key, err := model.VitalTypeString(k)
			if err != nil {
				log.Printf("[WARN] mapper: unknown vital type %q, skipping\n", k)
				continue
			}
			r.VitalsChange[key] = changeFromDTO(v)
		}
	}

	if d.BaseAttrsChange != nil {
		r.BaseAttrsChange = make(map[model.AttrType]model.Change, len(d.BaseAttrsChange))
		for k, v := range d.BaseAttrsChange {
			key, err := model.AttrTypeString(k)
			if err != nil {
				log.Printf("[WARN] mapper: unknown attr type %q, skipping\n", k)
				continue
			}
			r.BaseAttrsChange[key] = changeFromDTO(v)
		}
	}

	r.EffectsToApply = effectsMapFromDTO(d.EffectsToApply)

	return r
}

// --- Effect ---

func EffectToDTO(e *model.Effect) *dto.EffectDTO {
	if e == nil {
		return nil
	}

	d := &dto.EffectDTO{
		Kind:           string(e.Kind),
		Duration:       e.Duration,
		Charges:        e.Charges,
		VitalsChange:   vitalsToDTO(e.VitalsChange),
		AttrsChange:    attrsToDTO(e.AttrsChange),
		StatusesChange: statusesToDTO(e.StatusesChange),
		ConsumeOn:      e.ConsumeOn.String(),
		Procs:          reactionsMapToDTO(e.Procs),
	}

	return d
}

func EffectFromDTO(d *dto.EffectDTO) *model.Effect {
	if d == nil {
		return nil
	}

	consumeOn, err := model.TriggerTypeString(d.ConsumeOn)
	if err != nil {
		log.Printf("[WARN] mapper: unknown trigger type %q\n", d.ConsumeOn)
	}

	return &model.Effect{
		Kind:           model.EffectType(d.Kind),
		Duration:       d.Duration,
		Charges:        d.Charges,
		VitalsChange:   vitalsFromDTO(d.VitalsChange),
		AttrsChange:    attrsFromDTO(d.AttrsChange),
		StatusesChange: statusesFromDTO(d.StatusesChange),
		ConsumeOn:      consumeOn,
		Procs:          reactionsMapFromDTO(d.Procs),
	}
}

// --- Helpers: effects map ---

func effectsMapToDTO(src map[model.EffectType]*model.Effect) map[string]*dto.EffectDTO {
	if src == nil {
		return nil
	}
	res := make(map[string]*dto.EffectDTO, len(src))
	for k, v := range src {
		res[string(k)] = EffectToDTO(v)
	}
	return res
}

func effectsMapFromDTO(src map[string]*dto.EffectDTO) map[model.EffectType]*model.Effect {
	if src == nil {
		return nil
	}
	res := make(map[model.EffectType]*model.Effect, len(src))
	for k, v := range src {
		res[model.EffectType(k)] = EffectFromDTO(v)
	}
	return res
}

// --- Helpers: reactions map ---

func reactionsMapToDTO(src map[model.TriggerType][]*model.Reaction) map[string][]*dto.ReactionDTO {
	if src == nil {
		return nil
	}
	res := make(map[string][]*dto.ReactionDTO, len(src))
	for k, reactions := range src {
		dtos := make([]*dto.ReactionDTO, len(reactions))
		for i, r := range reactions {
			dtos[i] = reactionToDTO(r)
		}
		res[k.String()] = dtos
	}
	return res
}

func reactionsMapFromDTO(src map[string][]*dto.ReactionDTO) map[model.TriggerType][]*model.Reaction {
	if src == nil {
		return nil
	}
	res := make(map[model.TriggerType][]*model.Reaction, len(src))
	for k, reactions := range src {
		key, err := model.TriggerTypeString(k)
		if err != nil {
			log.Printf("[WARN] mapper: unknown trigger type %q, skipping\n", k)
			continue
		}
		models := make([]*model.Reaction, len(reactions))
		for i, r := range reactions {
			models[i] = reactionFromDTO(r)
		}
		res[key] = models
	}
	return res
}
