package mapper

import (
	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/infrastructure/storage/json/dto"
)

func GameRulesToDTO(r *model.GameRules) *dto.GameRulesDTO {
	if r == nil {
		return nil
	}

	d := &dto.GameRulesDTO{
		Id:                     int64(r.Id),
		MaxDungeonCount:        r.MaxDungeonCount,
		DungeonWidth:           r.DungeonWidth,
		DungeonHeight:          r.DungeonHeight,
		MaxHorizontalRoomCount: r.MaxHorizontalRoomCount,
		MaxVerticalRoomCount:   r.MaxVerticalRoomCount,
		HpRestore:              r.HpRestore,
		TimeForMove:            r.TimeForMove,
		DiffCurve:              difficultyCurveToDTO(r.DiffCurve),
	}

	if r.ActorsConf != nil {
		d.ActorsConf = make(map[string]*dto.ActorDTO, len(r.ActorsConf))
		for label, actor := range r.ActorsConf {
			d.ActorsConf[string(label)] = ActorToDTO(actor)
		}
	}

	if r.ItemsConf != nil {
		d.ItemsConf = make(map[string]*dto.ItemDTO, len(r.ItemsConf))
		for label, item := range r.ItemsConf {
			d.ItemsConf[string(label)] = ItemToDTO(item)
		}
	}

	return d
}

func GameRulesFromDTO(d *dto.GameRulesDTO) *model.GameRules {
	if d == nil {
		return nil
	}

	r := &model.GameRules{
		Id:                     model.RulesId(d.Id),
		MaxDungeonCount:        d.MaxDungeonCount,
		DungeonWidth:           d.DungeonWidth,
		DungeonHeight:          d.DungeonHeight,
		MaxHorizontalRoomCount: d.MaxHorizontalRoomCount,
		MaxVerticalRoomCount:   d.MaxVerticalRoomCount,
		HpRestore:              d.HpRestore,
		TimeForMove:            d.TimeForMove,
		DiffCurve:              difficultyCurveFromDTO(d.DiffCurve),
	}

	if d.ActorsConf != nil {
		r.ActorsConf = make(map[model.ActorLabel]*model.Actor, len(d.ActorsConf))
		for label, actor := range d.ActorsConf {
			r.ActorsConf[model.ActorLabel(label)] = ActorFromDTO(actor)
		}
	}

	if d.ItemsConf != nil {
		r.ItemsConf = make(map[model.ItemLabel]*model.Item, len(d.ItemsConf))
		for label, item := range d.ItemsConf {
			r.ItemsConf[model.ItemLabel(label)] = ItemFromDTO(item)
		}
	}

	return r
}

func difficultyCurveToDTO(c *model.DifficultyCurve) *dto.DifficultyCurveDTO {
	if c == nil {
		return nil
	}
	return &dto.DifficultyCurveDTO{
		Start: dungParamsToDTO(c.Start),
		End:   dungParamsToDTO(c.End),
	}
}

func difficultyCurveFromDTO(d *dto.DifficultyCurveDTO) *model.DifficultyCurve {
	if d == nil {
		return nil
	}
	return &model.DifficultyCurve{
		Start: dungParamsFromDTO(d.Start),
		End:   dungParamsFromDTO(d.End),
	}
}
