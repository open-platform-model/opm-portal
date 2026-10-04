package authz

// Code classifies a denial, so the read API can tell "you may not read this"
// from "the portal could not decide".
type Code string

const (
	// CodeUnauthenticated: the identity names no principal, or names one
	// this Authorizer does not serve. No review was sent.
	CodeUnauthenticated Code = "unauthenticated"
	// CodeForbidden: the cluster denied the read, gave no opinion, or the
	// portal never permits it (a write verb, Secret data).
	CodeForbidden Code = "forbidden"
	// CodeInvalid: the attributes are malformed or use a wildcard. No
	// review was sent.
	CodeInvalid Code = "invalid"
	// CodeUnavailable: the access review failed, timed out, or could not be
	// evaluated. It is never cached.
	CodeUnavailable Code = "unavailable"
)

// DenialError is the error Check returns whenever it does not grant.
type DenialError struct {
	Code       Code
	Attributes Attributes
	cause      error
}

// Error never names the object or the identity: a caller receives the same
// text whether or not the object exists (0030:D7:R1), and no principal is
// written into an error string.
func (d *DenialError) Error() string {
	msg := "authorization " + string(d.Code) + ": " + d.Attributes.String()
	if d.cause != nil {
		msg += ": " + d.cause.Error()
	}
	return msg
}

// Unwrap returns the failure behind a CodeUnavailable denial, for logs.
func (d *DenialError) Unwrap() error { return d.cause }
