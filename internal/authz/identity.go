package authz

import (
	"maps"
	"slices"
	"strconv"
	"strings"
)

// anonymousUser is the name Kubernetes gives an unauthenticated request.
const anonymousUser = "system:anonymous"

// Identity is the principal a read is authorized for: the kubeconfig's user
// in local mode, the signed-in person in-cluster.
type Identity struct {
	Username string
	UID      string
	Groups   []string
	Extra    map[string][]string

	// reader marks the in-cluster portal ServiceAccount. Only
	// ServiceAccountIdentity sets it, so no identity built from sign-in
	// claims can take the reader's route, however its fields read
	// (portal:D6:R3). It is part of the key, so a person with the reader's
	// exact claims shares no decision, grant or log exemption with it.
	reader bool
}

// Authenticated reports whether i names a principal. An empty or blank
// username, or the anonymous user, is no principal, whatever groups it
// carries: such an identity must never reach the cluster, because an empty
// subject would be answered for someone other than the caller (portal:D6:R2).
func (i Identity) Authenticated() bool {
	u := strings.TrimSpace(i.Username)
	return u != "" && u != anonymousUser
}

// clone returns a deep copy of i, so a caller that mutates its slices or map
// after Check cannot change what a Grant records.
func (i Identity) clone() Identity {
	c := Identity{Username: i.Username, UID: i.UID, Groups: slices.Clone(i.Groups), reader: i.reader}
	if i.Extra != nil {
		c.Extra = make(map[string][]string, len(i.Extra))
		for k, v := range i.Extra {
			c.Extra[k] = slices.Clone(v)
		}
	}
	return c
}

// key returns a canonical, unambiguous encoding of i. Two identities that
// differ only in the order of their groups or extra values have equal keys.
func (i Identity) key() string {
	var b strings.Builder
	if i.reader {
		b.WriteString("reader;")
	}
	b.WriteString("u=")
	b.WriteString(strconv.Quote(i.Username))
	b.WriteString(";uid=")
	b.WriteString(strconv.Quote(i.UID))
	b.WriteString(";g=")
	writeSorted(&b, i.Groups)
	b.WriteString(";x=")
	for _, k := range slices.Sorted(maps.Keys(i.Extra)) {
		b.WriteString(strconv.Quote(k))
		b.WriteByte(':')
		writeSorted(&b, i.Extra[k])
	}
	return b.String()
}

func writeSorted(b *strings.Builder, values []string) {
	b.WriteByte('[')
	for _, v := range slices.Sorted(slices.Values(values)) {
		b.WriteString(strconv.Quote(v))
		b.WriteByte(',')
	}
	b.WriteByte(']')
}
