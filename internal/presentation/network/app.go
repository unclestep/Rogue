// Temporary application which unites client and server in one executable
package app

import (
	"github.com/google/uuid"
	"github.com/unclestep/Rogue/internal/dto"
)

type App struct {
	ToClient   chan dto.WorldInfo
	FromClient chan dto.Command
	Client     *Client
	Server     *Server
}

func NewApp() *App {
	toClient := make(chan dto.WorldInfo)
	fromClient := make(chan dto.Command)

	return &App{
		ToClient:   toClient,
		FromClient: fromClient,
		Client:     NewClient(fromClient, toClient),
		Server:     NewServer(fromClient, toClient),
	}
}

func (a *App) Run() {
	go a.Server.Listen()

	a.FromClient <- dto.Command{
		CommandType: dto.CommandJoin,
		PlayerUUID:  a.Client.Uuid,
	}

	a.Client.Ui.Run()

	a.SaveGame()
}

func (a *App) SaveGame() {
	a.Client.Save()
	a.Server.Save()
}
