package customresolver

import (
	"context"
	"fmt"
)

// CreateOrder is the resolver for the createOrder field.
func (r *mutationCustomResolverType) CreateOrder(ctx context.Context) (*Order, error) {
	panic(fmt.Errorf("not implemented: CreateOrder - createOrder"))
}
