package observer

type Observer[T any] interface {
	OnNotify(data T)
}

type Subject[T any] struct {
	Observers []Observer[T]
}

func (s *Subject[T]) Attach(o Observer[T]) {
	s.Observers = append(s.Observers, o)
}

func (s *Subject[T]) Notify(data T) {
	for _, obs := range s.Observers {
		obs.OnNotify(data)
	}
}
