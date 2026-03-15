package json

import (
	"encoding/json"
	"fmt"
	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/infrastructure/storage/json/dto"
	"os"
)

type JsonRulesRepo struct {
	folder string
}

func (r *JsonRulesRepo) Get(id model.RulesId) (*model.GameRules, error) {
	data, err := os.ReadFile(fmt.Sprintf("%s/rules_%d.json", r.folder, id))

	if err != nil {
		if id == model.DefaultRulesId {
			defaults := model.NewDefaultGameRules()
			// defaultsDto := mapper.RulesToDTO(default)
			data := json.Marshal(defaultsDto)
			os.WriteFile(fmt.Sprintf("%s/rules_%d.json", r.folder, 0))
			return model.NewDefaultGameRules(), nil
		}
		return r.Get(model.DefaultRulesId)
	}

	var dto dto.GameRulesDTO
	err = json.Unmarshal(data, &dto)

	if err != nil {
		if id == model.DefaultRulesId {
			defaults := model.NewDefaultGameRules()
			// defaultsDto := mapper.RulesToDTO(default)
			data := json.Marshal(defaultsDto)
			os.WriteFile(fmt.Sprintf("%s/rules_%d.json", r.folder, 0))
			return model.NewDefaultGameRules(), nil
		}
		return r.Get(model.DefaultRulesId)
	}

	// TODO:
	return mapper.RulesFromDTO(dto), nil
}

func (r *JsonRulesRepo) Save()
