package customresolver

import (
	"context"
	"fmt"
)

// Name is the resolver for the name field.
func (r *customerCustomResolverType) Name(ctx context.Context, obj *Customer) (string, error) {
	panic(fmt.Errorf("not implemented: Name - name"))
}

// Orders is the resolver for the orders field.
func (r *customerCustomResolverType) Orders(ctx context.Context, obj *Customer) ([]*Order, error) {
	panic(fmt.Errorf("not implemented: Orders - orders"))
}

// Email is the resolver for the email field.
func (r *customerCustomResolverType) Email(ctx context.Context, obj *Customer) (string, error) {
	panic(fmt.Errorf("not implemented: Email - email"))
}
