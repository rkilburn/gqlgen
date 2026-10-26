package resolvergen

import (
	"path/filepath"
	"strings"
	"unicode"

	"github.com/99designs/gqlgen/codegen"
)

// operationFilenameTemplate returns the template that gives each field of o a file of its own,
// or "" when o is not the Mutation or Subscription type or no template is configured for it.
func operationFilenameTemplate(data *codegen.Data, o *codegen.Object) string {
	switch o {
	case nil:
		return ""
	case data.MutationRoot:
		return data.Config.Resolver.MutationFilenameTemplate
	case data.SubscriptionRoot:
		return data.Config.Resolver.SubscriptionFilenameTemplate
	}
	return ""
}

// gqlToOperationResolverName fills in a mutation or subscription filename template: {name} is the
// schema file's base name and {field} the snake_case name of the field, e.g. "{field}_mutation.go"
// gives create_order_mutation.go.
func gqlToOperationResolverName(base, gqlname, tmpl, fieldName string) string {
	gqlname = filepath.Base(gqlname)
	name := strings.TrimSuffix(gqlname, filepath.Ext(gqlname))
	filename := strings.ReplaceAll(tmpl, "{name}", name)
	filename = strings.ReplaceAll(filename, "{field}", snakeCase(fieldName))
	return filepath.Join(base, goSafeFilename(filename))
}

// goSafeFilename keeps a name built from a field name from being skipped or misread by the go
// tool: files starting with "_" or "." are ignored, and files ending in "_test.go" are only
// compiled into tests.
func goSafeFilename(filename string) string {
	dir, name := filepath.Split(filename)
	name = strings.TrimLeft(name, "_.")
	if strings.HasSuffix(name, "_test.go") {
		name = strings.TrimSuffix(name, ".go") + "_.go"
	}
	return dir + name
}

// snakeCase turns a GraphQL name such as "createOrder" or "uploadHTTPLog" into "create_order" or
// "upload_http_log".
func snakeCase(s string) string {
	runes := []rune(s)
	var sb strings.Builder
	for i, r := range runes {
		if unicode.IsUpper(r) {
			if i > 0 && (unicode.IsLower(runes[i-1]) || unicode.IsDigit(runes[i-1]) ||
				(unicode.IsUpper(runes[i-1]) && i+1 < len(runes) && unicode.IsLower(runes[i+1]))) {
				sb.WriteByte('_')
			}
			r = unicode.ToLower(r)
		}
		sb.WriteRune(r)
	}
	return sb.String()
}
