package customresolver

import "context"

type Customer struct{}

type QueryResolver interface {
	Customer(ctx context.Context) (*Customer, error)
}

type MutationResolver interface {
	CreateOrder(ctx context.Context) (*Customer, error)
	CancelOrder(ctx context.Context) (*Customer, error)
}

type SubscriptionResolver interface {
	OrderSent(ctx context.Context) (<-chan *Customer, error)
}

type CustomerResolver interface {
	Name(ctx context.Context, obj *Customer) (string, error)
}
