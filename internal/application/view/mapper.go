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

type Mapper struct {
	repo port.PlaythroughRepository
}

func NewMapper(repo port.PlaythroughRepository) *Mapper {
	return &Mapper{
		repo: repo,
	}
}

//
//
// --- WORLD ---
//
//

func (t *Mapper) MakeSnapshots(id model.PlaythroughId) map[string]*dto.GameView {
	playthrough, err := t.repo.Get(id)
	if err != nil || playthrough == nil {
		return nil
	}

	snapshots := make(map[string]*dto.GameView, len(playthrough.PlayersUuid))

	for playerUuid := range playthrough.PlayersUuid {
		player := playthrough.GetPlayer(playerUuid)
		if player == nil {
			continue
		}

		view := &dto.GameView{
			PlaythroughId: string(id),
			State:         t.mapGameState(playthrough),
			Player:        t.constructPlayer(playthrough, player, playerUuid),
			Events:        t.convertEvents(playthrough.TurnEvents),
			LobbyPlayers:  t.constructLobbyPlayers(playthrough),
			Leaderboard:   t.buildLeaderboard(playthrough),
		}

		if playthrough.Map != nil {
			view.Height = playthrough.Map.Height
			view.Width = playthrough.Map.Width
			view.Grid = t.constructGrid(playthrough, playthrough.Map, playthrough.PlayersFoW[player.Id])
		}

		snapshots[playerUuid] = view
	}

	return snapshots
}

func (t *Mapper) constructGrid(playthrough *model.Playthrough, m *model.Map, fow *model.VisibleArea) [][]*dto.Cell {
	rows := m.Height
	cols := m.Width
	world := make([][]*dto.Cell, rows)

	for r := range rows {
		world[r] = make([]*dto.Cell, cols)
		for c := range cols {
			// Default to Unexplored so players who lack a FOW entry (e.g.
			// right after dungeon generation, before the first turn) see
			// darkness rather than the entire map.
			visibility := model.Unexplored
			if fow != nil {
				visibility = fow.Area[r][c]
			}
			world[r][c] = t.constructCell(playthrough, m, visibility, r, c)
		}
	}
	return world
}

func (t *Mapper) constructCell(playthrough *model.Playthrough, m *model.Map, visibility model.VisibilityState, r, c int) *dto.Cell {
	cellDTO := &dto.Cell{
		VisibilityState: t.mapVisibilityState(visibility),
	}

	if visibility == model.Visible || visibility == model.Explored {
		pos := geometry.Point{X: c, Y: r}
		tileType, _ := m.GetTileType(pos)
		cellDTO.TopologyType = t.mapTopologyType(tileType)

		if tileType == model.ClosedDoor {
			if keyhole, ok := m.GetDoorKeyhole(pos); ok {
				cellDTO.DoorKeyhole = dto.Keyhole(keyhole)
			}
		}
	}

	if visibility == model.Visible {
		cellDTO.Actor = t.constructActor(playthrough, m, r, c)
		cellDTO.Item = t.constructItem(playthrough, m, r, c)
	}

	return cellDTO
}

//
//
// --- EFFECTS ---
//
//

func (t *Mapper) constructEffects(effects []*model.Effect) []*dto.Effect {
	effectsDTO := make([]*dto.Effect, 0, len(effects))

	for _, effect := range effects {
		effectsDTO = append(effectsDTO, &dto.Effect{
			Kind:         t.mapEffectType(effect.Kind),
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

func (t *Mapper) constructActor(playthrough *model.Playthrough, m *model.Map, row, col int) *dto.Actor {
	actor := playthrough.GetActor(model.ActorId(m.GetActorGrid()[row][col]))
	return t.mapActorToDTO(actor)
}

func (t *Mapper) mapActorToDTO(actor *model.Actor) *dto.Actor {
	if actor == nil {
		return nil
	}

	effVitalsChange, effAttrsChange := actor.GetEffectsInfluence()

	return &dto.Actor{
		Kind:           t.mapActorType(actor.Kind),
		BaseVitals:     t.convertVitals(actor.Vitals),
		BaseAttrs:      t.convertAttrs(actor.BaseAttrs),
		VitalsChange:   t.convertVitalsChange(effVitalsChange),
		AttrsChange:    t.convertAttrsChange(effAttrsChange),
		Vitals:         t.formatActorVitals(actor.Vitals, effVitalsChange),
		Attrs:          t.formatActorAttrs(actor.BaseAttrs, effAttrsChange),
		Statuses:       t.formatStatuses(actor.Statuses),
		AppliedEffects: t.constructEffects(actor.CollectActiveEffects()),
	}
}

//
//
// --- PLAYER ---
//
//

func (t *Mapper) constructLobbyPlayers(playthrough *model.Playthrough) []dto.LobbyPlayer {
	if playthrough.State != model.LobbyGameState {
		return nil
	}
	players := make([]dto.LobbyPlayer, 0, len(playthrough.PlayersUuid))
	for uuid := range playthrough.PlayersUuid {
		nick := playthrough.PlayersNicknames[uuid]
		players = append(players, dto.LobbyPlayer{
			IsHost:   playthrough.IsHost(uuid),
			Nickname: nick,
		})
	}
	return players
}

func (t *Mapper) constructPlayer(playthrough *model.Playthrough, actor *model.Actor, playerUUID string) *dto.Player {
	if actor == nil {
		return nil
	}

	return &dto.Player{
		Actor:     t.mapActorToDTO(actor),
		HUD:       t.constructHUD(playthrough, actor),
		Inventory: t.constructInventory(actor),
		Equipped:  t.constructEquipped(actor),
		RunStats:  t.constructRunStats(playthrough, actor),
		IsHost:    playthrough.IsHost(playerUUID),
		Row:       actor.Pos.Y,
		Col:       actor.Pos.X,
	}
}

func (t *Mapper) constructHUD(playthrough *model.Playthrough, a *model.Actor) *dto.HUD {
	return &dto.HUD{
		HP:        a.Vitals[model.VitalHP],
		MaxHP:     a.DerivedAttrs[model.AttrMaxHP],
		Strength:  a.DerivedAttrs[model.AttrStrength],
		Dexterity: a.DerivedAttrs[model.AttrDexterity],
		Dungeon:   playthrough.Depth,
		Treasure:  a.Backpack.TreasuresValue,
	}
}

func (t *Mapper) constructInventory(a *model.Actor) map[dto.ItemType][]*dto.Item {
	inventory := make(map[dto.ItemType][]*dto.Item)

	for _, slot := range a.Backpack.Slots {
		for _, item := range slot {
			itemDTO := t.mapItemType(item.Kind)
			inventory[itemDTO] = append(inventory[itemDTO], t.convertItem(item))
		}
	}

	return inventory
}

func (t *Mapper) constructEquipped(a *model.Actor) map[dto.ItemType]*dto.Item {
	equipped := make(map[dto.ItemType]*dto.Item)

	for _, gear := range a.EquippedGear {
		gearDTO := t.mapItemType(gear.Kind)
		equipped[gearDTO] = t.convertItem(gear)
	}

	return equipped
}

// buildLeaderboard aggregates all players' run statistics into a slice sorted
// by TotalTreasure descending so the UI can display a live leaderboard.
func (t *Mapper) buildLeaderboard(play *model.Playthrough) []dto.LeaderboardEntry {
	if len(play.PlayersStats) == 0 {
		return nil
	}

	// Build a reverse map so we can look up UUID → nickname by ActorId.
	actorToUUID := make(map[model.ActorId]string, len(play.PlayersUuid))
	for uuid, id := range play.PlayersUuid {
		actorToUUID[id] = uuid
	}

	entries := make([]dto.LeaderboardEntry, 0, len(play.PlayersStats))
	for id, stats := range play.PlayersStats {
		nick := play.PlayersNicknames[actorToUUID[id]]
		if nick == "" {
			nick = "???"
		}
		entries = append(entries, dto.LeaderboardEntry{
			Nickname:      nick,
			TotalTreasure: stats.TotalTreasure,
			DeepestLevel:  stats.DeepestLevel,
		})
	}

	slices.SortFunc(entries, func(a, b dto.LeaderboardEntry) int {
		return b.TotalTreasure - a.TotalTreasure // descending
	})
	return entries
}

func (t *Mapper) constructRunStats(playthrough *model.Playthrough, a *model.Actor) *dto.RunStats {
	stats, exists := playthrough.PlayersStats[a.Id]
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

func (t *Mapper) constructItem(playthrough *model.Playthrough, m *model.Map, row, col int) *dto.Item {
	item, _ := playthrough.Items[model.ItemId(m.GetItemGrid()[row][col])]
	return t.convertItem(item)
}

func (t *Mapper) convertItem(item *model.Item) *dto.Item {
	if item == nil {
		return nil
	}

	i := &dto.Item{
		Id:           int(item.Id),
		Kind:         t.mapItemType(item.Kind),
		Label:        t.mapItemLabel(item.Label),
		Keyhole:      t.mapItemKeyhole(item.Keyhole),
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

func (t *Mapper) mapTopologyType(tile model.TileType) dto.TopologyType {
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

func (t *Mapper) mapActorType(actorType model.ActorType) dto.ActorKind {
	return dto.ActorKind(actorType)
}

func (t *Mapper) mapItemType(itemType model.ItemType) dto.ItemType {
	return dto.ItemType(itemType)
}

func (t *Mapper) mapItemLabel(itemLabel model.ItemLabel) dto.ItemLabel {
	return dto.ItemLabel(itemLabel)
}

func (t *Mapper) mapItemKeyhole(itemKeyhole model.Keyhole) dto.Keyhole {
	switch itemKeyhole {
	case model.KeyholeNone:
		return dto.KeyholeNone
	default:
		return dto.Keyhole(itemKeyhole)
	}
}

func (t *Mapper) mapVitalType(vital model.VitalType) dto.VitalType {
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

func (t *Mapper) mapAttrType(attr model.AttrType) dto.AttrType {
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

func (t *Mapper) mapEffectType(effectType model.EffectType) dto.EffectKind {
	return dto.EffectKind(effectType)
}

func (t *Mapper) mapGameState(playthrough *model.Playthrough) dto.GameState {
	switch playthrough.State {
	case model.LobbyGameState:
		return dto.StateLobby
	case model.PlayingGameState:
		return dto.StatePlaying
	case model.GameOverGameState:
		return dto.StateGameover
	default:
		log.Printf("[WARNING] Unknown session state detected: %v\n", playthrough.State)
		return dto.StateUnknown
	}
}

func (t *Mapper) mapVisibilityState(vs model.VisibilityState) dto.VisibilityState {
	switch vs {
	case model.Unexplored:
		return dto.VisibilityUnexplored
	case model.Visible:
		return dto.VisibilityVisible
	case model.Explored:
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

func (t *Mapper) convertVitals(vitals map[model.VitalType]int) map[dto.VitalType]int {
	vitalsDTO := make(map[dto.VitalType]int, len(vitals))

	for vital, val := range vitals {
		vitalsDTO[t.mapVitalType(vital)] = val
	}

	return vitalsDTO
}

func (t *Mapper) convertAttrs(attrs map[model.AttrType]int) map[dto.AttrType]int {
	attrsDTO := make(map[dto.AttrType]int, len(attrs))

	for attr, val := range attrs {
		attrsDTO[t.mapAttrType(attr)] = val
	}

	return attrsDTO
}

func (t *Mapper) convertVitalsChange(vitalsChange map[model.VitalType][]int) map[dto.VitalType][]int {
	vitalsChangeDTO := make(map[dto.VitalType][]int, len(vitalsChange))

	for vital, changes := range vitalsChange {
		cloned := make([]int, len(changes))
		copy(cloned, changes)
		vitalsChangeDTO[t.mapVitalType(vital)] = cloned
	}

	return vitalsChangeDTO
}

func (t *Mapper) convertAttrsChange(attrsChange map[model.AttrType][]int) map[dto.AttrType][]int {
	attrsChangeDTO := make(map[dto.AttrType][]int, len(attrsChange))

	for attr, changes := range attrsChange {
		cloned := make([]int, len(changes))
		copy(cloned, changes)
		attrsChangeDTO[t.mapAttrType(attr)] = cloned
	}

	return attrsChangeDTO
}

//
// -- STRING CONVERTERS --
//

func (t *Mapper) convertEvents(events []model.Event) []*dto.Event {
	eventsDTO := make([]*dto.Event, 0, len(events))

	for _, event := range events {
		eventsDTO = append(eventsDTO, t.convertEvent(event))
	}

	return eventsDTO
}

func (t *Mapper) convertEvent(event model.Event) *dto.Event {
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

func (t *Mapper) formatAttackEvent(event *service.AttackEvent) string {
	if event.Attacker == nil || event.Defender == nil {
		return ""
	}

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

func (t *Mapper) formatMoveEvent(event *service.MoveEvent) string {
	mover := event.Mover.Actor

	switch event.Outcome {
	case service.MoveOutcomeSuccess:
		return fmt.Sprintf("%v#%v moves to (%d,%d)", t.formatActorType(mover.Kind), mover.Id, mover.Pos.X, mover.Pos.Y)
	case service.MoveOutcomeNoStamina:
		return fmt.Sprintf("%v#%v is too exhausted to move", t.formatActorType(mover.Kind), mover.Id)
	case service.MoveOutcomeCantMove:
		return fmt.Sprintf("%v#%v cannot move there", t.formatActorType(mover.Kind), mover.Id)
	case service.MoveOutcomeNoSpace:
		return fmt.Sprintf("%v#%v has no space to move", t.formatActorType(mover.Kind), mover.Id)
	default:
		return ""
	}
}

func (t *Mapper) formatItemPickupEvent(event *service.ItemPickupEvent) string {
	actor := event.Actor.Actor

	switch event.Outcome {
	case service.PickupOutcomeSuccess:
		if event.PickupItem == nil {
			return fmt.Sprintf("%v#%v picks up something", t.formatActorType(actor.Kind), actor.Id)
		}
		return fmt.Sprintf("%v#%v picks up %s", t.formatActorType(actor.Kind), actor.Id, t.formatItemLabel(event.PickupItem.Label))
	case service.PickupOutcomeBackpackNoFreeSpace:
		return fmt.Sprintf("%v#%v cannot pick up item: backpack is full", t.formatActorType(actor.Kind), actor.Id)
	default:
		return ""
	}
}

func (t *Mapper) formatItemUsageEvent(event *service.ItemUsageEvent) string {
	user := event.User.Actor

	switch event.Outcome {
	case service.ItemUsageOutcomeSuccess:
		if event.RetrievedItem != nil {
			return fmt.Sprintf("%v#%v uses %s", t.formatActorType(user.Kind), user.Id, t.formatItemLabel(event.RetrievedItem.Label))
		}
		return fmt.Sprintf("%v#%v uses an item", t.formatActorType(user.Kind), user.Id)
	case service.ItemUsageOutcomeNotFound:
		return fmt.Sprintf("%v#%v tries to use an item but cannot find it", t.formatActorType(user.Kind), user.Id)
	case service.ItemUsageOutcomeCantUnequip:
		return fmt.Sprintf("%v#%v cannot unequip item", t.formatActorType(user.Kind), user.Id)
	case service.ItemUsageOutcomeNoStamina:
		return fmt.Sprintf("%v#%v is too exhausted to use item", t.formatActorType(user.Kind), user.Id)
	default:
		return ""
	}
}

// formatItemLabel converts a model.ItemLabel constant to a human-readable display name.
func (t *Mapper) formatItemLabel(label model.ItemLabel) string {
	switch label {
	case model.ItemLabelDefaultFood:
		return "Ration"
	case model.ItemLabelDexterityElixir:
		return "Dexterity Elixir"
	case model.ItemLabelStrengthElixir:
		return "Strength Elixir"
	case model.ItemLabelMaxHpElixir:
		return "Max HP Elixir"
	case model.ItemLabelDexterityScroll:
		return "Dexterity Scroll"
	case model.ItemLabelStrengthScroll:
		return "Strength Scroll"
	case model.ItemLabelMaxHpScroll:
		return "Max HP Scroll"
	case model.ItemLabelDefaultWeapon:
		return "Sword"
	case model.ItemLabelDefaultTreasure:
		return "Treasure"
	case model.ItemLabelKey:
		return "Key"
	default:
		return string(label)
	}
}

func (t *Mapper) formatActorType(actorType model.ActorType) string {
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

func (t *Mapper) formatActorVitals(baseVitals map[model.VitalType]int, vitalsChange map[model.VitalType][]int) map[string][]string {
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

func (t *Mapper) formatActorAttrs(baseAttrs map[model.AttrType]int, attrsChange map[model.AttrType][]int) map[string][]string {
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

func (t *Mapper) formatAttributes(attrsChange map[model.AttrType]int) []string {
	stringAttrs := make([]string, 0, len(attrsChange))

	attrs := conv.MapToKV(attrsChange)
	slices.SortFunc(attrs, func(a, b conv.KV[model.AttrType, int]) int { return cmp.Compare(a.Key, b.Key) })

	for _, attr := range attrs {
		if attr.Val == 0 {
			continue
		}

		str := fmt.Sprintf("%+d %s", attr.Val, t.formatAttrType(attr.Key))
		stringAttrs = append(stringAttrs, str)
	}

	return stringAttrs
}

func (t *Mapper) formatStatuses(statusesChange map[model.StatusType]int) []string {
	var stringStatuses []string

	for status, val := range statusesChange {
		if val <= 0 {
			continue
		}
		stringStatuses = append(stringStatuses, t.formatStatus(status))
	}

	return stringStatuses
}

func (t *Mapper) formatAttrType(attr model.AttrType) string {
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

func (t *Mapper) formatVitalType(vital model.VitalType) string {
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

func (t *Mapper) formatEffectType(effectType model.EffectType) string {
	return string(effectType)
}

func (t *Mapper) formatStatus(status model.StatusType) string {
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
