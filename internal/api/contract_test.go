package api

import (
	"maps"
	"os"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"sigs.k8s.io/yaml"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
)

const openAPIPath = "../../openapi/v1alpha1.yaml"

const schemaRef = "#/components/schemas/"

type openAPI struct {
	OpenAPI    string                    `json:"openapi"`
	Paths      map[string]map[string]any `json:"paths"`
	Components struct {
		Schemas    map[string]schemaDoc `json:"schemas"`
		Parameters map[string]paramDoc  `json:"parameters"`
	} `json:"components"`
}

type schemaDoc struct {
	Ref        string               `json:"$ref"`
	Type       string               `json:"type"`
	Format     string               `json:"format"`
	Required   []string             `json:"required"`
	Properties map[string]schemaDoc `json:"properties"`
	Items      *schemaDoc           `json:"items"`
}

type paramDoc struct {
	Name string `json:"name"`
	In   string `json:"in"`
}

func loadOpenAPI(t *testing.T) openAPI {
	t.Helper()
	raw, err := os.ReadFile(openAPIPath)
	if err != nil {
		t.Fatal(err)
	}
	var doc openAPI
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parsing %s: %v", openAPIPath, err)
	}
	if doc.OpenAPI != "3.1.0" {
		t.Errorf("openapi = %q, want 3.1.0", doc.OpenAPI)
	}
	return doc
}

// lookup walks a decoded YAML value along keys.
func lookup(v any, keys ...string) any {
	for _, k := range keys {
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = m[k]
	}
	return v
}

var pathParam = regexp.MustCompile(`\{(\w+)\}`)

// TestEveryRouteIsInTheOpenAPIDocument: the document's paths are exactly
// the served routes, each a GET whose 200 is the route's wire type, with
// every path parameter declared.
func TestEveryRouteIsInTheOpenAPIDocument(t *testing.T) {
	doc := loadOpenAPI(t)
	served := map[string]route{}
	for _, rt := range routes {
		served[rt.pattern] = rt
	}
	if got, want := slices.Sorted(maps.Keys(doc.Paths)), slices.Sorted(maps.Keys(served)); !slices.Equal(got, want) {
		t.Fatalf("document paths %v\nserved routes  %v", got, want)
	}
	for pattern, rt := range served {
		item := doc.Paths[pattern]
		if ops := slices.Sorted(maps.Keys(item)); !slices.Equal(ops, []string{"get"}) {
			t.Errorf("%s: operations %v, want get only", pattern, ops)
			continue
		}
		op := item["get"]
		var declared []string
		params, _ := lookup(op, "parameters").([]any)
		for _, p := range params {
			ref, _ := lookup(p, "$ref").(string)
			if pd, ok := doc.Components.Parameters[strings.TrimPrefix(ref, "#/components/parameters/")]; ok && pd.In == "path" {
				declared = append(declared, pd.Name)
			}
		}
		var want []string
		for _, m := range pathParam.FindAllStringSubmatch(pattern, -1) {
			want = append(want, m[1])
		}
		if !slices.Equal(declared, want) {
			t.Errorf("%s: path parameters %v, want %v", pattern, declared, want)
		}
		if rt.doc == nil {
			if lookup(op, "responses", "200", "content", "text/event-stream") == nil {
				t.Errorf("%s: the stream's 200 is not text/event-stream", pattern)
			}
			continue
		}
		ref, _ := lookup(op, "responses", "200", "content", "application/json", "schema", "$ref").(string)
		if want := schemaRef + reflect.TypeOf(rt.doc).Name(); ref != want {
			t.Errorf("%s: 200 schema %q, want %q", pattern, ref, want)
		}
		if lookup(op, "responses", "default", "$ref") != "#/components/responses/Problem" {
			t.Errorf("%s: errors are not the Problem response", pattern)
		}
	}
}

// TestEveryWireTypeMatchesItsSchema: every type reachable from a served
// document, Removed and Problem has a schema of its own name whose
// properties are its JSON fields with matching types, and whose required
// list is exactly its fields without omitempty. No schema is left over.
func TestEveryWireTypeMatchesItsSchema(t *testing.T) {
	c := &schemaCheck{t: t, schemas: loadOpenAPI(t).Components.Schemas, seen: map[string]bool{}}
	for _, rt := range routes {
		if rt.doc != nil {
			c.check(reflect.TypeOf(rt.doc))
		}
	}
	c.check(reflect.TypeFor[v1.Removed]())
	c.check(reflect.TypeFor[v1.Problem]())
	for name := range c.schemas {
		if !c.seen[name] {
			t.Errorf("schema %s matches no wire type", name)
		}
	}
}

type schemaCheck struct {
	t       *testing.T
	schemas map[string]schemaDoc
	seen    map[string]bool
}

func (c *schemaCheck) check(typ reflect.Type) {
	t, name := c.t, typ.Name()
	if c.seen[name] {
		return
	}
	c.seen[name] = true
	s, ok := c.schemas[name]
	if !ok {
		t.Errorf("no schema %s", name)
		return
	}
	if s.Type != "object" {
		t.Errorf("%s: type %q, want object", name, s.Type)
	}
	fields := jsonFields(typ)
	if got, want := slices.Sorted(maps.Keys(s.Properties)), slices.Sorted(maps.Keys(fields)); !slices.Equal(got, want) {
		t.Errorf("%s: properties %v, want %v", name, got, want)
	}
	var required []string
	for fname, f := range fields {
		if !f.omitempty {
			required = append(required, fname)
		}
	}
	if got, want := slices.Sorted(slices.Values(s.Required)), slices.Sorted(slices.Values(required)); !slices.Equal(got, want) {
		t.Errorf("%s: required %v, want %v", name, got, want)
	}
	for fname, f := range fields {
		if p, ok := s.Properties[fname]; ok {
			if msg := matchType(f.typ, p); msg != "" {
				t.Errorf("%s.%s: %s", name, fname, msg)
			}
		}
		if st := structOf(f.typ); st != nil {
			c.check(st)
		}
	}
}

type jsonField struct {
	typ       reflect.Type
	omitempty bool
}

// jsonFields returns typ's JSON fields as encoding/json sees them, with
// embedded structs flattened.
func jsonFields(typ reflect.Type) map[string]jsonField {
	out := map[string]jsonField{}
	for f := range typ.Fields() {
		tag := f.Tag.Get("json")
		if f.Anonymous && tag == "" {
			maps.Copy(out, jsonFields(f.Type))
			continue
		}
		name, opts, _ := strings.Cut(tag, ",")
		if name == "-" || name == "" {
			continue
		}
		out[name] = jsonField{typ: f.Type, omitempty: strings.Contains(opts, "omitempty")}
	}
	return out
}

var timeType = reflect.TypeFor[time.Time]()

// structOf returns the wire struct a field holds, through pointers and
// slices, or nil.
func structOf(typ reflect.Type) reflect.Type {
	for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice {
		typ = typ.Elem()
	}
	if typ.Kind() == reflect.Struct && typ != timeType {
		return typ
	}
	return nil
}

// scalarTypes maps Go kinds to the schema types they encode as.
var scalarTypes = map[reflect.Kind]string{
	reflect.String: "string",
	reflect.Bool:   "boolean",
	reflect.Int:    "integer",
	reflect.Int64:  "integer",
}

// matchType compares a Go field type with a property schema.
func matchType(typ reflect.Type, s schemaDoc) string {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if want, ok := scalarTypes[typ.Kind()]; ok {
		if s.Type != want {
			return "want " + want + ", schema says " + s.Type + s.Ref
		}
		return ""
	}
	switch {
	case typ == timeType:
		if s.Type != "string" || s.Format != "date-time" {
			return "want a date-time string"
		}
	case typ.Kind() == reflect.Slice:
		if s.Type != "array" || s.Items == nil {
			return "want an array"
		}
		if msg := matchType(typ.Elem(), *s.Items); msg != "" {
			return "items: " + msg
		}
	case typ.Kind() == reflect.Struct:
		if s.Ref != schemaRef+typ.Name() {
			return "want $ref " + schemaRef + typ.Name() + ", schema says " + s.Ref
		}
	default:
		return "no schema mapping for " + typ.String()
	}
	return ""
}
