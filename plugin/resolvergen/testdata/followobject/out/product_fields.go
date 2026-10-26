package customresolver

import (
	"context"
	"fmt"
)

// Name is the resolver for the name field.
func (r *productCustomResolverType) Name(ctx context.Context, obj *Product) (string, error) {
	panic(fmt.Errorf("not implemented: Name - name"))
}

// Supplier is the resolver for the supplier field.
func (r *productCustomResolverType) Supplier(ctx context.Context, obj *Product) (*Supplier, error) {
	panic(fmt.Errorf("not implemented: Supplier - supplier"))
}
