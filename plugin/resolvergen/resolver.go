package resolvergen

import (
	_ "embed"
	"errors"
	"fmt"
	"go/ast"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/99designs/gqlgen/codegen"
	"github.com/99designs/gqlgen/codegen/config"
	"github.com/99designs/gqlgen/codegen/templates"
	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/internal/rewrite"
	"github.com/99designs/gqlgen/plugin"
)

//go:embed resolver.gotpl
var resolverTemplate string

func New() plugin.Plugin {
	return &Plugin{}
}

type Plugin struct{}

var _ plugin.CodeGenerator = &Plugin{}

func (m *Plugin) Name() string {
	return "resolvergen"
}

func (m *Plugin) GenerateCode(data *codegen.Data) error {
	if !data.Config.Resolver.IsDefined() {
		return nil
	}

	switch data.Config.Resolver.Layout {
	case config.LayoutSingleFile:
		return m.generateSingleFile(data)
	case config.LayoutFollowSchema:

		return m.generatePerSchema(data)
	}

	return nil
}

func (m *Plugin) generateSingleFile(data *codegen.Data) error {
	file := File{}

	if fileExists(data.Config.Resolver.Filename) &&
		data.Config.Resolver.PreserveResolver {
		// file already exists and config says not to update resolver
		// so just return
		return nil
	}

	rewriter, err := rewrite.New(data.Config.Resolver.Dir())
	if err != nil {
		return err
	}

	for _, o := range data.Objects {
		if o.HasResolvers() {
			caser := cases.Title(language.English, cases.NoLower)
			rewriter.MarkStructCopied(
				templates.LcFirst(o.Name) + templates.UcFirst(data.Config.Resolver.Type),
			)
			rewriter.GetMethodBody(data.Config.Resolver.Type, caser.String(o.Name))

			file.Objects = append(file.Objects, o)
		}

		for _, f := range o.Fields {
			if !f.IsResolver {
				continue
			}

			structName := templates.LcFirst(o.Name) + templates.UcFirst(data.Config.Resolver.Type)
			comment := strings.TrimSpace(
				strings.TrimLeft(rewriter.GetMethodComment(structName, f.GoFieldName), `\`),
			)
			implementation := strings.TrimSpace(rewriter.GetMethodBody(structName, f.GoFieldName))
			if implementation != "" {
				resolver := Resolver{
					o,
					f,
					rewriter.GetPrevDecl(structName, f.GoFieldName),
					comment,
					implementation,
					nil,
				}
				file.Resolvers = append(file.Resolvers, &resolver)
			} else {
				resolver := Resolver{o, f, nil, "", `panic("not implemented")`, nil}
				file.Resolvers = append(file.Resolvers, &resolver)
			}
		}
	}

	if fileExists(data.Config.Resolver.Filename) {
		file.name = data.Config.Resolver.Filename
		file.imports = rewriter.ExistingImports(file.name)
		file.RemainingSource = rewriter.RemainingSource(file.name)
	}

	resolverBuild := &ResolverBuild{
		File:                  &file,
		PackageName:           data.Config.Resolver.Package,
		ResolverType:          data.Config.Resolver.Type,
		HasRoot:               true,
		OmitTemplateComment:   data.Config.Resolver.OmitTemplateComment,
		OmitResolverEmbedding: data.Config.Resolver.OmitResolverEmbedding,
	}

	newResolverTemplate := resolverTemplate
	if data.Config.Resolver.ResolverTemplate != "" {
		newResolverTemplate = readResolverTemplate(data.Config.Resolver.ResolverTemplate)
	}

	fileNotice := `// THIS CODE WILL BE UPDATED WITH SCHEMA CHANGES. PREVIOUS IMPLEMENTATION FOR SCHEMA CHANGES WILL BE KEPT IN THE COMMENT SECTION. IMPLEMENTATION FOR UNCHANGED SCHEMA WILL BE KEPT.`
	if data.Config.Resolver.PreserveResolver {
		fileNotice = `// THIS CODE IS A STARTING POINT ONLY. IT WILL NOT BE UPDATED WITH SCHEMA CHANGES.`
	}

	return templates.Render(templates.Options{
		PackageName:  data.Config.Resolver.Package,
		FileNotice:   fileNotice,
		Filename:     data.Config.Resolver.Filename,
		Data:         resolverBuild,
		Packages:     data.Config.Packages,
		Template:     newResolverTemplate,
		PruneOptions: data.Config.GetPruneOptions(),
	})
}

func (m *Plugin) generatePerSchema(data *codegen.Data) error {
	rewriter, err := rewrite.New(data.Config.Resolver.Dir())
	if err != nil {
		return err
	}

	files := map[string]*File{}

	objects := make(codegen.Objects, len(data.Objects)+len(data.Inputs))
	copy(objects, data.Objects)
	copy(objects[len(data.Objects):], data.Inputs)

	for _, o := range objects {
		if o.HasResolvers() {
			// A file per field leaves no file for the object's own declarations to follow.
			objectTmpl := data.Config.Resolver.FilenameTemplate
			if strings.Contains(objectTmpl, fieldPlaceholder) {
				objectTmpl = "{object}.go"
			}
			fnCase := gqlToResolverName(
				data.Config.Resolver.Dir(),
				o.Position.Src.Name,
				objectTmpl,
				o.Name,
				"",
			)
			if data.Config.Resolver.BoilerplateFilename != "" {
				fnCase = filepath.Join(
					data.Config.Resolver.Dir(),
					data.Config.Resolver.BoilerplateFilename,
				)
			}
			fn := strings.ToLower(fnCase)
			if files[fn] == nil {
				files[fn] = &File{
					name: fnCase,
				}
			}

			caser := cases.Title(language.English, cases.NoLower)
			rewriter.MarkStructCopied(
				templates.LcFirst(o.Name) + templates.UcFirst(data.Config.Resolver.Type),
			)
			rewriter.GetMethodBody(data.Config.Resolver.Type, caser.String(o.Name))
			files[fn].Objects = append(files[fn].Objects, o)
		}
		for _, f := range o.Fields {
			if !f.IsResolver {
				continue
			}
			structName := templates.LcFirst(o.Name) + templates.UcFirst(data.Config.Resolver.Type)
			// TODO(steve): Why do we need to trimLeft "\" here? Some bazel thing?
			comment := strings.TrimSpace(
				strings.TrimLeft(rewriter.GetMethodComment(structName, f.GoFieldName), `\`),
			)
			implementation := strings.TrimSpace(rewriter.GetMethodBody(structName, f.GoFieldName))
			resolver := Resolver{
				o,
				f,
				rewriter.GetPrevDecl(structName, f.GoFieldName),
				comment,
				implementation,
				nil,
			}
			var implExists bool
			for _, p := range data.Plugins {
				rImpl, ok := p.(plugin.ResolverImplementer)
				if !ok {
					continue
				}
				if implExists {
					return errors.New("multiple plugins implement ResolverImplementer")
				}
				implExists = true
				resolver.ImplementationRender = rImpl.Implement
			}
			// A template naming the object or the field is independent of the schema file the field
			// is declared in, so a field extended elsewhere still lands in the same place.
			fieldTmpl := data.Config.Resolver.FilenameTemplate
			if o.Root && data.Config.Resolver.OperationFilenameTemplate != "" {
				fieldTmpl = data.Config.Resolver.OperationFilenameTemplate
			}
			src := f.Position.Src.Name
			if strings.Contains(fieldTmpl, objectPlaceholder) ||
				strings.Contains(fieldTmpl, fieldPlaceholder) {
				src = o.Position.Src.Name
			}
			fnCase := gqlToResolverName(
				data.Config.Resolver.Dir(),
				src,
				fieldTmpl,
				o.Name,
				f.Name,
			)
			fn := strings.ToLower(fnCase)
			if files[fn] == nil {
				files[fn] = &File{
					name: fnCase,
				}
			}

			files[fn].Resolvers = append(files[fn].Resolvers, &resolver)
			// With {object} in the template an implementation can move here from another file,
			// so it needs the imports of the file it came from.
			files[fn].movedImports = append(
				files[fn].movedImports,
				rewriter.MethodImports(structName, f.GoFieldName)...,
			)
		}
	}

	var allImports []string
	for _, file := range files {
		file.imports = mergeImports(rewriter.ExistingImports(file.name), file.movedImports)
		file.RemainingSource = rewriter.RemainingSource(file.name)

		for _, i := range file.imports {
			allImports = append(allImports, i.ImportPath)
		}
	}
	data.Config.Packages.LoadAllNames(
		allImports...) // Preload all names in one Load call for performance reasons

	newResolverTemplate := resolverTemplate
	if data.Config.Resolver.ResolverTemplate != "" {
		newResolverTemplate = readResolverTemplate(data.Config.Resolver.ResolverTemplate)
	}

	for _, file := range files {
		if fileExists(file.name) &&
			data.Config.Resolver.PreserveResolver {
			// file already exists and config says not to update resolver
			continue
		}
		resolverBuild := &ResolverBuild{
			File:                  file,
			PackageName:           data.Config.Resolver.Package,
			ResolverType:          data.Config.Resolver.Type,
			OmitTemplateComment:   data.Config.Resolver.OmitTemplateComment,
			OmitResolverEmbedding: data.Config.Resolver.OmitResolverEmbedding,
		}

		var fileNotice strings.Builder
		isBoilerplate := data.Config.Resolver.BoilerplateFilename != "" &&
			file.name == filepath.Join(
				data.Config.Resolver.Dir(),
				data.Config.Resolver.BoilerplateFilename,
			)
		// The boilerplate file holds no implementations, so it gets the standard generated-file
		// header above the package line, which tools use to skip it, instead of the notice.
		if !data.Config.OmitGQLGenFileNotice && !isBoilerplate {
			fileNotice.WriteString(`
			// This file will be automatically regenerated based on the schema, any resolver
			// implementations
			// will be copied through when generating and any unknown code will be moved to the end.
			// Code generated by github.com/99designs/gqlgen`,
			)
			if !data.Config.OmitGQLGenVersionInFileNotice {
				fileNotice.WriteString(` version `)
				fileNotice.WriteString(graphql.Version)
			}
		}

		err := templates.Render(templates.Options{
			PackageName:  data.Config.Resolver.Package,
			FileNotice:   fileNotice.String(),
			Filename:     file.name,
			Data:         resolverBuild,
			Packages:     data.Config.Packages,
			Template:     newResolverTemplate,
			PruneOptions: data.Config.GetPruneOptions(),

			GeneratedHeader: isBoilerplate && !data.Config.OmitGQLGenFileNotice,
		})
		if err != nil {
			return err
		}
	}

	if !fileExists(data.Config.Resolver.Filename) {
		err := templates.Render(templates.Options{
			PackageName: data.Config.Resolver.Package,
			FileNotice: `
				// This file will not be regenerated automatically.
				//
				// It serves as dependency injection for your app, add any dependencies you require
				// here.`,
			Template:     `type {{.}} struct {}`,
			Filename:     data.Config.Resolver.Filename,
			Data:         data.Config.Resolver.Type,
			Packages:     data.Config.Packages,
			PruneOptions: data.Config.GetPruneOptions(),
		})
		if err != nil {
			return err
		}
	}
	return nil
}

type ResolverBuild struct {
	*File
	HasRoot               bool
	PackageName           string
	ResolverType          string
	OmitTemplateComment   bool
	OmitResolverEmbedding bool
}

// ResolverTypeDeclarations renders the unexported per-object resolver
// implementation types — e.g. "queryResolver struct{ *Resolver }", or
// "queryResolver struct{ r *Resolver }" when the root resolver is held in a
// named field rather than embedded.
//
// When more than one type is declared they are emitted as a single grouped
// "type ( ... )" block: gofumpt requires adjacent single-line type
// declarations to be grouped, so producing the group (already column-aligned)
// here keeps the generated file gofumpt-compliant without relying on a
// gofmt/gofumpt pass over the codegen output. A lone type is emitted as a
// single declaration, and the empty string is returned when there are no
// resolver objects.
func (b *ResolverBuild) ResolverTypeDeclarations() string {
	if len(b.Objects) == 0 {
		return ""
	}

	names := make([]string, len(b.Objects))
	width := 0
	for i, o := range b.Objects {
		names[i] = templates.LcFirst(o.Name) + templates.UcFirst(b.ResolverType)
		width = max(width, len(names[i]))
	}

	if len(names) == 1 {
		return fmt.Sprintf("type %s %s", names[0], b.resolverStructType())
	}

	var sb strings.Builder
	sb.WriteString("type (\n")
	for _, name := range names {
		fmt.Fprintf(&sb, "\t%-*s %s\n", width, name, b.resolverStructType())
	}
	sb.WriteString(")")
	return sb.String()
}

// resolverStructType renders the struct body shared by every per-object
// resolver type: the root resolver embedded, or held in a named "r" field when
// Resolver.OmitResolverEmbedding is set.
func (b *ResolverBuild) resolverStructType() string {
	if b.OmitResolverEmbedding {
		return fmt.Sprintf("struct{ r *%s }", b.ResolverType)
	}
	return fmt.Sprintf("struct{ *%s }", b.ResolverType)
}

type File struct {
	name string
	// These are separated because the type definition of the resolver object may live in a
	// different file from the resolver method implementations, for example when extending a type in
	// a different graphql schema file
	Objects         []*codegen.Object
	Resolvers       []*Resolver
	imports         []rewrite.Import
	movedImports    []rewrite.Import
	RemainingSource string
}

// mergeImports appends the extra imports that the existing ones don't already have.
func mergeImports(existing, extra []rewrite.Import) []rewrite.Import {
	seen := make(map[rewrite.Import]bool, len(existing))
	for _, i := range existing {
		seen[i] = true
	}
	for _, i := range extra {
		if !seen[i] {
			seen[i] = true
			existing = append(existing, i)
		}
	}
	return existing
}

func (f *File) Imports() string {
	for _, imp := range f.imports {
		if imp.Alias == "" {
			_, _ = templates.CurrentImports.Reserve(imp.ImportPath)
		} else {
			_, _ = templates.CurrentImports.Reserve(imp.ImportPath, imp.Alias)
		}
	}
	return ""
}

type Resolver struct {
	Object               *codegen.Object
	Field                *codegen.Field
	PrevDecl             *ast.FuncDecl
	Comment              string
	ImplementationStr    string
	ImplementationRender func(prevImplementation string, r *codegen.Field) string
}

func (r *Resolver) Implementation() string {
	if r.ImplementationRender != nil {
		// use custom implementation
		return r.ImplementationRender(r.ImplementationStr, r.Field)
	}
	// if not implementation was previously used, use default implementation
	if r.ImplementationStr == "" {
		// use default implementation, if no implementation was previously used
		return fmt.Sprintf(
			"panic(fmt.Errorf(\"not implemented: %v - %v\"))",
			r.Field.GoFieldName,
			r.Field.Name,
		)
	}
	// use previously used implementation
	return r.ImplementationStr
}

// objectPlaceholder in a filename template is replaced with the snake_case name of the object,
// putting each object's resolvers in a file of their own.
const objectPlaceholder = "{object}"

// fieldPlaceholder in a filename template is replaced with the snake_case name of the field,
// putting each resolver in a file of its own.
const fieldPlaceholder = "{field}"

func gqlToResolverName(base, gqlname, filenameTmpl, objectName, fieldName string) string {
	gqlname = filepath.Base(gqlname)
	ext := filepath.Ext(gqlname)
	if filenameTmpl == "" {
		filenameTmpl = "{name}.resolvers.go"
	}
	filename := strings.ReplaceAll(filenameTmpl, "{name}", strings.TrimSuffix(gqlname, ext))
	filename = strings.ReplaceAll(filename, objectPlaceholder, snakeCase(objectName))
	filename = strings.ReplaceAll(filename, fieldPlaceholder, snakeCase(fieldName))
	return filepath.Join(base, goSafeFilename(filename))
}

// goSafeFilename keeps a name built from a field or object name from being skipped or
// misread by the go tool: files starting with "_" or "." are ignored, and files ending in
// "_test.go" are only compiled into tests. The schema's "_empty" placeholder field is the
// common case.
func goSafeFilename(filename string) string {
	dir, name := filepath.Split(filename)
	name = strings.TrimLeft(name, "_.")
	if strings.HasSuffix(name, "_test.go") {
		name = strings.TrimSuffix(name, ".go") + "_.go"
	}
	return dir + name
}

// snakeCase turns a GraphQL type name such as "CertificateApplication" or "HTTPLog" into
// "certificate_application" or "http_log".
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

func readResolverTemplate(customResolverTemplate string) string {
	contentBytes, err := os.ReadFile(customResolverTemplate)
	if err != nil {
		panic(err)
	}
	return string(contentBytes)
}

func fileExists(fileName string) bool {
	if _, err := os.Stat(fileName); err == nil {
		return true
	}
	return false
}
