package network

import (
	"github.com/unclestep/Rogue/internal/presentation/tui"
)

type App struct {
	server *Server
	client *Client
}

func NewApp(server *Server, client *Client) *App {
	return &App{
		server: server,
		client: client,
	}
}

func (a *App) Run() {
	a.server.Listen()

	toClient := a.server.Subscribe(a.client.Uuid)
	fromClient := a.server.CommandChan()

	// The menu is responsible for sending the initial ActionJoin command;
	// the app does not auto-join so the player sees the main menu first.
	ui := tui.NewUIModel(fromClient, toClient, a.client.Uuid, a.client.LastPlaythroughId)
	a.client.LastPlaythroughId = ui.Run()

	a.server.Stop()
	a.client.Save()
}
