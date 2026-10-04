package stream

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation"
)

// Kind names what a topic follows.
type Kind string

const (
	// KindPlatform follows the Platform singleton: "platform".
	KindPlatform Kind = "platform"
	// KindInstances follows the instance list: "instances", or
	// "instances:<namespace>" for one namespace.
	KindInstances Kind = "instances"
	// KindInstance follows one ModuleInstance: "instance:<namespace>/<name>".
	KindInstance Kind = "instance"
	// KindPackage follows one ModulePackage: "package:<namespace>/<name>".
	KindPackage Kind = "package"
	// KindRegistration follows one TransformerRegistration:
	// "registration:<name>".
	KindRegistration Kind = "registration"
	// KindEvents follows the Kubernetes events about an object topic:
	// "events:<object topic>".
	KindEvents Kind = "events"
	// KindLog is reserved for pod log topics,
	// "log:<namespace>/<pod>/<container>". It parses, and the broker refuses
	// it until log topics are served.
	KindLog Kind = "log"
)

// InstancesResource returns the resource a list topic lists: a producer
// names list on it, and nothing else, for "instances" and "instances:<ns>".
// It is a function so no importer can change what the broker serves a list
// topic under.
func InstancesResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "opmodel.dev", Version: "v1alpha1", Resource: "moduleinstances"}
}

// maxTopicLen bounds a topic name, well above the longest valid one an
// object reference can produce.
const maxTopicLen = 512

// Topic is a parsed, valid topic name. The zero Topic is invalid. Topics are
// comparable and can be map keys.
type Topic struct {
	kind      Kind
	namespace string
	name      string
	container string
	ref       string // the object topic an events topic follows
}

// TopicError reports a topic name that does not parse. Its message quotes
// the name the client sent.
type TopicError struct {
	Topic  string
	Reason string
}

func (e *TopicError) Error() string {
	return fmt.Sprintf("invalid topic %q: %s", e.Topic, e.Reason)
}

// ParseTopic parses a topic name. It returns a *TopicError for anything that
// is not one of the forms the Kind constants document.
func ParseTopic(s string) (Topic, error) {
	if len(s) > maxTopicLen {
		return Topic{}, &TopicError{Topic: s[:64] + "...", Reason: "name too long"}
	}
	t, reason := parse(s)
	if reason != "" {
		return Topic{}, &TopicError{Topic: s, Reason: reason}
	}
	return t, nil
}

func parse(s string) (t Topic, reason string) {
	head, rest, hasArg := strings.Cut(s, ":")
	if !hasArg {
		switch Kind(head) {
		case KindPlatform:
			return Topic{kind: KindPlatform}, ""
		case KindInstances:
			return Topic{kind: KindInstances}, ""
		case KindInstance, KindPackage, KindRegistration, KindEvents, KindLog:
			return Topic{}, "missing argument"
		}
		return Topic{}, "unknown topic kind"
	}
	switch Kind(head) {
	case KindPlatform:
		return Topic{}, "platform takes no argument"
	case KindInstances:
		if r := checkLabel("namespace", rest); r != "" {
			return Topic{}, r
		}
		return Topic{kind: KindInstances, namespace: rest}, ""
	case KindInstance, KindPackage:
		ns, name, r := namespaced(rest)
		if r != "" {
			return Topic{}, r
		}
		return Topic{kind: Kind(head), namespace: ns, name: name}, ""
	case KindRegistration:
		if r := checkSubdomain("name", rest); r != "" {
			return Topic{}, r
		}
		return Topic{kind: KindRegistration, name: rest}, ""
	case KindEvents:
		return parseEvents(rest)
	case KindLog:
		return parseLog(rest)
	}
	return Topic{}, "unknown topic kind"
}

func parseEvents(rest string) (t Topic, reason string) {
	ref, r := parse(rest)
	if r != "" {
		return Topic{}, r
	}
	if !ref.isObject() {
		return Topic{}, "events follow platform, instance, package or registration topics only"
	}
	return Topic{kind: KindEvents, ref: ref.String()}, ""
}

func parseLog(rest string) (t Topic, reason string) {
	parts := strings.Split(rest, "/")
	if len(parts) != 3 {
		return Topic{}, "want log:<namespace>/<pod>/<container>"
	}
	if r := checkLabel("namespace", parts[0]); r != "" {
		return Topic{}, r
	}
	if r := checkSubdomain("pod", parts[1]); r != "" {
		return Topic{}, r
	}
	if r := checkLabel("container", parts[2]); r != "" {
		return Topic{}, r
	}
	return Topic{kind: KindLog, namespace: parts[0], name: parts[1], container: parts[2]}, ""
}

func namespaced(rest string) (ns, name, reason string) {
	ns, name, ok := strings.Cut(rest, "/")
	if !ok {
		return "", "", "want <namespace>/<name>"
	}
	if r := checkLabel("namespace", ns); r != "" {
		return "", "", r
	}
	if r := checkSubdomain("name", name); r != "" {
		return "", "", r
	}
	return ns, name, ""
}

func checkLabel(what, v string) string {
	if len(validation.IsDNS1123Label(v)) > 0 {
		return what + " is not a DNS-1123 label"
	}
	return ""
}

func checkSubdomain(what, v string) string {
	if len(validation.IsDNS1123Subdomain(v)) > 0 {
		return what + " is not a DNS-1123 subdomain"
	}
	return ""
}

func (t Topic) isObject() bool {
	switch t.kind {
	case KindPlatform, KindInstance, KindPackage, KindRegistration:
		return true
	case KindInstances, KindEvents, KindLog:
		return false
	}
	return false
}

// Kind returns what t follows, or "" for the zero Topic.
func (t Topic) Kind() Kind { return t.kind }

// Namespace returns t's namespace, or "" when t has none.
func (t Topic) Namespace() string { return t.namespace }

// Name returns t's object name (the pod for a log topic), or "".
func (t Topic) Name() string { return t.name }

// Container returns a log topic's container, or "".
func (t Topic) Container() string { return t.container }

// Ref returns the object topic an events topic follows.
func (t Topic) Ref() (Topic, bool) {
	if t.kind != KindEvents {
		return Topic{}, false
	}
	ref, reason := parse(t.ref)
	return ref, reason == ""
}

// String returns t's canonical name, which ParseTopic accepts.
func (t Topic) String() string {
	switch t.kind {
	case KindPlatform:
		return string(KindPlatform)
	case KindInstances:
		if t.namespace == "" {
			return string(KindInstances)
		}
		return string(KindInstances) + ":" + t.namespace
	case KindInstance, KindPackage:
		return string(t.kind) + ":" + t.namespace + "/" + t.name
	case KindRegistration:
		return string(KindRegistration) + ":" + t.name
	case KindEvents:
		return string(KindEvents) + ":" + t.ref
	case KindLog:
		return string(KindLog) + ":" + t.namespace + "/" + t.name + "/" + t.container
	}
	return ""
}
