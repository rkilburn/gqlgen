package customresolver

import (
	"context"
	"fmt"
)

// Order is the resolver for the order field.
func (r *queryCustomResolverType) Order(ctx context.Context) (*Order, error) {
	panic(fmt.Errorf("not implemented: Order - order"))
}
