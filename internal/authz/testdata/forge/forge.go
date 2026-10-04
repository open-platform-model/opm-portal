// Package forge must not compile: it tries every way another package could
// build an authz.Grant. seal_test.go builds it and expects each attempt to
// fail. It lives under testdata so ./... never builds it.
package forge

import "github.com/open-platform-model/opm-portal/internal/authz"

// ByLiteral fills the grant's field from outside the package.
func ByLiteral() authz.Grant {
	return authz.Grant{sealed: nil}
}

// ByIssue calls the package's unexported constructor.
func ByIssue() authz.Grant {
	return authz.issue(authz.Identity{Username: "mallory"}, authz.Attributes{})
}
