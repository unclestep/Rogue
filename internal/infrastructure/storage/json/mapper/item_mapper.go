package mapper

import (
	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/infrastructure/storage/json/dto"
)

// --- Item ---

func ItemToDTO(i *model.Item) *dto.ItemDTO {
	if i == nil {
		return nil
	}

	return &dto.ItemDTO{
		Id:              int64(i.Id),
		Kind:            string(i.Kind),
		Label:           string(i.Label),
		Pos:             pointToDTO(i.Pos),
		Keyhole:         int(i.Keyhole),
		Value:           i.Value,
		VitalsChange:    vitalsToDTO(i.VitalsChange),
		BaseAttrsChange: attrsToDTO(i.BaseAttrsChange),
		Effects:         effectsMapToDTO(i.Effects),
		Procs:           reactionsMapToDTO(i.Procs),
	}
}

func ItemFromDTO(d *dto.ItemDTO) *model.Item {
	if d == nil {
		return nil
	}

	return &model.Item{
		Id:              model.ItemId(d.Id),
		Kind:            model.ItemType(d.Kind),
		Label:           model.ItemLabel(d.Label),
		Pos:             pointFromDTO(d.Pos),
		Keyhole:         model.Keyhole(d.Keyhole),
		Value:           d.Value,
		VitalsChange:    vitalsFromDTO(d.VitalsChange),
		BaseAttrsChange: attrsFromDTO(d.BaseAttrsChange),
		Effects:         effectsMapFromDTO(d.Effects),
		Procs:           reactionsMapFromDTO(d.Procs),
	}
}

// --- Backpack ---

func BackpackToDTO(b *model.Backpack) *dto.BackpackDTO {
	if b == nil {
		return nil
	}

	d := &dto.BackpackDTO{
		TreasuresValue: b.TreasuresValue,
		SlotsCapacity:  b.SlotsCapacity,
	}

	if b.Slots != nil {
		d.Slots = make(map[string][]*dto.ItemDTO, len(b.Slots))
		for kind, items := range b.Slots {
			dtos := make([]*dto.ItemDTO, len(items))
			for i, item := range items {
				dtos[i] = ItemToDTO(item)
			}
			d.Slots[string(kind)] = dtos
		}
	}

	return d
}

func BackpackFromDTO(d *dto.BackpackDTO) *model.Backpack {
	if d == nil {
		return nil
	}

	b := &model.Backpack{
		TreasuresValue: d.TreasuresValue,
		SlotsCapacity:  d.SlotsCapacity,
	}

	if d.Slots != nil {
		b.Slots = make(map[model.ItemType][]*model.Item, len(d.Slots))
		for kind, items := range d.Slots {
			models := make([]*model.Item, len(items))
			for i, item := range items {
				models[i] = ItemFromDTO(item)
			}
			b.Slots[model.ItemType(kind)] = models
		}
	} else {
		b.Slots = make(map[model.ItemType][]*model.Item)
	}

	return b
}

// --- Items map helpers ---

func itemsMapToDTO(src map[model.ItemId]*model.Item) map[int64]*dto.ItemDTO {
	if src == nil {
		return nil
	}
	res := make(map[int64]*dto.ItemDTO, len(src))
	for k, v := range src {
		res[int64(k)] = ItemToDTO(v)
	}
	return res
}

func itemsMapFromDTO(src map[int64]*dto.ItemDTO) map[model.ItemId]*model.Item {
	if src == nil {
		return make(map[model.ItemId]*model.Item)
	}
	res := make(map[model.ItemId]*model.Item, len(src))
	for k, v := range src {
		res[model.ItemId(k)] = ItemFromDTO(v)
	}
	return res
}
