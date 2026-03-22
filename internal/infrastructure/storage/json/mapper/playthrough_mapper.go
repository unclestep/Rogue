package mapper

import (
	"log"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/infrastructure/storage/json/dto"
)

// --- GameStats ---

func gameStatsToDTO(s *model.GameStats) *dto.GameStatsDTO {
	if s == nil {
		return nil
	}
	return &dto.GameStatsDTO{
		TotalTreasure:    s.TotalTreasure,
		DeepestLevel:     s.DeepestLevel,
		MonstersDefeated: s.MonstersDefeated,
		FoodConsumed:     s.FoodConsumed,
		ElixirsDrunk:     s.ElixirsDrunk,
		ScrollsRead:      s.ScrollsRead,
		HitsDealt:        s.HitsDealt,
		HitsReceived:     s.HitsReceived,
		TilesTraveled:    s.TilesTraveled,
	}
}

func gameStatsFromDTO(d *dto.GameStatsDTO) *model.GameStats {
	if d == nil {
		return nil
	}
	return &model.GameStats{
		TotalTreasure:    d.TotalTreasure,
		DeepestLevel:     d.DeepestLevel,
		MonstersDefeated: d.MonstersDefeated,
		FoodConsumed:     d.FoodConsumed,
		ElixirsDrunk:     d.ElixirsDrunk,
		ScrollsRead:      d.ScrollsRead,
		HitsDealt:        d.HitsDealt,
		HitsReceived:     d.HitsReceived,
		TilesTraveled:    d.TilesTraveled,
	}
}

// --- LevelMetrics ---

func levelMetricsToDTO(m *model.LevelMetrics) *dto.LevelMetricsDTO {
	if m == nil {
		return nil
	}
	return &dto.LevelMetricsDTO{
		DamageTaken: m.DamageTaken,
		DamageDealt: m.DamageDealt,
	}
}

func levelMetricsFromDTO(d *dto.LevelMetricsDTO) *model.LevelMetrics {
	if d == nil {
		return nil
	}
	return &model.LevelMetrics{
		DamageTaken: d.DamageTaken,
		DamageDealt: d.DamageDealt,
	}
}

// --- VisibleArea ---

func visibleAreaToDTO(v *model.VisibleArea) *dto.VisibleAreaDTO {
	if v == nil {
		return nil
	}

	d := &dto.VisibleAreaDTO{
		VisibleCells: pointsToDTO(v.VisibleCells),
	}

	if v.Area != nil {
		d.Area = make([][]string, len(v.Area))
		for y, row := range v.Area {
			d.Area[y] = make([]string, len(row))
			for x, state := range row {
				d.Area[y][x] = state.String()
			}
		}
	}

	return d
}

func visibleAreaFromDTO(d *dto.VisibleAreaDTO) *model.VisibleArea {
	if d == nil {
		return nil
	}

	v := &model.VisibleArea{
		VisibleCells: pointsFromDTO(d.VisibleCells),
	}

	if d.Area != nil {
		v.Area = make([][]model.VisibilityState, len(d.Area))
		for y, row := range d.Area {
			v.Area[y] = make([]model.VisibilityState, len(row))
			for x, s := range row {
				state, err := model.VisibilityStateString(s)
				if err != nil {
					log.Printf("[WARN] mapper: unknown visibility state %q at (%d,%d)\n", s, x, y)
				}
				v.Area[y][x] = state
			}
		}
	}

	return v
}

// --- DungParams ---

func dungParamsToDTO(p *model.DungParams) *dto.DungParamsDTO {
	if p == nil {
		return nil
	}

	d := &dto.DungParamsDTO{
		MaxMonsters:             p.MaxMonsters,
		MinMonsters:             p.MinMonsters,
		MonsterStatsMultiplier:  p.MonsterStatsMultiplier,
		MaxItems:                p.MaxItems,
		MinItems:                p.MinItems,
		TreasureValueMultiplier: p.TreasureValueMultiplier,
		LockedDoorsStartDepth:   p.LockedDoorsStartDepth,
		MaxLockedDoors:          p.MaxLockedDoors,
		MinLockedDoors:          p.MinLockedDoors,
	}

	if p.MonsterWeights != nil {
		d.MonsterWeights = make(map[string]int, len(p.MonsterWeights))
		for k, v := range p.MonsterWeights {
			d.MonsterWeights[string(k)] = v
		}
	}

	if p.ItemWeights != nil {
		d.ItemWeights = make(map[string]int, len(p.ItemWeights))
		for k, v := range p.ItemWeights {
			d.ItemWeights[string(k)] = v
		}
	}

	return d
}

func dungParamsFromDTO(d *dto.DungParamsDTO) *model.DungParams {
	if d == nil {
		return nil
	}

	p := &model.DungParams{
		MaxMonsters:             d.MaxMonsters,
		MinMonsters:             d.MinMonsters,
		MonsterStatsMultiplier:  d.MonsterStatsMultiplier,
		MaxItems:                d.MaxItems,
		MinItems:                d.MinItems,
		TreasureValueMultiplier: d.TreasureValueMultiplier,
		LockedDoorsStartDepth:   d.LockedDoorsStartDepth,
		MaxLockedDoors:          d.MaxLockedDoors,
		MinLockedDoors:          d.MinLockedDoors,
	}

	if d.MonsterWeights != nil {
		p.MonsterWeights = make(map[model.ActorLabel]int, len(d.MonsterWeights))
		for k, v := range d.MonsterWeights {
			p.MonsterWeights[model.ActorLabel(k)] = v
		}
	}

	if d.ItemWeights != nil {
		p.ItemWeights = make(map[model.ItemLabel]int, len(d.ItemWeights))
		for k, v := range d.ItemWeights {
			p.ItemWeights[model.ItemLabel(k)] = v
		}
	}

	return p
}

// --- Playthrough ---

func PlaythroughToDTO(p *model.Playthrough) *dto.Playthrough {
	if p == nil {
		return nil
	}

	d := &dto.Playthrough{
		PlaythroughId:     string(p.PlaythroughId),
		HostId:            int64(p.HostId),
		RulesId:           int64(p.RulesId),
		Map:               MapToDTO(p.Map),
		Depth:             p.Depth,
		State:             p.State.String(),
		DungParams:        dungParamsToDTO(p.DungParams),
		DynamicDifficulty: p.DynamicDifficulty,
		NextId:            p.NextId,
		Seed:              p.Seed,
	}

	// PlayersUuid: map[string]ActorId -> map[string]int64
	if p.PlayersUuid != nil {
		d.PlayersUuid = make(map[string]int64, len(p.PlayersUuid))
		for uuid, id := range p.PlayersUuid {
			d.PlayersUuid[uuid] = int64(id)
		}
	}

	d.Players = actorsMapToDTO(p.Players)
	d.Monsters = actorsMapToDTO(p.Monsters)
	d.DeadMonsters = actorsMapToDTO(p.DeadMonsters)
	d.Items = itemsMapToDTO(p.Items)

	// PlayersStats
	if p.PlayersStats != nil {
		d.PlayersStats = make(map[int64]*dto.GameStatsDTO, len(p.PlayersStats))
		for k, v := range p.PlayersStats {
			d.PlayersStats[int64(k)] = gameStatsToDTO(v)
		}
	}

	// PlayersLevelMetrics
	if p.PlayersLevelMetrics != nil {
		d.PlayersLevelMetrics = make(map[int64]*dto.LevelMetricsDTO, len(p.PlayersLevelMetrics))
		for k, v := range p.PlayersLevelMetrics {
			d.PlayersLevelMetrics[int64(k)] = levelMetricsToDTO(v)
		}
	}

	// PlayersFoW
	if p.PlayersFoW != nil {
		d.PlayersFoW = make(map[int64]*dto.VisibleAreaDTO, len(p.PlayersFoW))
		for k, v := range p.PlayersFoW {
			d.PlayersFoW[int64(k)] = visibleAreaToDTO(v)
		}
	}

	// TurnEvents and TurnDeadline are intentionally not serialized.

	return d
}

func PlaythroughFromDTO(d *dto.Playthrough) *model.Playthrough {
	if d == nil {
		return nil
	}

	state, err := model.GameStateString(d.State)
	if err != nil {
		log.Printf("[WARN] mapper: unknown game state %q\n", d.State)
	}

	p := &model.Playthrough{
		PlaythroughId:     model.PlaythroughId(d.PlaythroughId), // string → PlaythroughId
		HostId:            model.ActorId(d.HostId),
		RulesId:           model.RulesId(d.RulesId),
		Map:               MapFromDTO(d.Map),
		Depth:             d.Depth,
		State:             state,
		DungParams:        dungParamsFromDTO(d.DungParams),
		DynamicDifficulty: d.DynamicDifficulty,
		NextId:            d.NextId,
		Seed:              d.Seed,
	}

	// PlayersUuid
	if d.PlayersUuid != nil {
		p.PlayersUuid = make(map[string]model.ActorId, len(d.PlayersUuid))
		for uuid, id := range d.PlayersUuid {
			p.PlayersUuid[uuid] = model.ActorId(id)
		}
	} else {
		p.PlayersUuid = make(map[string]model.ActorId)
	}

	p.Players = actorsMapFromDTO(d.Players)
	p.Monsters = actorsMapFromDTO(d.Monsters)
	p.DeadMonsters = actorsMapFromDTO(d.DeadMonsters)
	p.Items = itemsMapFromDTO(d.Items)

	// PlayersStats
	if d.PlayersStats != nil {
		p.PlayersStats = make(map[model.ActorId]*model.GameStats, len(d.PlayersStats))
		for k, v := range d.PlayersStats {
			p.PlayersStats[model.ActorId(k)] = gameStatsFromDTO(v)
		}
	} else {
		p.PlayersStats = make(map[model.ActorId]*model.GameStats)
	}

	// PlayersLevelMetrics
	if d.PlayersLevelMetrics != nil {
		p.PlayersLevelMetrics = make(map[model.ActorId]*model.LevelMetrics, len(d.PlayersLevelMetrics))
		for k, v := range d.PlayersLevelMetrics {
			p.PlayersLevelMetrics[model.ActorId(k)] = levelMetricsFromDTO(v)
		}
	} else {
		p.PlayersLevelMetrics = make(map[model.ActorId]*model.LevelMetrics)
	}

	// PlayersFoW
	if d.PlayersFoW != nil {
		p.PlayersFoW = make(map[model.ActorId]*model.VisibleArea, len(d.PlayersFoW))
		for k, v := range d.PlayersFoW {
			p.PlayersFoW[model.ActorId(k)] = visibleAreaFromDTO(v)
		}
	} else {
		p.PlayersFoW = make(map[model.ActorId]*model.VisibleArea)
	}

	// Restore transient state to zero values
	p.PendingIntents = make([]*model.Intent, 0)
	p.TurnEvents = make([]model.Event, 0)

	return p
}
