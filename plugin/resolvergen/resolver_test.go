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

func TestLayoutFollowSchemaWithObjectFilename(t *testing.T) {
	testFollowSchemaPersistence(t, "testdata/perobject")

	// Every field of an object lands in that object's file, including Customer.email,
	// which the schema extends further down than the type itself.
	for file, wants := range map[string][]string{
		"query":    {") Customer(ctx", ") Order(ctx", "type queryCustomResolverType struct"},
		"customer": {") Name(ctx", ") Orders(ctx", ") Email(ctx", "type customerCustomResolverType struct"},
		"order":    {") Total(ctx", "type orderCustomResolverType struct"},
	} {
		b, err := os.ReadFile("testdata/perobject/out/" + file + ".go")
		require.NoError(t, err, file)
		for _, want := range wants {
			require.Contains(t, string(b), want, file)
		}
	}
	require.NoFileExists(t, "testdata/perobject/out/schema.go")
}

func TestLayoutFollowSchemaWithFieldFilename(t *testing.T) {
	testFollowSchemaPersistence(t, "testdata/perfield")

	// One file per resolver, plus a file per object for its type and accessor.
	for file, wants := range map[string][]string{
		"customer_query.go":  {") Customer(ctx"},
		"order_query.go":     {") Order(ctx"},
		"name_customer.go":   {") Name(ctx"},
		"orders_customer.go": {") Orders(ctx"},
		"email_customer.go":  {") Email(ctx"},
		"total_order.go":     {") Total(ctx"},
		"query.go":           {"type queryCustomResolverType struct", ") Query() QueryResolver"},
		"customer.go":        {"type customerCustomResolverType struct"},
		"order.go":           {"type orderCustomResolverType struct"},
	} {
		b, err := os.ReadFile("testdata/perfield/out/" + file)
		require.NoError(t, err, file)
		for _, want := range wants {
			require.Contains(t, string(b), want, file)
		}
		if strings.HasSuffix(file, "_query.go") || strings.Contains(file, "_customer.go") ||
			strings.HasSuffix(file, "_order.go") {
			require.NotContains(t, string(b), "type ", file)
		}
	}
}

func TestLayoutFollowSchemaWithObjectFilenameMovesImports(t *testing.T) {
	const dir = "testdata/perobject_move/out/"
	// Start from the layout before {object} was used: the implementation lives in another
	// file, which is the only place that imports strings.
	_ = syscall.Unlink(dir + "query.go")
	overWriteFile(t, dir+"legacy.go.txt", dir+"legacy.go")

	cfg, err := config.LoadConfig("testdata/perobject_move/gqlgen.yml")
	require.NoError(t, err)
	require.NoError(t, cfg.Init())
	data, err := codegen.BuildData(cfg)
	require.NoError(t, err)
	require.NoError(t, (&Plugin{}).GenerateCode(data))

	b, err := os.ReadFile(dir + "query.go")
	require.NoError(t, err)
	require.Contains(t, string(b), `"strings"`)
	require.Contains(t, string(b), `strings.ToUpper("hello")`)

	// The old file is now redundant; once it is gone the package must compile.
	require.NoError(t, syscall.Unlink(dir+"legacy.go"))
	assertNoErrors(t, "github.com/99designs/gqlgen/plugin/resolvergen/testdata/perobject_move/out")
}

func TestGqlToResolverName(t *testing.T) {
	tests := map[string]struct {
		tmpl, object, field, want string
	}{
		"default template follows the schema file": {"", "Query", "", "out/schema.resolvers.go"},
		"name placeholder":                         {"{name}.custom.go", "Query", "", "out/schema.custom.go"},
		"object placeholder":                       {"{object}.go", "CertificateApplication", "", "out/certificate_application.go"},
		"acronym stays together":                   {"{object}.go", "HTTPLog", "", "out/http_log.go"},
		"digits":                                   {"{object}.go", "OAuth2Client", "", "out/o_auth2_client.go"},
		"both placeholders":                        {"{name}_{object}.go", "Query", "", "out/schema_query.go"},
		"field and object":                         {"{field}_{object}.go", "Mutation", "createOrder", "out/createOrder_mutation.go"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tt.want, gqlToResolverName("out", "path/to/schema.graphql", tt.tmpl, tt.object, tt.field))
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
