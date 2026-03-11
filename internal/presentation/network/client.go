// Client file
package app

import (
	"encoding/json"
	"github.com/google/uuid"
	"github.com/unclestep/Rogue/internal/dto"
	"github.com/unclestep/Rogue/internal/ui/tui"
	"os"
	"path/filepath"
)

const clientSaves = "client_saves/client.json" // Temporary solution

type Client struct {
	Ui   *tui.UIModel
	Uuid string
}

//
//
// --- CONSTRUCTORS ---
//
//

func NewClient(fromClient chan<- dto.Command, toClient <-chan dto.WorldInfo) *Client {
	return &Client{
		Ui:   tui.NewUIModel(fromClient, toClient),
		Uuid: getUuid(clientSaves),
	}
}

//
//
// --- START CLIENT METHOD ---
//
//

func (c *Client) Listen() {
	c.Ui.Run()
}

//
//
// --- SAVE&LOAD METHODS ---
//
//

func (c *Client) Save() {
	data, _ := json.MarshalIndent(map[string]string{"uuid": c.Uuid}, "", "\t")
	saveFile(clientSaves, data)
}

func saveFile(path string, data []byte) {
	dir := filepath.Dir(path)
	os.MkdirAll(dir, 0755)
	os.WriteFile(path, data, 0644)
}

// getUuid - loads uuid from file. If file does not exist, creates new file and uuid
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
	saveFile(path, newData)

	return newUuid
}
