package model

type Event interface {
	Perform(ctx *SessionContext)
}
