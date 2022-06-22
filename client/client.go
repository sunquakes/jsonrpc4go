package client

type Protocol interface {
	NewClient() (Client, error)
}

type Client interface {
	SetOptions(any)
	Call(string, any, any, bool) error
	BatchAppend(string, any, any, bool) *error
	BatchCall() error
}

func NewClinet[T Protocol](p T) (Client, error) {
	return p.NewClient()
}
