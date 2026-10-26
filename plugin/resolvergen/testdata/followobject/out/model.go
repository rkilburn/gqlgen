package customresolver

import "context"

type Customer struct{}

type Order struct{}

type OrderItem struct{}

type Product struct{}

type Supplier struct{}

type QueryResolver interface {
	Customer(ctx context.Context) (*Customer, error)
	Order(ctx context.Context) (*Order, error)
}

type MutationResolver interface {
	CreateOrder(ctx context.Context) (*Order, error)
}

type CustomerResolver interface {
	Name(ctx context.Context, obj *Customer) (string, error)
	Orders(ctx context.Context, obj *Customer) ([]*Order, error)
	Email(ctx context.Context, obj *Customer) (string, error)
}

type OrderResolver interface {
	Total(ctx context.Context, obj *Order) (int, error)
	Items(ctx context.Context, obj *Order) ([]*OrderItem, error)
}

type OrderItemResolver interface {
	Quantity(ctx context.Context, obj *OrderItem) (int, error)
	Product(ctx context.Context, obj *OrderItem) (*Product, error)
}

type ProductResolver interface {
	Name(ctx context.Context, obj *Product) (string, error)
	Supplier(ctx context.Context, obj *Product) (*Supplier, error)
}

type SupplierResolver interface {
	Name(ctx context.Context, obj *Supplier) (string, error)
}
