package resolvergen

import (
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2/ast"
	"golang.org/x/tools/go/packages"

	"github.com/99designs/gqlgen/codegen"
	"github.com/99designs/gqlgen/codegen/config"
)

func TestResolverBuild_ResolverTypeDeclarations(t *testing.T) {
	object := func(name string) *codegen.Object {
		return &codegen.Object{Definition: &ast.Definition{Name: name}}
	}

	tests := map[string]struct {
		objects               []*codegen.Object
		resolverType          string
		omitResolverEmbedding bool
		want                  string
	}{
		"no objects yields no declaration": {
			objects:      nil,
			resolverType: "Resolver",
			want:         "",
		},
		"single object is a lone declaration": {
			objects:      []*codegen.Object{object("Query")},
			resolverType: "Resolver",
			want:         "type queryResolver struct{ *Resolver }",
		},
		"multiple objects are grouped and column-aligned": {
			objects:      []*codegen.Object{object("Query"), object("Subscription")},
			resolverType: "Resolver",
			want: "type (\n" +
				"\tqueryResolver        struct{ *Resolver }\n" +
				"\tsubscriptionResolver struct{ *Resolver }\n" +
				")",
		},
		"alignment follows the custom resolver type name": {
			objects:      []*codegen.Object{object("Query"), object("Mutation")},
			resolverType: "rootResolver",
			want: "type (\n" +
				"\tqueryRootResolver    struct{ *rootResolver }\n" +
				"\tmutationRootResolver struct{ *rootResolver }\n" +
				")",
		},
		"omitting the embedding names the field on a lone declaration": {
			objects:               []*codegen.Object{object("Query")},
			resolverType:          "Resolver",
			omitResolverEmbedding: true,
			want:                  "type queryResolver struct{ r *Resolver }",
		},
		"omitting the embedding names the field on every grouped declaration": {
			objects:               []*codegen.Object{object("Query"), object("Subscription")},
			resolverType:          "Resolver",
			omitResolverEmbedding: true,
			want: "type (\n" +
				"\tqueryResolver        struct{ r *Resolver }\n" +
				"\tsubscriptionResolver struct{ r *Resolver }\n" +
				")",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			b := &ResolverBuild{
				File:                  &File{Objects: tt.objects},
				ResolverType:          tt.resolverType,
				OmitResolverEmbedding: tt.omitResolverEmbedding,
			}
			require.Equal(t, tt.want, b.ResolverTypeDeclarations())
		})
	}
}

func TestLayoutSingleFile(t *testing.T) {
	_ = syscall.Unlink("testdata/singlefile/out/resolver.go")

	cfg, err := config.LoadConfig("testdata/singlefile/gqlgen.yml")
	require.NoError(t, err)
	p := Plugin{}

	require.NoError(t, cfg.Init())

	data, err := codegen.BuildData(cfg)
	require.NoError(t, err)

	require.NoError(t, p.GenerateCode(data))
	assertNoErrors(t, "github.com/99designs/gqlgen/plugin/resolvergen/testdata/singlefile/out")
}

func TestLayoutSingleFileWithEnableRewrite(t *testing.T) {
	// Ensure the resolver file exists before running the test
	resolverFilePath := "testdata/singlefile_preserve/out/resolver.go"
	_, err := os.Stat(resolverFilePath)
	if os.IsNotExist(err) {
		t.Fatalf("Expected resolver file does not exist: %s", resolverFilePath)
	}
	require.NoError(t, err)

	cfg, err := config.LoadConfig("testdata/singlefile_preserve/gqlgen.yml")
	require.NoError(t, err)
	p := Plugin{}

	require.NoError(t, cfg.Init())

	data, err := codegen.BuildData(cfg)
	require.NoError(t, err)

	require.NoError(t, p.GenerateCode(data))
	assertNoErrors(
		t,
		"github.com/99designs/gqlgen/plugin/resolvergen/testdata/singlefile_preserve/out",
	)
}

func TestLayoutFollowSchema(t *testing.T) {
	testFollowSchemaPersistence(t, "testdata/followschema")

	resolverFilePath := "testdata/followschema/out/schema.resolvers.go"
	overWriteFile(t, resolverFilePath+".txt", resolverFilePath)

	b, err := os.ReadFile(resolverFilePath)
	require.NoError(t, err)
	source := string(b)

	require.Contains(t, source, "(_ *customresolver.Resolver, err error)")
	require.Contains(t, source, "// Named return values are supported.")
	require.Contains(t, source, "// CustomerResolverType.Name implementation")
	require.Contains(t, source, "// AUserHelperFunction implementation")
}

func TestLayoutFollowSchemaWithCustomFilename(t *testing.T) {
	testFollowSchemaPersistence(t, "testdata/filetemplate")

	resolverFilePath := "testdata/filetemplate/out/schema.custom.go"
	overWriteFile(t, resolverFilePath+".txt", resolverFilePath)
	b, err := os.ReadFile(resolverFilePath)
	require.NoError(t, err)
	source := string(b)

	require.Contains(t, source, "// CustomerResolverType.Resolver implementation")
	require.Contains(t, source, "// CustomerResolverType.Name implementation")
	require.Contains(t, source, "// AUserHelperFunction implementation")
}

func TestLayoutFollowObject(t *testing.T) {
	testFollowSchemaPersistence(t, "testdata/followobject")
	const dir = "testdata/followobject/out/"

	// Root-type fields get a file each; the fields of other objects share their object's file,
	// however deeply nested: Query.order -> Order.items -> OrderItem.product -> Product.supplier.
	for file, wants := range map[string][]string{
		"customer_query.go":        {") Customer(ctx"},
		"order_query.go":           {") Order(ctx"},
		"create_order_mutation.go": {") CreateOrder(ctx"},
		"customer_fields.go":       {") Name(ctx", ") Orders(ctx", ") Email(ctx"},
		"order_fields.go":          {") Total(ctx", ") Items(ctx"},
		"order_item_fields.go":     {") Quantity(ctx", ") Product(ctx"},
		"product_fields.go":        {") Name(ctx", ") Supplier(ctx"},
		"supplier_fields.go":       {") Name(ctx"},
	} {
		b, err := os.ReadFile(dir + file)
		require.NoError(t, err, file)
		for _, want := range wants {
			require.Contains(t, string(b), want, file)
		}
		// The resolver types live in one file, not beside the methods.
		require.NotContains(t, string(b), "struct{", file)
	}

	// Every object's type and accessor, root types included, is in the one types file, which
	// is marked as generated and so has no implementations to keep.
	b, err := os.ReadFile(dir + "resolver_generated.go")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(string(b),
		"// Code generated by github.com/99designs/gqlgen, DO NOT EDIT.\n\npackage "))
	require.NotContains(t, string(b), "resolver implementations")
	for _, want := range []string{
		") Query() QueryResolver", ") Mutation() MutationResolver", ") Customer() CustomerResolver",
		") Order() OrderResolver", ") OrderItem() OrderItemResolver", ") Product() ProductResolver",
		") Supplier() SupplierResolver", "queryCustomResolverType", "supplierCustomResolverType",
	} {
		require.Contains(t, string(b), want)
	}
}

func TestLayoutFollowObjectMovesImplementations(t *testing.T) {
	const dir = "testdata/followobject_move/out/"
	// Start from another layout: the implementation lives in a file that is the only place
	// importing strings.
	_ = syscall.Unlink(dir + "query.go")
	overWriteFile(t, dir+"legacy.go.txt", dir+"legacy.go")

	cfg, err := config.LoadConfig("testdata/followobject_move/gqlgen.yml")
	require.NoError(t, err)
	require.NoError(t, cfg.Init())
	data, err := codegen.BuildData(cfg)
	require.NoError(t, err)
	require.NoError(t, (&Plugin{}).GenerateCode(data))

	b, err := os.ReadFile(dir + "greeting_query.go")
	require.NoError(t, err)
	require.Contains(t, string(b), `"strings"`)
	require.Contains(t, string(b), `strings.ToUpper("hello")`)

	// The old file is now redundant; once it is gone the package must compile.
	require.NoError(t, syscall.Unlink(dir+"legacy.go"))
	assertNoErrors(t, "github.com/99designs/gqlgen/plugin/resolvergen/testdata/followobject_move/out")
}

func TestLayoutFollowObjectTypesFileKeepsHeaderWhenNoticeOmitted(t *testing.T) {
	testFollowSchemaPersistence(t, "testdata/followobject_omit")
	const dir = "testdata/followobject_omit/out/"

	// The notice is dropped from files people edit but not from the file that is all generated.
	b, err := os.ReadFile(dir + "resolver_generated.go")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(string(b),
		"// Code generated by github.com/99designs/gqlgen, DO NOT EDIT.\n\npackage "))
	b, err = os.ReadFile(dir + "customer_fields.go")
	require.NoError(t, err)
	require.NotContains(t, string(b), "Code generated")
}

func TestObjectFilename(t *testing.T) {
	tests := map[string]struct {
		tmpl, object, field, want string
	}{
		"operation":                  {"{field}_{object}.go", "Mutation", "createOrder", "create_order_mutation.go"},
		"fields":                     {"{object}_fields.go", "CertificateApplication", "x", "certificate_application_fields.go"},
		"acronym stays together":     {"{object}_fields.go", "HTTPLog", "x", "http_log_fields.go"},
		"digits":                     {"{object}_fields.go", "OAuth2Client", "x", "o_auth2_client_fields.go"},
		"leading underscore dropped": {"{field}_{object}.go", "Query", "_empty", "empty_query.go"},
		"test suffix avoided":        {"{field}_{object}.go", "Test", "run", "run_test_.go"},
		"object first":               {"{object}_{field}_resolver.go", "OrderItem", "product", "order_item_product_resolver.go"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tt.want, objectFilename(tt.tmpl, tt.object, tt.field))
		})
	}
}

func TestLayoutInvalidModelPath(t *testing.T) {
	cfg, err := config.LoadConfig("testdata/invalid_model_path/gqlgen.yml")
	require.NoError(t, err)

	require.NoError(t, cfg.Init())

	_, err = codegen.BuildData(cfg)
	require.Error(t, err)
}

func TestOmitTemplateComment(t *testing.T) {
	_ = syscall.Unlink("testdata/omit_template_comment/resolver.go")

	cfg, err := config.LoadConfig("testdata/omit_template_comment/gqlgen.yml")
	require.NoError(t, err)
	p := Plugin{}

	require.NoError(t, cfg.Init())

	data, err := codegen.BuildData(cfg)
	require.NoError(t, err)

	require.NoError(t, p.GenerateCode(data))
	assertNoErrors(
		t,
		"github.com/99designs/gqlgen/plugin/resolvergen/testdata/omit_template_comment/out",
	)
}

func TestOmitResolverEmbedding(t *testing.T) {
	_ = syscall.Unlink("testdata/omit_resolver_embedding/resolver.go")

	cfg, err := config.LoadConfig("testdata/omit_resolver_embedding/gqlgen.yml")
	require.NoError(t, err)
	p := Plugin{}

	require.NoError(t, cfg.Init())

	data, err := codegen.BuildData(cfg)
	require.NoError(t, err)

	require.NoError(t, p.GenerateCode(data))
	assertNoErrors(
		t,
		"github.com/99designs/gqlgen/plugin/resolvergen/testdata/omit_resolver_embedding/out",
	)

	b, err := os.ReadFile("testdata/omit_resolver_embedding/out/schema.resolvers.go")
	require.NoError(t, err)
	require.Contains(t, string(b), "struct{ r *CustomResolverType }")
}

func TestResolver_Implementation(t *testing.T) {
	_ = syscall.Unlink("testdata/resolver_implementor/resolver.go")

	cfg, err := config.LoadConfig("testdata/resolver_implementor/gqlgen.yml")
	require.NoError(t, err)
	p := Plugin{}

	require.NoError(t, cfg.Init())

	data, err := codegen.BuildData(cfg, &implementorTest{})
	require.NoError(t, err)

	require.NoError(t, p.GenerateCode(data))
	assertNoErrors(
		t,
		"github.com/99designs/gqlgen/plugin/resolvergen/testdata/resolver_implementor/out",
	)
}

func TestCustomResolverTemplate(t *testing.T) {
	_ = syscall.Unlink("testdata/resolvertemplate/out/resolver.go")
	cfg, err := config.LoadConfig("testdata/resolvertemplate/gqlgen.yml")
	require.NoError(t, err)
	p := Plugin{}

	require.NoError(t, cfg.Init())

	data, err := codegen.BuildData(cfg)
	require.NoError(t, err)

	require.NoError(t, p.GenerateCode(data))
}

func TestCommentDirective(t *testing.T) {
	_ = syscall.Unlink("testdata/comment_directive/resolver.go")

	cfg, err := config.LoadConfig("testdata/comment_directive/gqlgen.yml")
	require.NoError(t, err)
	p := Plugin{}

	require.NoError(t, cfg.Init())

	data, err := codegen.BuildData(cfg)
	require.NoError(t, err)

	require.NoError(t, p.GenerateCode(data))
	assertNoErrors(
		t,
		"github.com/99designs/gqlgen/plugin/resolvergen/testdata/comment_directive/out",
	)

	b, err := os.ReadFile("testdata/comment_directive/out/schema.resolvers.go")
	require.NoError(t, err)
	source := string(b)

	require.Contains(t, source, "//nolint:test // test")
}

func testFollowSchemaPersistence(t *testing.T, dir string) {
	_ = syscall.Unlink(dir + "/out/resolver.go")

	cfg, err := config.LoadConfig(dir + "/gqlgen.yml")
	require.NoError(t, err)
	p := Plugin{}

	require.NoError(t, cfg.Init())

	data, err := codegen.BuildData(cfg)
	require.NoError(t, err)

	require.NoError(t, p.GenerateCode(data))
	assertNoErrors(t, "github.com/99designs/gqlgen/plugin/resolvergen/"+dir+"/out")
}

func overWriteFile(t *testing.T, sourceFile, destinationFile string) {
	input, err := os.ReadFile(sourceFile)
	require.NoError(t, err)

	err = os.WriteFile(destinationFile, input, 0o644)
	require.NoError(t, err)
}

func assertNoErrors(t *testing.T, pkg string) {
	t.Helper()
	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedImports |
			packages.NeedTypes |
			packages.NeedTypesSizes,
	}, pkg)
	require.NoError(t, err)

	var errFilePos []string
	var errors []packages.Error
	for _, pkg := range pkgs {
		errors = append(errors, pkg.Errors...)
		for _, err := range pkg.Errors {
			errFilePos = append(errFilePos, err.Pos+":"+err.Msg)
		}
	}
	require.Emptyf(t, errors, "There are compilation errors:\n"+
		strings.Join(errFilePos, "\n"))
}

type implementorTest struct{}

func (i *implementorTest) Implement(_ string, _ *codegen.Field) string {
	return "panic(\"implementor implemented me\")"
}
