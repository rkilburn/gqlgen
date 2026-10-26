package customresolver

import "context"

type QueryResolver interface {
	Greeting(ctx context.Context) (string, error)
}
