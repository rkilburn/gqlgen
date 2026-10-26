package customresolver

import (
	"context"
	"fmt"
)

// Name is the resolver for the name field.
func (r *supplierCustomResolverType) Name(ctx context.Context, obj *Supplier) (string, error) {
	panic(fmt.Errorf("not implemented: Name - name"))
}
