package json

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/unclestep/Rogue/internal/domain/model"
	jsonDto "github.com/unclestep/Rogue/internal/infrastructure/storage/json/dto"
	"github.com/unclestep/Rogue/internal/infrastructure/storage/json/mapper"
)

type JsonRulesRepo struct {
	folder string
}

func (r *JsonRulesRepo) Get(id model.RulesId) (*model.GameRules, error) {
	data, err := os.ReadFile(fmt.Sprintf("%s/rules_%d.json", r.folder, id))
	if err != nil {
		if id == model.DefaultRulesId {
			return r.saveAndReturnDefaults()
		}
		return r.Get(model.DefaultRulesId)
	}

	var d jsonDto.GameRulesDTO
	if err = json.Unmarshal(data, &d); err != nil {
		if id == model.DefaultRulesId {
			return r.saveAndReturnDefaults()
		}
		return r.Get(model.DefaultRulesId)
	}

	return mapper.GameRulesFromDTO(&d), nil
}

func (r *JsonRulesRepo) saveAndReturnDefaults() (*model.GameRules, error) {
	defaults := model.NewDefaultGameRules()
	defaultsDto := mapper.GameRulesToDTO(defaults)

	data, err := json.Marshal(defaultsDto)
	if err != nil {
		return defaults, nil
	}

	_ = os.WriteFile(fmt.Sprintf("%s/rules_%d.json", r.folder, model.DefaultRulesId), data, 0o644)
	return defaults, nil
}

func (r *JsonRulesRepo) Save(rules *model.GameRules) error {
	d := mapper.GameRulesToDTO(rules)
	data, err := json.Marshal(d)
	if err != nil {
		return err
	}
	return os.WriteFile(fmt.Sprintf("%s/rules_%d.json", r.folder, rules.Id), data, 0o644)
}
