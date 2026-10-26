package customresolver

import "context"

type QueryResolver interface {
	Greeting(ctx context.Context) (string, error)
}

type MutationResolver interface {
	Shout(ctx context.Context) (string, error)
}
