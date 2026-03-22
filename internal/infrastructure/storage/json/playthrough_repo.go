package json

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/unclestep/Rogue/internal/domain/model"
	jsonDto "github.com/unclestep/Rogue/internal/infrastructure/storage/json/dto"
	"github.com/unclestep/Rogue/internal/infrastructure/storage/json/mapper"
)

type JsonPlaythroughRepo struct {
	folder string
}

func NewJsonPlaythroughRepo(folder string) *JsonPlaythroughRepo {
	os.MkdirAll(folder, 0o755)
	return &JsonPlaythroughRepo{folder: folder}
}

func (r *JsonPlaythroughRepo) path(id model.PlaythroughId) string {
	return fmt.Sprintf("%s/playthrough_%s.json", r.folder, string(id))
}

func (r *JsonPlaythroughRepo) Get(id model.PlaythroughId) (*model.Playthrough, error) {
	data, err := os.ReadFile(r.path(id))
	if err != nil {
		return nil, fmt.Errorf("playthrough %s not found: %w", id, err)
	}

	var d jsonDto.Playthrough
	if err = json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("playthrough %s unmarshal error: %w", id, err)
	}

	return mapper.PlaythroughFromDTO(&d), nil
}

func (r *JsonPlaythroughRepo) Create(rulesId model.RulesId, seed int64) *model.Playthrough {
	id := model.PlaythroughId(uuid.New().String())
	p := model.NewPlaythrough(id, rulesId, seed)
	_ = r.Save(p)
	return p
}

func (r *JsonPlaythroughRepo) Save(p *model.Playthrough) error {
	d := mapper.PlaythroughToDTO(p)
	data, err := json.MarshalIndent(d, "", "\t")
	if err != nil {
		return err
	}
	return os.WriteFile(r.path(p.PlaythroughId), data, 0o644)
}

func (r *JsonPlaythroughRepo) Delete(id model.PlaythroughId) {
	os.Remove(r.path(id))
}
