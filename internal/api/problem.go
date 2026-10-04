package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
	"github.com/open-platform-model/opm-portal/internal/authz"
	"github.com/open-platform-model/opm-portal/internal/readmodel"
	"github.com/open-platform-model/opm-portal/internal/stream"
)

// Problem details. None names an object, an identity or the check that
// failed, so the forbidden one reads the same for every refusal (0030:D7:R1).
const (
	detailForbidden       = "The request was refused: you may not read this, or the portal does not serve it."
	detailNotFound        = "The object does not exist."
	detailUnknownCluster  = "The portal does not serve this cluster."
	detailUnknownPath     = "The read API has no such resource."
	detailUnauthenticated = "The request names no authenticated user."
	detailNotReadable     = "The portal holds no readable copy of this kind: its reading identity may not list and watch it, or its cache has not synced yet."
	detailUnavailable     = "An authorization review or the Kubernetes API failed. Try again."
	detailMethod          = "The read API serves GET only."
	detailTooManyStreams  = "This session holds as many streams as it may. Close one first."
)

// apiError is a refusal on its way to a problem document. cause is logged,
// never sent.
type apiError struct {
	status int
	code   string
	detail string
	cause  error
}

func (e *apiError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.code, e.cause)
	}
	return e.code
}

func (e *apiError) Unwrap() error { return e.cause }

func forbidden() *apiError {
	return &apiError{status: http.StatusForbidden, code: v1.CodeForbidden, detail: detailForbidden}
}

func badRequest(detail string) *apiError {
	return &apiError{status: http.StatusBadRequest, code: v1.CodeBadRequest, detail: detail}
}

func notFound(detail string) *apiError {
	return &apiError{status: http.StatusNotFound, code: v1.CodeNotFound, detail: detail}
}

// classify maps an error from authz, the read model or the broker to the
// refusal the client sees. Anything unrecognized is upstream_unavailable.
func classify(err error) *apiError {
	if e, ok := errors.AsType[*apiError](err); ok {
		return e
	}
	if d, ok := errors.AsType[*authz.DenialError](err); ok {
		switch d.Code {
		case authz.CodeForbidden, authz.CodeInvalid:
			return forbidden()
		case authz.CodeUnauthenticated:
			return &apiError{status: http.StatusUnauthorized, code: v1.CodeUnauthenticated, detail: detailUnauthenticated}
		case authz.CodeUnavailable:
		}
		return &apiError{status: http.StatusServiceUnavailable, code: v1.CodeUpstreamUnavailable, detail: detailUnavailable, cause: err}
	}
	switch {
	case errors.Is(err, readmodel.ErrNotCovered):
		return forbidden()
	case errors.Is(err, readmodel.ErrNotFound):
		return notFound(detailNotFound)
	case errors.Is(err, readmodel.ErrUnavailable):
		return &apiError{status: http.StatusServiceUnavailable, code: v1.CodeNotReadableByPortal, detail: detailNotReadable, cause: err}
	case errors.Is(err, stream.ErrUnauthenticated):
		return &apiError{status: http.StatusUnauthorized, code: v1.CodeUnauthenticated, detail: detailUnauthenticated}
	case errors.Is(err, stream.ErrTooManyStreams):
		return &apiError{status: http.StatusTooManyRequests, code: v1.CodeTooManyStreams, detail: detailTooManyStreams}
	case errors.Is(err, stream.ErrTooManyTopics), errors.Is(err, stream.ErrTopicNotServed):
		return badRequest(err.Error())
	}
	if t, ok := errors.AsType[*stream.TopicError](err); ok {
		return badRequest(t.Error())
	}
	return &apiError{status: http.StatusServiceUnavailable, code: v1.CodeUpstreamUnavailable, detail: detailUnavailable, cause: err}
}

// writeProblem writes err as a problem document. It sets no instance
// member: echoing the path would make the refusal for an existing object
// differ from the one for a missing object only by their names, and a
// constant body is simpler to hold to that (0030:D7:R1). A server-side failure is
// logged with its cause; the client sees only the code and detail.
func writeProblem(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error) {
	e := classify(err)
	if e.status >= http.StatusInternalServerError {
		log.Warn("read api request failed", "path", r.URL.Path, "code", e.code, "error", e.cause)
	}
	p := v1.Problem{
		Type:   v1.ProblemTypeBlank,
		Title:  http.StatusText(e.status),
		Status: e.status,
		Detail: e.detail,
		Code:   e.code,
	}
	body, mErr := json.Marshal(p)
	if mErr != nil {
		// A Problem holds strings and an int; it always encodes.
		http.Error(w, http.StatusText(e.status), e.status)
		return
	}
	h := w.Header()
	h.Set("Content-Type", v1.ProblemContentType)
	h.Set("Cache-Control", "no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	if e.status == http.StatusMethodNotAllowed {
		h.Set("Allow", http.MethodGet)
	}
	w.WriteHeader(e.status)
	if _, err := w.Write(body); err != nil {
		// The status is sent; a client that went away is all this can mean.
		log.Debug("writing a problem document", "path", r.URL.Path, "error", err)
	}
}
