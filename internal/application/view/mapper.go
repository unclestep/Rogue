package view

import (
	"cmp"
	"fmt"
	"log"
	"slices"
	"strconv"

	"github.com/unclestep/Rogue/internal/application/port"
	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/domain/service"
	"github.com/unclestep/Rogue/internal/dto"
	"github.com/unclestep/Rogue/pkg/conv"
	"github.com/unclestep/Rogue/pkg/geometry"
)

type ViewMapper struct {
	repo port.PlaythroughRepository
}

func NewTuiMapper(repo port.PlaythroughRepository) *ViewMapper {
	return &ViewMapper{
		repo: repo,
	}
}

//
//
// --- WORLD ---
//
//

func (t *ViewMapper) MakeSnapshots(id model.PlaythroughId) map[string]*dto.GameView {
	playthrough, _ := t.repo.Get(id)
	snapshots := make(map[string]*dto.GameView, len(playthrough.PlayersUuid))

	for playerUuid := range playthrough.PlayersUuid {
		player := playthrough.GetPlayer(playerUuid)
		snapshots[playerUuid] = &dto.GameView{
			Height: playthrough.Map.Height,
			Width:  playthrough.Map.Width,
			Grid:   t.constructGrid(playthrough.Map, playthrough.PlayersFoW[player.Id]),
			State:  t.MapGameState(),
			Player: t.constructPlayer(player),
			Events: t.convertEvents(playthrough.TurnEvents),
		}
	}

	return snapshots
}

func (t *ViewMapper) constructGrid(m *model.Map, fow *model.VisibleArea) [][]*dto.Cell {
	rows := m.Height
	cols := m.Width
	world := make([][]*dto.Cell, rows)

	for r := 0; r < rows; r++ {
		world[r] = make([]*dto.Cell, cols)
		for c := 0; c < cols; c++ {
			world[r][c] = t.constructCell(m, fow.Area[r][c], r, c)
		}
	}
	return world
}

func (t *ViewMapper) constructCell(m *model.Map, visibility model.VisibilityState, r, c int) *dto.Cell {
	cellDTO := &dto.Cell{
		VisibilityState: t.MapVisibilityState(visibility),
	}

	if visibility == model.Visible || visibility == model.Explored {
		tileType, _ := m.GetTileType(geometry.Point{X: c, Y: r})
		cellDTO.TopologyType = t.MapTopologyType(tileType)
	}

	if visibility == model.Visible {
		cellDTO.Actor = t.ConstructActor(m, r, c)
		cellDTO.Item = t.ConstructItem(m, r, c)
	}

	return cellDTO
}

//
//
// --- EFFECTS ---
//
//

func (t *ViewMapper) ConstructEffects(effects []*model.Effect) []*dto.Effect {
	effectsDTO := make([]*dto.Effect, 0, len(effects))

	for _, effect := range effects {
		effectsDTO = append(effectsDTO, &dto.Effect{
			Kind:         t.MapEffectType(effect.Kind),
			Duration:     effect.Duration,
			Charges:      effect.Charges,
			VitalsChange: t.convertVitals(effect.VitalsChange),
			AttrsChange:  t.convertAttrs(effect.AttrsChange),
			Buffs:        t.formatAttributes(effect.GetBuffs()),
			Debuffs:      t.formatAttributes(effect.GetDebuffs()),
			Statuses:     t.formatStatuses(effect.StatusesChange),
		})
	}

	return effectsDTO
}

//
//
// --- ACTORS ---
//
//

func (t *ViewMapper) ConstructActor(m *model.Map, row, col int) *dto.Actor {
	id := m.GetActorGrid()[row][col]
	if id == 0 {
		return nil
	}

	actor := t.Session.GetActor(model.ActorId(id))

	return t.MapActorToDTO(actor)
}

func (t *ViewMapper) MapActorToDTO(actor *model.Actor) *dto.Actor {
	effVitalsChange, effAttrsChange := actor.GetEffectsInfluence()

	return &dto.Actor{
		Kind:           t.MapActorType(actor.Kind),
		BaseVitals:     t.convertVitals(actor.Vitals),
		BaseAttrs:      t.convertAttrs(actor.BaseAttrs),
		VitalsChange:   t.convertVitalsChange(effVitalsChange),
		AttrsChange:    t.convertAttrsChange(effAttrsChange),
		Vitals:         t.formatActorVitals(actor.Vitals, effVitalsChange),
		Attrs:          t.formatActorAttrs(actor.BaseAttrs, effAttrsChange),
		Statuses:       t.formatStatuses(actor.Statuses),
		AppliedEffects: t.ConstructEffects(actor.CollectActiveEffects()),
	}
}

//
//
// --- PLAYER ---
//
//

func (t *ViewMapper) constructPlayer(actor *model.Actor) *dto.Player {
	if actor == nil {
		return nil
	}

	return &dto.Player{
		Actor:     t.MapActorToDTO(actor),
		HUD:       t.constructHUD(actor),
		Inventory: t.constructInventory(actor),
		Equipped:  t.constructEquipped(actor),
		RunStats:  t.constructRunStats(actor),
	}
}

func (t *ViewMapper) constructHUD(a *model.Actor) *dto.HUD {
	return &dto.HUD{
		HP:       a.Vitals[model.VitalHP],
		MaxHP:    a.DerivedAttrs[model.AttrMaxHP],
		Strength: a.DerivedAttrs[model.AttrStrength],
		Dungeon:  t.Session.Depth,
		Treasure: a.Backpack.TreasuresValue,
	}
}

func (t *ViewMapper) constructInventory(a *model.Actor) map[dto.ItemType][]*dto.Item {
	inventory := make(map[dto.ItemType][]*dto.Item)

	for _, slot := range a.Backpack.Slots {
		for _, item := range slot {
			itemDTO := t.MapItemType(item.Kind)
			inventory[itemDTO] = append(inventory[itemDTO], t.ConvertItem(item))
		}
	}

	return inventory
}

func (t *ViewMapper) constructEquipped(a *model.Actor) map[dto.ItemType]*dto.Item {
	equipped := make(map[dto.ItemType]*dto.Item)

	for _, gear := range a.EquippedGear {
		gearDTO := t.MapItemType(gear.Kind)
		equipped[gearDTO] = t.ConvertItem(gear)
	}

	return equipped
}

func (t *ViewMapper) constructRunStats(a *model.Actor) *dto.RunStats {
	stats, exists := t.Session.PlayersStats[a.Id]
	if !exists {
		return nil
	}

	return &dto.RunStats{
		TotalTreasure:    stats.TotalTreasure,
		DeepestLevel:     stats.DeepestLevel,
		MonstersDefeated: stats.MonstersDefeated,
		FoodConsumed:     stats.FoodConsumed,
		ElixirsDrunk:     stats.ElixirsDrunk,
		ScrollsRead:      stats.ScrollsRead,
		HitsDealt:        stats.HitsDealt,
		HitsReceived:     stats.HitsReceived,
		TilesTraveled:    stats.TilesTraveled,
	}
}

//
//
// --- ITEMS ---
//
//

func (t *ViewMapper) ConstructItem(row, col int) *dto.Item {
	id := t.Session.Map.ItemGrid[row][col]
	if id == 0 {
		return nil
	}

	item, exists := t.Session.Items[model.ItemId(id)]
	if !exists {
		t.Session.Map.RemoveItem(geometry.Point{X: col, Y: row})
		log.Printf("[ERROR] Desync at %d,%d: item ID %d not found in Session", col, row, id)
		return nil
	}

	return t.ConvertItem(item)
}

func (t *ViewMapper) ConvertItem(item *model.Item) *dto.Item {
	i := &dto.Item{
		Id:           int(item.Id),
		Kind:         t.MapItemType(item.Kind),
		Label:        t.MapItemLabel(item.Label),
		Keyhole:      t.MapItemKeyhole(item.Keyhole),
		VitalsChange: t.convertVitals(item.VitalsChange),
		AttrsChange:  t.convertAttrs(item.BaseAttrsChange),
	}

	for _, effect := range item.Effects {
		i.Effects = append(i.Effects, t.formatEffectType(effect.Kind))
	}

	return i
}

//
//
// --- ENUM MAPPERS ---
//
//

func (t *ViewMapper) MapTopologyType(tile model.TileType) dto.TopologyType {
	switch tile {
	case model.Empty:
		return dto.TopologyEmpty
	case model.Wall:
		return dto.TopologyWall
	case model.Floor:
		return dto.TopologyFloor
	case model.OpenDoor:
		return dto.TopologyOpenDoor
	case model.ClosedDoor:
		return dto.TopologyClosedDoor
	case model.Corridor:
		return dto.TopologyCorridor
	case model.Exit:
		return dto.TopologyExit
	default:
		log.Printf("[WARNING] Unknown tile type detected: %v\n", tile)
		return dto.TopologyUnknown

	}
}

func (t *ViewMapper) MapActorType(actorType model.ActorType) dto.ActorKind {
	return dto.ActorKind(actorType)
}

func (t *ViewMapper) MapItemType(itemType model.ItemType) dto.ItemType {
	return dto.ItemType(itemType)
}

func (t *ViewMapper) MapItemLabel(itemLabel model.ItemLabel) dto.ItemLabel {
	return dto.ItemLabel(itemLabel)
}

func (t *ViewMapper) MapItemKeyhole(itemKeyhole model.Keyhole) dto.Keyhole {
	switch itemKeyhole {
	case model.KeyholeNone:
		return dto.KeyholeNone
	default:
		return dto.Keyhole(itemKeyhole)
	}
}

func (t *ViewMapper) MapVitalType(vital model.VitalType) dto.VitalType {
	switch vital {
	case model.VitalHP:
		return dto.VitalHP
	case model.VitalStamina:
		return dto.VitalStamina
	default:
		log.Printf("[WARNING] Unknown vital type detected: %v\n", vital)
		return dto.VitalUnknown
	}
}

func (t *ViewMapper) MapAttrType(attr model.AttrType) dto.AttrType {
	switch attr {
	case model.AttrMaxHP:
		return dto.MaxHealth
	case model.AttrMaxStamina:
		return dto.MaxStamina
	case model.AttrAttackStaminaCost:
		return dto.AttackStaminaCost
	case model.AttrMoveStaminaCost:
		return dto.MoveStaminaCost
	case model.AttrActionStaminaCost:
		return dto.ActionStaminaCost
	case model.AttrStaminaRegen:
		return dto.StaminaRegen
	case model.AttrStrength:
		return dto.Strength
	case model.AttrDexterity:
		return dto.Dexterity
	case model.AttrHostility:
		return dto.Hostility
	case model.AttrCounterAttackChance:
		return dto.CounterAttackChance
	default:
		log.Printf("[WARNING] Unknown attribute detected: %v\n", attr)
		return dto.AttrUnknown
	}
}

func (t *ViewMapper) MapEffectType(effectType model.EffectType) dto.EffectKind {
	return dto.EffectKind(effectType)
}

func (t *ViewMapper) MapGameState() dto.GameState {
	switch t.Session.State {
	case model.LobbyGameState:
		return dto.StateLobby
	case model.PlayingGameState:
		return dto.StatePlaying
	case model.GameOverGameState:
		return dto.StateGameover
	default:
		log.Printf("[WARNING] Unknown session state detected: %v\n", t.Session.State)
		return dto.StateUnknown
	}
}

func (t *ViewMapper) MapVisibilityState(vs service.VisibilityState) dto.VisibilityState {
	switch vs {
	case service.Unexplored:
		return dto.VisibilityUnexplored
	case service.Visible:
		return dto.VisibilityVisible
	case service.Explored:
		return dto.VisibilityExplored
	default:
		log.Printf("[WARNING] Unknown visibility state detected: %v\n", vs)
		return dto.VisibilityUnknown
	}
}

//
//
// --- CONVERTERS ---
//
//

//
// -- DTO CONVERTERS --
//

func (t *ViewMapper) convertVitals(vitals map[model.VitalType]int) map[dto.VitalType]int {
	vitalsDTO := make(map[dto.VitalType]int, len(vitals))

	for vital, val := range vitals {
		vitalsDTO[t.MapVitalType(vital)] = val
	}

	return vitalsDTO
}

func (t *ViewMapper) convertAttrs(attrs map[model.AttrType]int) map[dto.AttrType]int {
	attrsDTO := make(map[dto.AttrType]int, len(attrs))

	for attr, val := range attrs {
		attrsDTO[t.MapAttrType(attr)] = val
	}

	return attrsDTO
}

func (t *ViewMapper) convertVitalsChange(vitalsChange map[model.VitalType][]int) map[dto.VitalType][]int {
	vitalsChangeDTO := make(map[dto.VitalType][]int, len(vitalsChange))

	for vital := range vitalsChange {
		vitalsChangeDTO[t.MapVitalType(vital)] = slices.Clone(vitalsChange[vital])
	}

	return vitalsChangeDTO
}

func (t *ViewMapper) convertAttrsChange(attrsChange map[model.AttrType][]int) map[dto.AttrType][]int {
	attrsChangeDTO := make(map[dto.AttrType][]int, len(attrsChange))

	for attr := range attrsChange {
		attrsChangeDTO[t.MapAttrType(attr)] = slices.Clone(attrsChange[attr])
	}

	return attrsChangeDTO
}

//
// -- STRING CONVERTERS --
//

func (t *ViewMapper) convertEvents(events []service.Event) []*dto.Event {
	eventsDTO := make([]*dto.Event, 0, len(events))

	for _, event := range events {
		eventsDTO = append(eventsDTO, t.convertEvent(event))
	}

	return eventsDTO
}

func (t *ViewMapper) convertEvent(event service.Event) *dto.Event {
	switch e := event.(type) {
	case *service.AttackEvent:
		return &dto.Event{
			EventType: dto.EventAttack,
			Desc:      t.formatAttackEvent(e),
		}

	case *service.MoveEvent:
		return &dto.Event{
			EventType: dto.EventMove,
			Desc:      t.formatMoveEvent(e),
		}
	case *service.ItemPickupEvent:
		return &dto.Event{
			EventType: dto.EventItemPickup,
			Desc:      t.formatItemPickupEvent(e),
		}
	case *service.ItemUsageEvent:
		return &dto.Event{
			EventType: dto.EventItemUsage,
			Desc:      t.formatItemUsageEvent(e),
		}
	default:
		log.Printf("[WARNING] Unknown event detected: %T", event)
		return &dto.Event{
			EventType: dto.EventUnknown,
			Desc:      fmt.Sprintf("[Unknown Event: %T]", event),
		}
	}
}

func (t *ViewMapper) formatAttackEvent(event *service.AttackEvent) string {
	attacker := event.Attacker.Actor
	defender := event.Defender.Actor

	switch event.Outcome {
	case service.AttackOutcomeSuccess:
		return fmt.Sprintf("%v#%v hits %v#%v for %v damage", t.formatActorType(attacker.Kind), attacker.Id, t.formatActorType(defender.Kind), defender.Id, event.Defender.VitalsChange[model.VitalHP])
	case service.AttackOutcomeMissed:
		return fmt.Sprintf("%v#%v misses %v#%v", t.formatActorType(attacker.Kind), attacker.Id, t.formatActorType(defender.Kind), defender.Id)
	default:
		return ""
	}
}

func (t *ViewMapper) formatMoveEvent(event *service.MoveEvent) string {
	return ""
}

func (t *ViewMapper) formatItemPickupEvent(event *service.ItemPickupEvent) string {
	return ""
}

func (t *ViewMapper) formatItemUsageEvent(event *service.ItemUsageEvent) string {
	return ""
}

func (t *ViewMapper) formatActorType(actorType model.ActorType) string {
	switch actorType {
	case model.ActorPlayer:
		return "Player"
	case model.ActorZombie:
		return "Zombie"
	case model.ActorVampire:
		return "Vampire"
	case model.ActorGhost:
		return "Ghost"
	case model.ActorOgre:
		return "Ogre"
	case model.ActorSnakeMage:
		return "SnakeMage"
	case model.ActorMimic:
		return "Mimic"
	default:
		log.Printf("[WARNING] Unknown actor type detected: %v\n", actorType)
		return "Unknown creature"
	}
}

func (t *ViewMapper) formatActorVitals(baseVitals map[model.VitalType]int, vitalsChange map[model.VitalType][]int) map[string][]string {
	stringVitals := make(map[string][]string, len(baseVitals))

	baseVitalsKV := conv.MapToKV(baseVitals)
	slices.SortFunc(baseVitalsKV, func(a, b conv.KV[model.VitalType, int]) int { return cmp.Compare(a.Key, b.Key) })

	for _, vitalKV := range baseVitalsKV {

		stringKey := t.formatVitalType(vitalKV.Key)
		stringVitals[stringKey] = make([]string, 0, 8)
		stringVitals[stringKey] = append(stringVitals[stringKey], fmt.Sprintf("%d (base)", vitalKV.Val))

		for _, change := range vitalsChange[vitalKV.Key] {
			stringVitals[stringKey] = append(stringVitals[stringKey], strconv.Itoa(change))
		}
	}

	return stringVitals
}

func (t *ViewMapper) formatActorAttrs(baseAttrs map[model.AttrType]int, attrsChange map[model.AttrType][]int) map[string][]string {
	stringAttrs := make(map[string][]string, len(baseAttrs))

	baseAttrsKV := conv.MapToKV(baseAttrs)
	slices.SortFunc(baseAttrsKV, func(a, b conv.KV[model.AttrType, int]) int { return cmp.Compare(a.Key, b.Key) })

	for _, attrKV := range baseAttrsKV {

		stringKey := t.formatAttrType(attrKV.Key)
		stringAttrs[stringKey] = make([]string, 0, 8)
		stringAttrs[stringKey] = append(stringAttrs[stringKey], fmt.Sprintf("%d (base)", attrKV.Val))

		for _, change := range attrsChange[attrKV.Key] {
			stringAttrs[stringKey] = append(stringAttrs[stringKey], strconv.Itoa(change))
		}
	}

	return stringAttrs
}

func (t *ViewMapper) formatAttributes(attrsChange map[model.AttrType]int) []string {
	stringAttrs := make([]string, 0, len(attrsChange))

	attrs := conv.MapToKV(attrsChange)
	slices.SortFunc(attrs, func(a, b conv.KV[model.AttrType, int]) int { return cmp.Compare(a.Key, b.Key) })

	for _, attr := range attrs {
		if attr.Val == 0 {
			continue
		}

		str := fmt.Sprintf("%s%+d %s", attr.Val, t.formatAttrType(attr.Key))
		stringAttrs = append(stringAttrs, str)
	}

	return stringAttrs
}

func (t *ViewMapper) formatStatuses(statusesChange map[model.StatusType]int) []string {
	var stringStatuses []string

	for status, val := range statusesChange {
		if val <= 0 {
			continue
		}
		stringStatuses = append(stringStatuses, t.formatStatus(status))
	}

	return stringStatuses
}

func (t *ViewMapper) formatAttrType(attr model.AttrType) string {
	switch attr {
	case model.AttrMaxHP:
		return "Max HP"
	case model.AttrMaxStamina:
		return "Max stamina"
	case model.AttrAttackStaminaCost:
		return "Attack cost"
	case model.AttrMoveStaminaCost:
		return "Move cost"
	case model.AttrActionStaminaCost:
		return "Action cost"
	case model.AttrStaminaRegen:
		return "Stamina regeneration"
	case model.AttrStrength:
		return "Strength"
	case model.AttrDexterity:
		return "Dexterity"
	case model.AttrHostility:
		return "Hostility"
	case model.AttrCounterAttackChance:
		return "Counter-attack chance"
	default:
		log.Printf("[WARNING] Unknown attribute type detected: %v\n", attr)
		return "Unknown attribute"
	}
}

func (t *ViewMapper) formatVitalType(vital model.VitalType) string {
	switch vital {
	case model.VitalHP:
		return "HP"
	case model.VitalStamina:
		return "Stamina"
	default:
		log.Printf("[WARNING] Unknown vital type detected: %v\n", vital)
		return "Unknown vital"
	}
}

func (t *ViewMapper) formatEffectType(effectType model.EffectType) string {
	return string(effectType)
}

func (t *ViewMapper) formatStatus(status model.StatusType) string {
	switch status {
	case model.StatusSleep:
		return "Sleep"
	case model.StatusFatigue:
		return "Fatigue"
	case model.StatusUntouchable:
		return "Untouchable"
	case model.StatusInfallible:
		return "Infallible"
	default:
		log.Printf("[WARNING] Unknown status detected: %v\n", status)
		return "Unknown status"
	}
}
