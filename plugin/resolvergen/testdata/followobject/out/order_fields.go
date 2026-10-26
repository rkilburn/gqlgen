package customresolver

import (
	"context"
	"fmt"
)

// Total is the resolver for the total field.
func (r *orderCustomResolverType) Total(ctx context.Context, obj *Order) (int, error) {
	panic(fmt.Errorf("not implemented: Total - total"))
}

// Items is the resolver for the items field.
func (r *orderCustomResolverType) Items(ctx context.Context, obj *Order) ([]*OrderItem, error) {
	panic(fmt.Errorf("not implemented: Items - items"))
}
