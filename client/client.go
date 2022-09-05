package client

type Protocol interface {
	NewClient() Client
}

type Client interface {
	SetOptions(any)
	SetPoolOptions(any)
	Call(string, any, any, bool) error
	BatchAppend(string, any, any, bool) *error
	BatchCall() error
}

func NewClient[T Protocol](p T) Client {
	return p.NewClient()
}
