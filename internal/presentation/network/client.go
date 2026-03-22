package network

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

const clientSavePath = "saves/client.json"

type Client struct {
	Uuid              string
	LastPlaythroughId string
}

func NewClient() *Client {
	return &Client{
		Uuid:              getUuid(clientSavePath),
		LastPlaythroughId: getLastPlaythroughId(clientSavePath),
	}
}

func (c *Client) Save() {
	data, _ := json.MarshalIndent(map[string]string{
		"uuid":                c.Uuid,
		"last_playthrough_id": c.LastPlaythroughId,
	}, "", "\t")
	writeFile(clientSavePath, data)
}

func writeFile(path string, data []byte) {
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, data, 0o644)
}

func getUuid(path string) string {
	data, err := os.ReadFile(path)
	if err == nil {
		var profile struct {
			Uuid string `json:"uuid"`
		}
		if err := json.Unmarshal(data, &profile); err == nil && profile.Uuid != "" {
			return profile.Uuid
		}
	}

	newUuid := uuid.New().String()
	newData, _ := json.MarshalIndent(map[string]string{"uuid": newUuid}, "", "\t")
	writeFile(path, newData)
	return newUuid
}

func getLastPlaythroughId(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var profile struct {
		LastPlaythroughId string `json:"last_playthrough_id"`
	}
	if err := json.Unmarshal(data, &profile); err != nil {
		return ""
	}
	return profile.LastPlaythroughId
}
