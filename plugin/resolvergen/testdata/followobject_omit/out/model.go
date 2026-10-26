package customresolver

import "context"

type Customer struct{}

type QueryResolver interface {
	Customer(ctx context.Context) (*Customer, error)
}

type CustomerResolver interface {
	Name(ctx context.Context, obj *Customer) (string, error)
}
