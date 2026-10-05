package graph

import (
	"net/url"
	"strings"

	"github.com/open-platform-model/opm-portal/internal/readmodel"
)

// Node ids are `<prefix>:<part>[/<part>...]`, built from kinds, groups,
// namespaces and names only. No id holds a UID, so a node keeps its id
// across a portal restart and a delete and recreate of its object
// (portal:D4:R6).
const (
	prefixPlatform     = "plat"
	prefixCatalog      = "cat"
	prefixRegistration = "treg"
	prefixInstance     = "mi"
	prefixPackage      = "mp"
	prefixModule       = "mod"
	prefixSource       = "src"
	prefixComponent    = "comp"
	prefixObject       = "obj"
	prefixGroup        = "grp"
)

// partEscaper escapes what url.PathEscape keeps but the id grammar uses:
// the prefix separator and the version marker of catalog and module paths.
var partEscaper = strings.NewReplacer(":", "%3A", "@", "%40")

// part escapes one id part. An empty part is "_", so the core group and
// cluster scope have a part; a literal "_" is escaped to stay distinct.
func part(s string) string {
	switch s {
	case "":
		return "_"
	case "_":
		return "%5F"
	}
	return partEscaper.Replace(url.PathEscape(s))
}

func id(prefix string, parts ...string) string {
	escaped := make([]string, len(parts))
	for i, p := range parts {
		escaped[i] = part(p)
	}
	return prefix + ":" + strings.Join(escaped, "/")
}

// below returns the id of a node scoped under another node: the parent's
// prefix becomes the first part, so the parent's escaped parts are kept.
func below(prefix, parent string, parts ...string) string {
	out := prefix + ":" + strings.Replace(parent, ":", "/", 1)
	for _, p := range parts {
		out += "/" + part(p)
	}
	return out
}

func platformID(name string) string        { return id(prefixPlatform, name) }
func catalogID(catalog string) string      { return id(prefixCatalog, catalog) }
func registrationID(name string) string    { return id(prefixRegistration, name) }
func instanceID(ns, name string) string    { return id(prefixInstance, ns, name) }
func packageID(ns, name string) string     { return id(prefixPackage, ns, name) }
func moduleID(path, version string) string { return id(prefixModule, path, version) }

func objectID(r readmodel.ObjectRef) string {
	return id(prefixObject, r.Group, r.Kind, r.Namespace, r.Name)
}

func sourceID(group, kind, ns, name string) string {
	return id(prefixSource, group, kind, ns, name)
}

func componentID(owner, component string) string {
	return below(prefixComponent, owner, component)
}

func groupID(kind GroupKind, parent string) string {
	return below(prefixGroup, string(kind)+":"+strings.Replace(parent, ":", "/", 1))
}
