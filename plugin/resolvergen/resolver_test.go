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

func TestLayoutFollowSchemaWithOperationFilenames(t *testing.T) {
	testFollowSchemaPersistence(t, "testdata/operationfiles")
	const dir = "testdata/operationfiles/out/"

	// Each mutation and subscription gets a file; everything else follows the schema file.
	for file, wants := range map[string][]string{
		"create_order_mutation.go":   {") CreateOrder(ctx"},
		"cancel_order_mutation.go":   {") CancelOrder(ctx"},
		"order_sent_subscription.go": {") OrderSent(ctx"},
		"schema.resolvers.go": {
			") Customer(ctx", ") Name(ctx",
			") Query() QueryResolver", ") Mutation() MutationResolver",
			") Subscription() SubscriptionResolver",
		},
	} {
		b, err := os.ReadFile(dir + file)
		require.NoError(t, err, file)
		for _, want := range wants {
			require.Contains(t, string(b), want, file)
		}
	}
	b, err := os.ReadFile(dir + "schema.resolvers.go")
	require.NoError(t, err)
	require.NotContains(t, string(b), ") CreateOrder(ctx")
	require.NotContains(t, string(b), ") OrderSent(ctx")
}

func TestLayoutFollowSchemaMovesImportsWithMutation(t *testing.T) {
	const dir = "testdata/operationfiles_move/out/"
	// Start from the layout without the option: the implementation lives in a file that is the
	// only place importing strings.
	_ = syscall.Unlink(dir + "shout_mutation.go")
	overWriteFile(t, dir+"legacy.go.txt", dir+"legacy.go")

	cfg, err := config.LoadConfig("testdata/operationfiles_move/gqlgen.yml")
	require.NoError(t, err)
	require.NoError(t, cfg.Init())
	data, err := codegen.BuildData(cfg)
	require.NoError(t, err)
	require.NoError(t, (&Plugin{}).GenerateCode(data))

	b, err := os.ReadFile(dir + "shout_mutation.go")
	require.NoError(t, err)
	require.Contains(t, string(b), `"strings"`)
	require.Contains(t, string(b), `strings.ToUpper("hello")`)

	// The old file is now redundant; once it is gone the package must compile.
	require.NoError(t, syscall.Unlink(dir+"legacy.go"))
	assertNoErrors(t, "github.com/99designs/gqlgen/plugin/resolvergen/testdata/operationfiles_move/out")
}

func TestGqlToOperationResolverName(t *testing.T) {
	tests := map[string]struct{ tmpl, schema, field, want string }{
		"field":                  {"{field}_mutation.go", "orders.graphql", "createOrder", "out/create_order_mutation.go"},
		"acronym stays together": {"{field}_mutation.go", "orders.graphql", "uploadHTTPLog", "out/upload_http_log_mutation.go"},
		"schema file name":       {"{name}_{field}.go", "orders.graphql", "cancel", "out/orders_cancel.go"},
		"leading underscore":     {"{field}_mutation.go", "orders.graphql", "_noop", "out/noop_mutation.go"},
		"test suffix avoided":    {"{field}_test.go", "orders.graphql", "run", "out/run_test_.go"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tt.want, gqlToOperationResolverName("out", "dir/"+tt.schema, tt.tmpl, tt.field))
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
