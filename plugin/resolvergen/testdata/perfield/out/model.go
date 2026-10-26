package customresolver

import "context"

type Customer struct{}

type Order struct{}

type QueryResolver interface {
	Customer(ctx context.Context) (*Customer, error)
	Order(ctx context.Context) (*Order, error)
}

type CustomerResolver interface {
	Name(ctx context.Context, obj *Customer) (string, error)
	Orders(ctx context.Context, obj *Customer) ([]*Order, error)
	Email(ctx context.Context, obj *Customer) (string, error)
}

type OrderResolver interface {
	Total(ctx context.Context, obj *Order) (int, error)
}
