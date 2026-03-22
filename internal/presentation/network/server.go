package network

import (
	"context"

	"github.com/unclestep/Rogue/internal/application"
	"github.com/unclestep/Rogue/internal/dto"
)

type Server struct {
	gameLoop *application.GameLoop
	stop     context.CancelFunc
}

func NewServer(loop *application.GameLoop) *Server {
	return &Server{gameLoop: loop}
}

func (s *Server) Listen() {
	ctx, cancel := context.WithCancel(context.Background())
	s.stop = cancel
	go s.gameLoop.Run(ctx)
}

// Stop signals the game loop to shut down gracefully.
func (s *Server) Stop() {
	if s.stop != nil {
		s.stop()
	}
}

func (s *Server) Subscribe(playerUUID string) <-chan dto.GameView {
	return s.gameLoop.AddSubscriber(playerUUID)
}

func (s *Server) CommandChan() chan<- dto.Command {
	return s.gameLoop.GetNotificationChan()
}
