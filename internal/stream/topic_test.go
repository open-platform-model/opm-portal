package stream

import (
	"errors"
	"strings"
	"testing"
)

func TestParseTopicAccepts(t *testing.T) {
	tests := []struct {
		in        string
		kind      Kind
		namespace string
		name      string
	}{
		{"platform", KindPlatform, "", ""},
		{"instances", KindInstances, "", ""},
		{"instances:team-a", KindInstances, "team-a", ""},
		{"instance:apps/blog", KindInstance, "apps", "blog"},
		{"instance:apps/blog.v2", KindInstance, "apps", "blog.v2"},
		{"package:apps/stack", KindPackage, "apps", "stack"},
		{"registration:backup.k8up", KindRegistration, "", "backup.k8up"},
		{"events:instance:apps/blog", KindEvents, "", ""},
		{"events:platform", KindEvents, "", ""},
		{"log:apps/blog-0/server", KindLog, "apps", "blog-0"},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ParseTopic(tc.in)
			if err != nil {
				t.Fatalf("ParseTopic(%q) = %v", tc.in, err)
			}
			if got.Kind() != tc.kind || got.Namespace() != tc.namespace || got.Name() != tc.name {
				t.Errorf("ParseTopic(%q) = %+v", tc.in, got)
			}
			if got.String() != tc.in {
				t.Errorf("String() = %q, want %q", got.String(), tc.in)
			}
		})
	}
}

func TestParseTopicRefuses(t *testing.T) {
	for _, in := range []string{
		"",
		"platform:x",
		"pods:*",
		"instances:*",
		"instances:",
		"instance:apps",
		"instance:apps/",
		"instance:/blog",
		"instance:Apps/blog",
		"instance:apps/blog/extra",
		"instance:apps/*",
		"registration:",
		"registration",
		"events",
		"events:",
		"events:instances",
		"events:events:platform",
		"events:log:apps/p/c",
		"log:apps/p",
		"log:apps/p/c/d",
		"Platform",
		" platform",
		"instance:" + strings.Repeat("a", 64) + "/blog",
		strings.Repeat("instances:", 100),
	} {
		t.Run(in, func(t *testing.T) {
			got, err := ParseTopic(in)
			if err == nil {
				t.Fatalf("ParseTopic(%q) = %v, want an error", in, got)
			}
			if _, ok := errors.AsType[*TopicError](err); !ok {
				t.Errorf("error %T is not a *TopicError", err)
			}
			if got != (Topic{}) {
				t.Errorf("a refused topic returned %+v", got)
			}
		})
	}
}

func TestTopicRef(t *testing.T) {
	ev, err := ParseTopic("events:instance:apps/blog")
	if err != nil {
		t.Fatal(err)
	}
	ref, ok := ev.Ref()
	if !ok || ref.Kind() != KindInstance || ref.Namespace() != "apps" || ref.Name() != "blog" {
		t.Errorf("Ref() = %+v, %v", ref, ok)
	}
	inst, _ := ParseTopic("instance:apps/blog")
	if _, ok := inst.Ref(); ok {
		t.Error("an instance topic has a ref")
	}
	logTopic, _ := ParseTopic("log:apps/blog-0/server")
	if logTopic.Container() != "server" {
		t.Errorf("Container() = %q", logTopic.Container())
	}
}

func TestTopicsAreComparable(t *testing.T) {
	a, _ := ParseTopic("instance:apps/blog")
	b, _ := ParseTopic("instance:apps/blog")
	set := map[Topic]int{a: 1}
	if set[b] != 1 {
		t.Error("equal topics are not equal map keys")
	}
}
