// package tui_adapter
//
// import (
// 	"cmp"
// 	"fmt"
// 	"log"
// 	"slices"
// 	"strconv"
//
// 	"github.com/unclestep/Rogue/internal/domain/model"
// 	"github.com/unclestep/Rogue/internal/domain/service"
// 	"github.com/unclestep/Rogue/internal/dto"
// 	"github.com/unclestep/Rogue/pkg/conv"
// 	"github.com/unclestep/Rogue/pkg/geometry"
// )
//
// type TuiMapper struct {
// 	Session *model.GameSession
// }
//
// func NewTuiMapper(session *model.GameSession) *TuiMapper {
// 	return &TuiMapper{
// 		Session: session,
// 	}
// }
//
// //
// //
// // --- WORLD ---
// //
// //
//
// func (t *TuiMapper) MakeWorldSnapshot(player *model.Actor, visibleArea *service.VisibleArea, events []service.Event) dto.WorldInfo {
// 	return dto.WorldInfo{
// 		Height: t.Session.Map.Rows,
// 		Width:  t.Session.Map.Cols,
// 		Grid:   t.constructGrid(visibleArea),
// 		State:  t.MapGameState(),
// 		Player: t.constructPlayer(player),
// 		Events: t.convertEvents(events),
// 	}
// }
//
// func (t *TuiMapper) constructGrid(visibleArea *service.VisibleArea) [][]*dto.Cell {
// 	rows := t.Session.Map.Rows
// 	cols := t.Session.Map.Cols
// 	world := make([][]*dto.Cell, rows)
//
// 	for r := 0; r < rows; r++ {
// 		world[r] = make([]*dto.Cell, cols)
// 		for c := 0; c < cols; c++ {
// 			world[r][c] = t.constructCell(r, c, visibleArea.Area[r][c])
// 		}
// 	}
// 	return world
// }
//
// func (t *TuiMapper) constructCell(row, col int, visibility service.VisibilityState) *dto.Cell {
// 	cellDTO := &dto.Cell{
// 		VisibilityState: t.MapVisibilityState(visibility),
// 	}
//
// 	if visibility == service.Visible || visibility == service.Explored {
// 		cellDTO.TopologyType = t.MapTopologyType(t.Session.Map.TileGrid[row][col].Type)
// 	}
//
// 	if visibility == service.Visible {
// 		cellDTO.Actor = t.ConstructActor(row, col)
// 		cellDTO.Item = t.ConstructItem(row, col)
// 	}
//
// 	return cellDTO
// }
//
// //
// //
// // --- EFFECTS ---
// //
// //
//
// func (t *TuiMapper) ConstructEffects(effects []*model.Effect) []*dto.Effect {
// 	effectsDTO := make([]*dto.Effect, 0, len(effects))
//
// 	for _, effect := range effects {
// 		effectsDTO = append(effectsDTO, &dto.Effect{
// 			Kind:         t.MapEffectType(effect.Kind),
// 			Duration:     effect.Duration,
// 			Charges:      effect.Charges,
// 			VitalsChange: t.convertVitals(effect.VitalsChange),
// 			AttrsChange:  t.convertAttrs(effect.AttrsChange),
// 			Buffs:        t.formatAttributes(effect.GetBuffs()),
// 			Debuffs:      t.formatAttributes(effect.GetDebuffs()),
// 			Statuses:     t.formatStatuses(effect.StatusesChange),
// 		})
// 	}
//
// 	return effectsDTO
// }
//
// //
// //
// // --- ACTORS ---
// //
// //
//
// func (t *TuiMapper) ConstructActor(row, col int) *dto.Actor {
// 	id := t.Session.Map.ActorGrid[row][col]
// 	if id == 0 {
// 		return nil
// 	}
//
// 	actor := t.Session.GetActor(model.ActorId(id))
// 	if actor == nil {
// 		t.Session.Map.RemoveActor(geometry.Point{X: col, Y: row})
// 		log.Printf("[ERROR] Desync at %d,%d: actor ID %d not found in session", col, row, id)
// 		return nil
// 	}
//
// 	return t.MapActorToDTO(actor)
// }
//
// func (t *TuiMapper) MapActorToDTO(actor *model.Actor) *dto.Actor {
// 	effVitalsChange, effAttrsChange := actor.GetEffectsInfluence()
//
// 	return &dto.Actor{
// 		Id:             int(actor.Id),
// 		Kind:           t.MapActorType(actor.Kind),
// 		BaseVitals:     t.convertVitals(actor.Vitals),
// 		BaseAttrs:      t.convertAttrs(actor.BaseAttrs),
// 		VitalsChange:   t.convertVitalsChange(effVitalsChange),
// 		AttrsChange:    t.convertAttrsChange(effAttrsChange),
// 		Vitals:         t.formatActorVitals(actor.Vitals, effVitalsChange),
// 		Attrs:          t.formatActorAttrs(actor.BaseAttrs, effAttrsChange),
// 		Statuses:       t.formatStatuses(actor.Statuses),
// 		AppliedEffects: t.ConstructEffects(actor.CollectActiveEffects()),
// 	}
// }
//
// //
// //
// // --- PLAYER ---
// //
// //
//
// func (t *TuiMapper) constructPlayer(actor *model.Actor) *dto.Player {
// 	if actor == nil {
// 		return nil
// 	}
//
// 	return &dto.Player{
// 		Actor:     t.MapActorToDTO(actor),
// 		HUD:       t.constructHUD(actor),
// 		Inventory: t.constructInventory(actor),
// 		Equipped:  t.constructEquipped(actor),
// 		RunStats:  t.constructRunStats(actor),
// 	}
// }
//
// func (t *TuiMapper) constructHUD(a *model.Actor) *dto.HUD {
// 	return &dto.HUD{
// 		HP:       a.Vitals[model.VitalHP],
// 		MaxHP:    a.DerivedAttrs[model.AttrMaxHP],
// 		Strength: a.DerivedAttrs[model.AttrStrength],
// 		Level:    t.Session.Depth,
// 		Treasure: a.Backpack.TreasuresValue,
// 	}
// }
//
// func (t *TuiMapper) constructInventory(a *model.Actor) map[dto.ItemType][]*dto.Item {
// 	inventory := make(map[dto.ItemType][]*dto.Item)
//
// 	for _, slot := range a.Backpack.Slots {
// 		for _, item := range slot {
// 			itemDTO := t.MapItemType(item.Kind)
// 			inventory[itemDTO] = append(inventory[itemDTO], t.ConvertItem(item))
// 		}
// 	}
//
// 	return inventory
// }
//
// func (t *TuiMapper) constructEquipped(a *model.Actor) map[dto.ItemType]*dto.Item {
// 	equipped := make(map[dto.ItemType]*dto.Item)
//
// 	for _, gear := range a.EquippedGear {
// 		gearDTO := t.MapItemType(gear.Kind)
// 		equipped[gearDTO] = t.ConvertItem(gear)
// 	}
//
// 	return equipped
// }
//
// func (t *TuiMapper) constructRunStats(a *model.Actor) *dto.RunStats {
// 	stats, exists := t.Session.PlayersStats[a.Id]
// 	if !exists {
// 		return nil
// 	}
//
// 	return &dto.RunStats{
// 		TotalTreasure:    stats.TotalTreasure,
// 		DeepestLevel:     stats.DeepestLevel,
// 		MonstersDefeated: stats.MonstersDefeated,
// 		FoodConsumed:     stats.FoodConsumed,
// 		ElixirsDrunk:     stats.ElixirsDrunk,
// 		ScrollsRead:      stats.ScrollsRead,
// 		HitsDealt:        stats.HitsDealt,
// 		HitsReceived:     stats.HitsReceived,
// 		TilesTraveled:    stats.TilesTraveled,
// 	}
// }
//
// //
// //
// // --- ITEMS ---
// //
// //
//
// func (t *TuiMapper) ConstructItem(row, col int) *dto.Item {
// 	id := t.Session.Map.ItemGrid[row][col]
// 	if id == 0 {
// 		return nil
// 	}
//
// 	item, exists := t.Session.Items[model.ItemId(id)]
// 	if !exists {
// 		t.Session.Map.RemoveItem(geometry.Point{X: col, Y: row})
// 		log.Printf("[ERROR] Desync at %d,%d: item ID %d not found in Session", col, row, id)
// 		return nil
// 	}
//
// 	return t.ConvertItem(item)
// }
//
// func (t *TuiMapper) ConvertItem(item *model.Item) *dto.Item {
// 	i := &dto.Item{
// 		Id:           int(item.Id),
// 		Kind:         t.MapItemType(item.Kind),
// 		Label:        t.MapItemLabel(item.Label),
// 		Keyhole:      t.MapItemKeyhole(item.Keyhole),
// 		VitalsChange: t.convertVitals(item.VitalsChange),
// 		AttrsChange:  t.convertAttrs(item.BaseAttrsChange),
// 	}
//
// 	for _, effect := range item.Effects {
// 		i.Effects = append(i.Effects, t.formatEffectType(effect.Kind))
// 	}
//
// 	return i
// }
//
// //
// //
// // --- ENUM MAPPERS ---
// //
// //
//
// func (t *TuiMapper) MapTopologyType(tile model.TileType) dto.TopologyType {
// 	switch tile {
// 	case model.Empty:
// 		return dto.TopologyEmpty
// 	case model.Wall:
// 		return dto.TopologyWall
// 	case model.Floor:
// 		return dto.TopologyFloor
// 	case model.OpenDoor:
// 		return dto.TopologyOpenDoor
// 	case model.ClosedDoor:
// 		return dto.TopologyClosedDoor
// 	case model.Corridor:
// 		return dto.TopologyCorridor
// 	case model.Exit:
// 		return dto.TopologyExit
// 	default:
// 		log.Printf("[WARNING] Unknown tile type detected: %v\n", tile)
// 		return dto.TopologyUnknown
//
// 	}
// }
//
// func (t *TuiMapper) MapActorType(actorType model.ActorType) dto.ActorKind {
// 	return dto.ActorKind(actorType)
// }
//
// func (t *TuiMapper) MapItemType(itemType model.ItemType) dto.ItemType {
// 	return dto.ItemType(itemType)
// }
//
// func (t *TuiMapper) MapItemLabel(itemLabel model.ItemLabel) dto.ItemLabel {
// 	return dto.ItemLabel(itemLabel)
// }
//
// func (t *TuiMapper) MapItemKeyhole(itemKeyhole model.Keyhole) dto.Keyhole {
// 	switch itemKeyhole {
// 	case model.KeyholeNone:
// 		return dto.KeyholeNone
// 	default:
// 		return dto.Keyhole(itemKeyhole)
// 	}
// }
//
// func (t *TuiMapper) MapVitalType(vital model.VitalType) dto.VitalType {
// 	switch vital {
// 	case model.VitalHP:
// 		return dto.VitalHP
// 	case model.VitalStamina:
// 		return dto.VitalStamina
// 	default:
// 		log.Printf("[WARNING] Unknown vital type detected: %v\n", vital)
// 		return dto.VitalUnknown
// 	}
// }
//
// func (t *TuiMapper) MapAttrType(attr model.AttrType) dto.AttrType {
// 	switch attr {
// 	case model.AttrMaxHP:
// 		return dto.MaxHealth
// 	case model.AttrMaxStamina:
// 		return dto.MaxStamina
// 	case model.AttrAttackStaminaCost:
// 		return dto.AttackStaminaCost
// 	case model.AttrMoveStaminaCost:
// 		return dto.MoveStaminaCost
// 	case model.AttrActionStaminaCost:
// 		return dto.ActionStaminaCost
// 	case model.AttrStaminaRegen:
// 		return dto.StaminaRegen
// 	case model.AttrStrength:
// 		return dto.Strength
// 	case model.AttrDexterity:
// 		return dto.Dexterity
// 	case model.AttrHostility:
// 		return dto.Hostility
// 	case model.AttrCounterAttackChance:
// 		return dto.CounterAttackChance
// 	default:
// 		log.Printf("[WARNING] Unknown attribute detected: %v\n", attr)
// 		return dto.AttrUnknown
// 	}
// }
//
// func (t *TuiMapper) MapEffectType(effectType model.EffectType) dto.EffectKind {
// 	return dto.EffectKind(effectType)
// }
//
// func (t *TuiMapper) MapGameState() dto.GameState {
// 	switch t.Session.State {
// 	case model.LobbyGameState:
// 		return dto.StateLobby
// 	case model.PlayingGameState:
// 		return dto.StatePlaying
// 	case model.GameOverGameState:
// 		return dto.StateGameover
// 	default:
// 		log.Printf("[WARNING] Unknown session state detected: %v\n", t.Session.State)
// 		return dto.StateUnknown
// 	}
// }
//
// func (t *TuiMapper) MapVisibilityState(vs service.VisibilityState) dto.VisibilityState {
// 	switch vs {
// 	case service.Unexplored:
// 		return dto.VisibilityUnexplored
// 	case service.Visible:
// 		return dto.VisibilityVisible
// 	case service.Explored:
// 		return dto.VisibilityExplored
// 	default:
// 		log.Printf("[WARNING] Unknown visibility state detected: %v\n", vs)
// 		return dto.VisibilityUnknown
// 	}
// }
//
// //
// //
// // --- CONVERTERS ---
// //
// //
//
// //
// // -- DTO CONVERTERS --
// //
//
// func (t *TuiMapper) convertVitals(vitals map[model.VitalType]int) map[dto.VitalType]int {
// 	vitalsDTO := make(map[dto.VitalType]int, len(vitals))
//
// 	for vital, val := range vitals {
// 		vitalsDTO[t.MapVitalType(vital)] = val
// 	}
//
// 	return vitalsDTO
// }
//
// func (t *TuiMapper) convertAttrs(attrs map[model.AttrType]int) map[dto.AttrType]int {
// 	attrsDTO := make(map[dto.AttrType]int, len(attrs))
//
// 	for attr, val := range attrs {
// 		attrsDTO[t.MapAttrType(attr)] = val
// 	}
//
// 	return attrsDTO
// }
//
// func (t *TuiMapper) convertVitalsChange(vitalsChange map[model.VitalType][]int) map[dto.VitalType][]int {
// 	vitalsChangeDTO := make(map[dto.VitalType][]int, len(vitalsChange))
//
// 	for vital := range vitalsChange {
// 		vitalsChangeDTO[t.MapVitalType(vital)] = slices.Clone(vitalsChange[vital])
// 	}
//
// 	return vitalsChangeDTO
// }
//
// func (t *TuiMapper) convertAttrsChange(attrsChange map[model.AttrType][]int) map[dto.AttrType][]int {
// 	attrsChangeDTO := make(map[dto.AttrType][]int, len(attrsChange))
//
// 	for attr := range attrsChange {
// 		attrsChangeDTO[t.MapAttrType(attr)] = slices.Clone(attrsChange[attr])
// 	}
//
// 	return attrsChangeDTO
// }
//
// //
// // -- STRING CONVERTERS --
// //
//
// func (t *TuiMapper) convertEvents(events []service.Event) []*dto.Event {
// 	eventsDTO := make([]*dto.Event, 0, len(events))
//
// 	for _, event := range events {
// 		eventsDTO = append(eventsDTO, t.convertEvent(event))
// 	}
//
// 	return eventsDTO
// }
//
// func (t *TuiMapper) convertEvent(event service.Event) *dto.Event {
// 	switch e := event.(type) {
// 	case *service.AttackEvent:
// 		return &dto.Event{
// 			EventType: dto.EventAttack,
// 			Desc:      t.formatAttackEvent(e),
// 		}
//
// 	case *service.MoveEvent:
// 		return &dto.Event{
// 			EventType: dto.EventMove,
// 			Desc:      t.formatMoveEvent(e),
// 		}
// 	case *service.ItemPickupEvent:
// 		return &dto.Event{
// 			EventType: dto.EventItemPickup,
// 			Desc:      t.formatItemPickupEvent(e),
// 		}
// 	case *service.ItemUsageEvent:
// 		return &dto.Event{
// 			EventType: dto.EventItemUsage,
// 			Desc:      t.formatItemUsageEvent(e),
// 		}
// 	default:
// 		log.Printf("[WARNING] Unknown event detected: %T", event)
// 		return &dto.Event{
// 			EventType: dto.EventUnknown,
// 			Desc:      fmt.Sprintf("[Unknown Event: %T]", event),
// 		}
// 	}
// }
//
// func (t *TuiMapper) formatAttackEvent(event *service.AttackEvent) string {
// 	attacker := event.Attacker.Actor
// 	defender := event.Defender.Actor
//
// 	switch event.Outcome {
// 	case service.AttackOutcomeSuccess:
// 		return fmt.Sprintf("%v#%v hits %v#%v for %v damage", t.formatActorType(attacker.Kind), attacker.Id, t.formatActorType(defender.Kind), defender.Id, event.Defender.VitalsChange[model.VitalHP])
// 	case service.AttackOutcomeMissed:
// 		return fmt.Sprintf("%v#%v misses %v#%v", t.formatActorType(attacker.Kind), attacker.Id, t.formatActorType(defender.Kind), defender.Id)
// 	default:
// 		return ""
// 	}
// }
//
// func (t *TuiMapper) formatMoveEvent(event *service.MoveEvent) string {
// 	return ""
// }
//
// func (t *TuiMapper) formatItemPickupEvent(event *service.ItemPickupEvent) string {
// 	return ""
// }
//
// func (t *TuiMapper) formatItemUsageEvent(event *service.ItemUsageEvent) string {
// 	return ""
// }
//
// func (t *TuiMapper) formatActorType(actorType model.ActorType) string {
// 	switch actorType {
// 	case model.ActorPlayer:
// 		return "Player"
// 	case model.ActorZombie:
// 		return "Zombie"
// 	case model.ActorVampire:
// 		return "Vampire"
// 	case model.ActorGhost:
// 		return "Ghost"
// 	case model.ActorOgre:
// 		return "Ogre"
// 	case model.ActorSnakeMage:
// 		return "SnakeMage"
// 	case model.ActorMimic:
// 		return "Mimic"
// 	default:
// 		log.Printf("[WARNING] Unknown actor type detected: %v\n", actorType)
// 		return "Unknown creature"
// 	}
// }
//
// func (t *TuiMapper) formatActorVitals(baseVitals map[model.VitalType]int, vitalsChange map[model.VitalType][]int) map[string][]string {
// 	stringVitals := make(map[string][]string, len(baseVitals))
//
// 	baseVitalsKV := conv.MapToKV(baseVitals)
// 	slices.SortFunc(baseVitalsKV, func(a, b conv.KV[model.VitalType, int]) int { return cmp.Compare(a.Key, b.Key) })
//
// 	for _, vitalKV := range baseVitalsKV {
//
// 		stringKey := t.formatVitalType(vitalKV.Key)
// 		stringVitals[stringKey] = make([]string, 0, 8)
// 		stringVitals[stringKey] = append(stringVitals[stringKey], fmt.Sprintf("%d (base)", vitalKV.Val))
//
// 		for _, change := range vitalsChange[vitalKV.Key] {
// 			stringVitals[stringKey] = append(stringVitals[stringKey], strconv.Itoa(change))
// 		}
// 	}
//
// 	return stringVitals
// }
//
// func (t *TuiMapper) formatActorAttrs(baseAttrs map[model.AttrType]int, attrsChange map[model.AttrType][]int) map[string][]string {
// 	stringAttrs := make(map[string][]string, len(baseAttrs))
//
// 	baseAttrsKV := conv.MapToKV(baseAttrs)
// 	slices.SortFunc(baseAttrsKV, func(a, b conv.KV[model.AttrType, int]) int { return cmp.Compare(a.Key, b.Key) })
//
// 	for _, attrKV := range baseAttrsKV {
//
// 		stringKey := t.formatAttrType(attrKV.Key)
// 		stringAttrs[stringKey] = make([]string, 0, 8)
// 		stringAttrs[stringKey] = append(stringAttrs[stringKey], fmt.Sprintf("%d (base)", attrKV.Val))
//
// 		for _, change := range attrsChange[attrKV.Key] {
// 			stringAttrs[stringKey] = append(stringAttrs[stringKey], strconv.Itoa(change))
// 		}
// 	}
//
// 	return stringAttrs
// }
//
// func (t *TuiMapper) formatAttributes(attrsChange map[model.AttrType]int) []string {
// 	stringAttrs := make([]string, 0, len(attrsChange))
//
// 	attrs := conv.MapToKV(attrsChange)
// 	slices.SortFunc(attrs, func(a, b conv.KV[model.AttrType, int]) int { return cmp.Compare(a.Key, b.Key) })
//
// 	for _, attr := range attrs {
// 		if attr.Val == 0 {
// 			continue
// 		}
//
// 		str := fmt.Sprintf("%s%+d %s", attr.Val, t.formatAttrType(attr.Key))
// 		stringAttrs = append(stringAttrs, str)
// 	}
//
// 	return stringAttrs
// }
//
// func (t *TuiMapper) formatStatuses(statusesChange map[model.StatusType]int) []string {
// 	var stringStatuses []string
//
// 	for status, val := range statusesChange {
// 		if val <= 0 {
// 			continue
// 		}
// 		stringStatuses = append(stringStatuses, t.formatStatus(status))
// 	}
//
// 	return stringStatuses
// }
//
// func (t *TuiMapper) formatAttrType(attr model.AttrType) string {
// 	switch attr {
// 	case model.AttrMaxHP:
// 		return "Max HP"
// 	case model.AttrMaxStamina:
// 		return "Max stamina"
// 	case model.AttrAttackStaminaCost:
// 		return "Attack cost"
// 	case model.AttrMoveStaminaCost:
// 		return "Move cost"
// 	case model.AttrActionStaminaCost:
// 		return "Action cost"
// 	case model.AttrStaminaRegen:
// 		return "Stamina regeneration"
// 	case model.AttrStrength:
// 		return "Strength"
// 	case model.AttrDexterity:
// 		return "Dexterity"
// 	case model.AttrHostility:
// 		return "Hostility"
// 	case model.AttrCounterAttackChance:
// 		return "Counter-attack chance"
// 	default:
// 		log.Printf("[WARNING] Unknown attribute type detected: %v\n", attr)
// 		return "Unknown attribute"
// 	}
// }
//
// func (t *TuiMapper) formatVitalType(vital model.VitalType) string {
// 	switch vital {
// 	case model.VitalHP:
// 		return "HP"
// 	case model.VitalStamina:
// 		return "Stamina"
// 	default:
// 		log.Printf("[WARNING] Unknown vital type detected: %v\n", vital)
// 		return "Unknown vital"
// 	}
// }
//
// func (t *TuiMapper) formatEffectType(effectType model.EffectType) string {
// 	return string(effectType)
// }
//
// func (t *TuiMapper) formatStatus(status model.StatusType) string {
// 	switch status {
// 	case model.StatusSleep:
// 		return "Sleep"
// 	case model.StatusFatigue:
// 		return "Fatigue"
// 	case model.StatusUntouchable:
// 		return "Untouchable"
// 	case model.StatusInfallible:
// 		return "Infallible"
// 	default:
// 		log.Printf("[WARNING] Unknown status detected: %v\n", status)
// 		return "Unknown status"
// 	}
// }
