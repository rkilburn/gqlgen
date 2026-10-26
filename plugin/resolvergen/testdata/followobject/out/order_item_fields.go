package customresolver

import (
	"context"
	"fmt"
)

// Quantity is the resolver for the quantity field.
func (r *orderItemCustomResolverType) Quantity(ctx context.Context, obj *OrderItem) (int, error) {
	panic(fmt.Errorf("not implemented: Quantity - quantity"))
}

// Product is the resolver for the product field.
func (r *orderItemCustomResolverType) Product(ctx context.Context, obj *OrderItem) (*Product, error) {
	panic(fmt.Errorf("not implemented: Product - product"))
}
