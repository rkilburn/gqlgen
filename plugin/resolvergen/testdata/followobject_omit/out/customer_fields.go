package customresolver

import (
	"context"
	"fmt"
)

// Name is the resolver for the name field.
func (r *customerCustomResolverType) Name(ctx context.Context, obj *Customer) (string, error) {
	panic(fmt.Errorf("not implemented: Name - name"))
}
