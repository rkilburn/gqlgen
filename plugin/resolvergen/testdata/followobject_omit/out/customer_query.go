package customresolver

import (
	"context"
	"fmt"
)

// Customer is the resolver for the customer field.
func (r *queryCustomResolverType) Customer(ctx context.Context) (*Customer, error) {
	panic(fmt.Errorf("not implemented: Customer - customer"))
}
