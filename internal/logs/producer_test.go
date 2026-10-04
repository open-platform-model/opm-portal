package logs

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/open-platform-model/opm-portal/internal/authz"
	"github.com/open-platform-model/opm-portal/internal/readmodel"
	"github.com/open-platform-model/opm-portal/internal/readmodel/readmodeltest"
	"github.com/open-platform-model/opm-portal/internal/stream"
)

// unit is a Producer over a fake source, publishing into a capture.
type unit struct {
	p     *Producer
	src   *fakeSource
	pub   *capture
	reach *fakeReach
	clock *clock
	logs  *bytes.Buffer
}

func newUnit(t *testing.T, src *fakeSource, readerRule readmodeltest.Rule, opts Options) *unit {
	t.Helper()
	c := &clock{now: time.Date(2026, 10, 4, 10, 0, 0, 0, time.UTC)}
	logs := &bytes.Buffer{}
	opts.Now = c.Now
	opts.Logger = slog.New(slog.NewTextHandler(&lockedWriter{w: logs}, nil))
	reach := &fakeReach{}
	p, err := New(Config{
		Source:     src,
		Reach:      reach,
		Authorizer: newAuthorizer(t, readerRule, 0, reader, alice),
		Reader:     reader,
		Options:    opts,
	})
	if err != nil {
		t.Fatal(err)
	}
	pub := newCapture()
	p.SetPublisher(pub)
	return &unit{p: p, src: src, pub: pub, reach: reach, clock: c, logs: logs}
}

// activate starts topic and releases it when the test ends.
func (u *unit) activate(t *testing.T, topic string) (release func()) {
	t.Helper()
	release = u.p.Activate(mustTopic(t, topic))
	t.Cleanup(release)
	return release
}

const liveTopic = "log:default/" + f1Pod + "/" + f1Container

func TestLinesAreDeliveredWithTheirTime(t *testing.T) {
	u := newUnit(t, newFakeSource(f1PodObject(t)), readmodeltest.AllowAll, Options{})
	u.activate(t, liveTopic)
	fs := u.src.open(t)
	at := u.clock.Now().Add(time.Second)
	fs.write(t, liveTS(at, "GET /healthz 200"), "no timestamp here")

	first, second := u.pub.next(t), u.pub.next(t)
	stamp := at
	want := []Message{
		{Seq: 1, Type: TypeLine, Container: f1Container, Time: &stamp, Text: "GET /healthz 200"},
		{Seq: 2, Type: TypeLine, Container: f1Container, Text: "no timestamp here"},
	}
	for i, got := range []Message{first, second} {
		if !sameMessage(got, want[i]) {
			t.Errorf("message %d = %+v, want %+v", i, got, want[i])
		}
	}
	tail := int64(200)
	wantOpts := corev1.PodLogOptions{Container: f1Container, Follow: true, Timestamps: true, TailLines: &tail}
	if opts := u.src.opts[0]; !equality.Semantic.DeepEqual(opts, wantOpts) {
		t.Errorf("log options = %+v, want follow, timestamps, tail 200 and no byte limit", opts)
	}
}

// sameMessage compares messages, their times by instant.
func sameMessage(a, b Message) bool {
	if (a.Time == nil) != (b.Time == nil) || (a.Time != nil && !a.Time.Equal(*b.Time)) {
		return false
	}
	a.Time, b.Time = nil, nil
	return a == b
}

func TestTailLinesAreCapped(t *testing.T) {
	u := newUnit(t, newFakeSource(f1PodObject(t)), readmodeltest.AllowAll, Options{TailLines: 1 << 20})
	u.activate(t, liveTopic)
	u.src.open(t)
	if got := *u.src.opts[0].TailLines; got != maxTailLines {
		t.Errorf("TailLines = %d, want %d", got, maxTailLines)
	}
}

func TestAnOversizeLineIsCutAndMarked(t *testing.T) {
	u := newUnit(t, newFakeSource(f1PodObject(t)), readmodeltest.AllowAll, Options{MaxLineBytes: 64})
	u.activate(t, liveTopic)
	fs := u.src.open(t)
	at := u.clock.Now().Add(time.Second)
	long := liveTS(at, strings.Repeat("x", 200))
	fs.write(t, long, liveTS(at, "next"))

	m := u.pub.next(t)
	if m.Marker != MarkerTruncated || m.Cut != len(long)-64 || len(m.Text) != 64-len(liveTS(at, "")) {
		t.Errorf("long line = marker %q cut %d text %d bytes, want truncated, cut %d", m.Marker, m.Cut, len(m.Text), len(long)-64)
	}
	if m := u.pub.next(t); m.Text != "next" || m.Marker != "" {
		t.Errorf("following line = %+v, want intact", m)
	}
}

func TestLinesOverTheRateAreDroppedAndCounted(t *testing.T) {
	u := newUnit(t, newFakeSource(f1PodObject(t)), readmodeltest.AllowAll, Options{LinesPerSecond: 1, LineBurst: 2})
	u.activate(t, liveTopic)
	fs := u.src.open(t)
	at := u.clock.Now().Add(time.Second)
	for i := range 5 {
		fs.write(t, liveTS(at, "line "+string(rune('a'+i))))
	}
	if a, b := u.pub.next(t), u.pub.next(t); a.Text != "line a" || b.Text != "line b" {
		t.Fatalf("burst = %q, %q", a.Text, b.Text)
	}
	u.pub.none(t)
	u.clock.advance(time.Second)
	fs.write(t, liveTS(at, "after"))
	if m := u.pub.next(t); m.Type != TypeMarker || m.Marker != MarkerRateLimited || m.Dropped != 3 {
		t.Fatalf("marker = %+v, want rate-limited with 3 dropped", m)
	}
	if m := u.pub.next(t); m.Text != "after" {
		t.Errorf("line after the marker = %+v", m)
	}
}

func TestAByteRateBoundsLinesToo(t *testing.T) {
	u := newUnit(t, newFakeSource(f1PodObject(t)), readmodeltest.AllowAll, Options{BytesPerSecond: 10, ByteBurst: 10})
	u.activate(t, liveTopic)
	fs := u.src.open(t)
	at := u.clock.Now().Add(time.Second)
	fs.write(t, liveTS(at, "123456"), liveTS(at, "789012"))
	_ = fs.w.Close()
	if got := types([]Message{u.pub.next(t), u.pub.next(t), u.pub.next(t)}); !slices.Equal(got, []string{"line", "marker:rate-limited", "end:upstream_closed"}) {
		t.Errorf("messages = %v", got)
	}
}

func TestALargeTailSkipsAheadToLiveOutput(t *testing.T) {
	u := newUnit(t, newFakeSource(f1PodObject(t)), readmodeltest.AllowAll, Options{MaxTailBytes: 10})
	u.activate(t, liveTopic)
	fs := u.src.open(t)
	old := u.clock.Now().Add(-time.Hour)
	fs.write(t, liveTS(old, "aaaaa"), liveTS(old, "bbbbb"), liveTS(old, "ccccc"), liveTS(old, "d"),
		liveTS(u.clock.Now().Add(time.Second), "live"))
	got := make([]Message, 0, 4)
	for range 4 {
		got = append(got, u.pub.next(t))
	}
	if ts := types(got); !slices.Equal(ts, []string{"line", "line", "marker:skipped", "line"}) || got[2].Dropped != 2 || got[3].Text != "live" {
		t.Errorf("messages = %v (%+v)", ts, got)
	}
}

func TestAStoppedContainerEndsTheTopic(t *testing.T) {
	stopped := f1PodObject(t)
	for i := range stopped.Status.ContainerStatuses {
		stopped.Status.ContainerStatuses[i].State = corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 1}}
	}
	tests := []struct {
		name   string
		after  *corev1.Pod
		reason string
	}{
		{"container stopped", stopped, ReasonContainerStopped},
		{"still running", f1PodObject(t), ReasonUpstreamClosed},
		{"pod deleted", nil, ReasonPodNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u := newUnit(t, newFakeSource(f1PodObject(t)), readmodeltest.AllowAll, Options{})
			u.activate(t, liveTopic)
			fs := u.src.open(t)
			fs.write(t, liveTS(u.clock.Now().Add(time.Second), "last words"))
			u.src.setPod(tc.after)
			_ = fs.w.Close()
			if got := types([]Message{u.pub.next(t), u.pub.next(t)}); !slices.Equal(got, []string{"line", "end:" + tc.reason}) {
				t.Errorf("messages = %v", got)
			}
			if it := u.pub.items[len(u.pub.items)-1]; it.Event != stream.EventLogEnd {
				t.Errorf("end message event = %q, want logend", it.Event)
			}
		})
	}
}

func TestPreviousContainerLogsAreReadOnce(t *testing.T) {
	u := newUnit(t, newFakeSource(f1PodObject(t)), readmodeltest.AllowAll, Options{})
	u.activate(t, liveTopic+"/previous")
	fs := u.src.open(t)
	fs.write(t, liveTS(u.clock.Now().Add(-time.Minute), "panic: boom"))
	_ = fs.w.Close()
	if got := types([]Message{u.pub.next(t), u.pub.next(t)}); !slices.Equal(got, []string{"line", "end:" + ReasonCompleted}) {
		t.Errorf("messages = %v", got)
	}
	if opts := u.src.opts[0]; opts.Follow || !opts.Previous || opts.LimitBytes != nil {
		t.Errorf("log options = %+v, want previous without follow or byte limit", opts)
	}
}

func TestEndReasonsBeforeAnyLine(t *testing.T) {
	tests := []struct {
		name    string
		topic   string
		pod     *corev1.Pod
		logsErr error
		reader  readmodeltest.Rule
		reason  string
		logs    int
	}{
		{"unknown container", "log:default/" + f1Pod + "/sidecar", f1PodObject(t), nil, readmodeltest.AllowAll, ReasonContainerNotFound, 0},
		{"pod gone", liveTopic, nil, nil, readmodeltest.AllowAll, ReasonPodNotFound, 0},
		{"no previous", liveTopic + "/previous", f1PodObject(t), apierrors.NewBadRequest("previous terminated container not found"), readmodeltest.AllowAll, ReasonNoPrevious, 1},
		{"container waiting", liveTopic, f1PodObject(t), apierrors.NewBadRequest("container is waiting to start: ContainerCreating"), readmodeltest.AllowAll, ReasonContainerWaiting, 1},
		{"upstream error", liveTopic, f1PodObject(t), errBoom, readmodeltest.AllowAll, ReasonUnavailable, 1},
		{"reader denied the log", liveTopic, f1PodObject(t), nil, denySubresource("log"), ReasonUnavailable, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := newFakeSource(tc.pod)
			src.logsErr = tc.logsErr
			u := newUnit(t, src, tc.reader, Options{})
			u.activate(t, tc.topic)
			if m := u.pub.next(t); m.Type != TypeEnd || m.Reason != tc.reason {
				t.Errorf("message = %+v, want end %s", m, tc.reason)
			}
			if _, logs := src.calls(); logs != tc.logs {
				t.Errorf("%d log streams opened, want %d", logs, tc.logs)
			}
		})
	}
}

func denySubresource(sub string) readmodeltest.Rule {
	return func(_ string, ra authorizationv1.ResourceAttributes) bool { return ra.Subresource != sub }
}

func TestReleaseClosesTheUpstreamStream(t *testing.T) {
	u := newUnit(t, newFakeSource(f1PodObject(t)), readmodeltest.AllowAll, Options{})
	release := u.activate(t, liveTopic)
	fs := u.src.open(t)
	fs.write(t, liveTS(u.clock.Now().Add(time.Second), "one"))
	u.pub.next(t)
	release()
	if !fs.isClosed() {
		t.Error("the upstream stream is still open after the release")
	}
	u.pub.none(t)
	if items, err := u.p.Snapshot(context.Background(), mustTopic(t, liveTopic)); err != nil || len(items) != 0 {
		t.Errorf("snapshot after release = %d items, %v", len(items), err)
	}
}

func TestSnapshotCarriesRecentMessages(t *testing.T) {
	u := newUnit(t, newFakeSource(f1PodObject(t)), readmodeltest.AllowAll, Options{Buffer: 3})
	u.activate(t, liveTopic)
	fs := u.src.open(t)
	at := u.clock.Now().Add(time.Second)
	for _, l := range []string{"1", "2", "3", "4", "5"} {
		fs.write(t, liveTS(at, l))
	}
	for range 5 {
		u.pub.next(t)
	}
	items, err := u.p.Snapshot(context.Background(), mustTopic(t, liveTopic))
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(items))
	for _, it := range items {
		m := decode(t, it.Data)
		got = append(got, m.Text)
		if it.Event != stream.EventLog || it.Attrs != podLogRead(mustTopic(t, liveTopic)) {
			t.Errorf("snapshot item %+v", it)
		}
	}
	if !slices.Equal(got, []string{"3", "4", "5"}) {
		t.Errorf("snapshot = %v, want the last three lines", got)
	}
}

func TestLogContentNeverReachesPortalLogs(t *testing.T) {
	u := newUnit(t, newFakeSource(f1PodObject(t)), readmodeltest.AllowAll, Options{MaxLineBytes: 32, LinesPerSecond: 1, LineBurst: 1})
	u.activate(t, liveTopic)
	fs := u.src.open(t)
	at := u.clock.Now().Add(time.Second)
	fs.write(t, liveTS(at, "password=hunter2-in-a-very-long-line"), liveTS(at, "token=s3cr3t"))
	u.pub.next(t)
	_ = fs.w.CloseWithError(errBoom)
	u.pub.next(t) // rate-limited marker
	if m := u.pub.next(t); m.Reason != ReasonUnavailable {
		t.Fatalf("end = %+v, want unavailable", m)
	}
	logged := u.logs.String()
	if !strings.Contains(logged, "boom") {
		t.Errorf("the read failure was not logged: %q", logged)
	}
	for _, secret := range []string{"hunter2", "s3cr3t", "password", "token="} {
		if strings.Contains(logged, secret) {
			t.Errorf("portal log carries log content %q: %q", secret, logged)
		}
	}
}

func TestAdmitAsksTheReadModel(t *testing.T) {
	u := newUnit(t, newFakeSource(f1PodObject(t)), readmodeltest.AllowAll, Options{})
	topic := mustTopic(t, liveTopic)
	attrs, ok := u.p.Attributes(topic)
	if !ok || len(attrs) != 1 || attrs[0] != (authz.Attributes{Verb: "get", Resource: pods, Subresource: "log", Namespace: "default", Name: f1Pod}) {
		t.Fatalf("Attributes = %v, %v", attrs, ok)
	}
	if _, ok := u.p.Attributes(mustTopic(t, "instance:default/podinfo")); ok {
		t.Error("an instance topic is served")
	}
	g, err := u.p.cfg.Authorizer.Check(context.Background(), alice, attrs[0])
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		reach  error
		grants []authz.Grant
		admit  bool
		notAd  bool
	}{
		{"reached", nil, []authz.Grant{g}, true, false},
		{"not reachable", readmodel.ErrNotReachable, []authz.Grant{g}, false, true},
		{"no grant", nil, []authz.Grant{{}}, false, true},
		{"no grants", nil, nil, false, true},
		{"unavailable", readmodel.ErrUnavailable, []authz.Grant{g}, false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u.reach.err = tc.reach
			err := u.p.Admit(context.Background(), alice, topic, tc.grants)
			if (err == nil) != tc.admit || errors.Is(err, stream.ErrNotAdmitted) != tc.notAd {
				t.Errorf("Admit = %v, want admitted %v, not admitted %v", err, tc.admit, tc.notAd)
			}
		})
	}
}

func TestNewRefusesAnIncompleteConfig(t *testing.T) {
	az := newAuthorizer(t, readmodeltest.AllowAll, 0, reader)
	full := Config{Source: newFakeSource(nil), Reach: &fakeReach{}, Authorizer: az, Reader: reader}
	for name, mutate := range map[string]func(*Config){
		"source":     func(c *Config) { c.Source = nil },
		"reach":      func(c *Config) { c.Reach = nil },
		"authorizer": func(c *Config) { c.Authorizer = nil },
		"reader":     func(c *Config) { c.Reader = authz.Identity{} },
	} {
		c := full
		mutate(&c)
		if _, err := New(c); err == nil {
			t.Errorf("New without %s succeeded", name)
		}
	}
}

// TestClientSourceStreamsAFakeClientsetLog runs a topic over client-go's
// fake clientset, whose log stream is a fixed body.
func TestClientSourceStreamsAFakeClientsetLog(t *testing.T) {
	cs := fake.NewClientset(f1PodObject(t))
	src := ClientSource(cs)
	pod, err := src.Pod(context.Background(), "default", f1Pod)
	if err != nil || pod.Name != f1Pod {
		t.Fatalf("Pod = %v, %v", pod, err)
	}
	if _, err := src.Pod(context.Background(), "default", "missing"); !apierrors.IsNotFound(err) {
		t.Errorf("missing Pod = %v, want not found", err)
	}
	rc, err := src.Logs(context.Background(), "default", f1Pod, &corev1.PodLogOptions{Container: f1Container})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(rc)
	_ = rc.Close()
	if string(body) != "fake logs" {
		t.Errorf("body = %q", body)
	}

	p, err := New(Config{Source: src, Reach: &fakeReach{}, Authorizer: newAuthorizer(t, readmodeltest.AllowAll, 0, reader), Reader: reader})
	if err != nil {
		t.Fatal(err)
	}
	pub := newCapture()
	p.SetPublisher(pub)
	t.Cleanup(p.Activate(mustTopic(t, liveTopic)))
	line, end := pub.next(t), pub.next(t)
	if line.Text != "fake logs" || end.Type != TypeEnd || end.Reason != ReasonUpstreamClosed {
		t.Errorf("messages = %+v, %+v; want the body, then upstream_closed for a running container", line, end)
	}
}

func TestInitAndEphemeralContainersCanBeFollowed(t *testing.T) {
	pod := f1PodObject(t)
	pod.Spec.InitContainers = []corev1.Container{{Name: "migrate"}}
	pod.Spec.EphemeralContainers = []corev1.EphemeralContainer{{EphemeralContainerCommon: corev1.EphemeralContainerCommon{Name: "debugger"}}}
	for _, c := range []string{"migrate", "debugger"} {
		t.Run(c, func(t *testing.T) {
			u := newUnit(t, newFakeSource(pod), readmodeltest.AllowAll, Options{})
			u.activate(t, "log:default/"+f1Pod+"/"+c)
			u.src.open(t)
			if got := u.src.opts[0].Container; got != c {
				t.Errorf("container = %q, want %q", got, c)
			}
		})
	}
}

func TestAnEndedReadRestartsOnTheNextAdmit(t *testing.T) {
	u := newUnit(t, newFakeSource(f1PodObject(t)), readmodeltest.AllowAll, Options{})
	u.activate(t, liveTopic)
	first := u.src.open(t)
	first.write(t, liveTS(u.clock.Now().Add(time.Second), "one"))
	_ = first.w.Close()
	if got := types([]Message{u.pub.next(t), u.pub.next(t)}); !slices.Equal(got, []string{"line", "end:" + ReasonUpstreamClosed}) {
		t.Fatalf("first read = %v", got)
	}
	topic := mustTopic(t, liveTopic)
	attrs, _ := u.p.Attributes(topic)
	g, err := u.p.cfg.Authorizer.Check(context.Background(), alice, attrs[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := u.p.Admit(context.Background(), alice, topic, []authz.Grant{g}); err != nil {
		t.Fatal(err)
	}
	second := u.src.open(t)
	second.write(t, liveTS(u.clock.Now().Add(time.Second), "two"))
	m := u.pub.next(t)
	if m.Text != "two" || m.Seq != 3 {
		t.Errorf("first message of the new read = %+v, want line two with seq 3", m)
	}
	items, err := u.p.Snapshot(context.Background(), topic)
	if err != nil || len(items) != 1 || decode(t, items[0].Data).Text != "two" {
		t.Errorf("snapshot after the restart = %d items, %v; want only the new read", len(items), err)
	}
	// A read in progress is not restarted.
	if err := u.p.Admit(context.Background(), alice, topic, []authz.Grant{g}); err != nil {
		t.Fatal(err)
	}
	if _, logs := u.src.calls(); logs != 2 {
		t.Errorf("%d log streams opened, want 2", logs)
	}
}

func TestASupersededActivationPublishesNothing(t *testing.T) {
	u := newUnit(t, newFakeSource(f1PodObject(t)), readmodeltest.AllowAll, Options{})
	u.activate(t, liveTopic)
	old := u.src.open(t)
	u.activate(t, liveTopic)
	current := u.src.open(t)
	if !old.isClosed() {
		eventually(t, "the old upstream to close", old.isClosed)
	}
	current.write(t, liveTS(u.clock.Now().Add(time.Second), "current"))
	if m := u.pub.next(t); m.Text != "current" {
		t.Errorf("message = %+v, want the current activation's line", m)
	}
	u.pub.none(t)
}

func TestMarkersArriveWhenNoLineFollows(t *testing.T) {
	u := newUnit(t, newFakeSource(f1PodObject(t)), readmodeltest.AllowAll, Options{LinesPerSecond: 1, LineBurst: 1, MarkerDelay: 20 * time.Millisecond})
	u.activate(t, liveTopic)
	fs := u.src.open(t)
	at := u.clock.Now().Add(time.Second)
	fs.write(t, liveTS(at, "a"), liveTS(at, "b"), liveTS(at, "c"))
	if m := u.pub.next(t); m.Text != "a" {
		t.Fatalf("first = %+v", m)
	}
	if m := u.pub.next(t); m.Type != TypeMarker || m.Marker != MarkerRateLimited || m.Dropped != 2 {
		t.Errorf("after silence = %+v, want rate-limited with 2 dropped", m)
	}
	u.pub.none(t)
}

func TestTheTailEndsAtTheFirstLiveLineOrTailLines(t *testing.T) {
	tests := []struct {
		name  string
		lines []time.Duration // offsets from the portal's clock
		want  []string
	}{
		// A node clock behind the portal's: live lines look old, but the
		// tail never holds more than TailLines lines.
		{"node clock behind", []time.Duration{-time.Hour, -time.Hour, -time.Second, -time.Second}, []string{"line", "marker:skipped", "line", "line"}},
		// A line stamped after the stream opened ends the tail, and an
		// older-looking line after it is live, not skipped.
		{"first live line", []time.Duration{-time.Hour, time.Second, -time.Hour, -time.Hour}, []string{"line", "line", "line", "line"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u := newUnit(t, newFakeSource(f1PodObject(t)), readmodeltest.AllowAll, Options{TailLines: 2, MaxTailBytes: 6})
			u.activate(t, liveTopic)
			fs := u.src.open(t)
			now := u.clock.Now()
			for i, d := range tc.lines {
				fs.write(t, liveTS(now.Add(d), "line"+string(rune('a'+i))))
			}
			got := make([]Message, 0, len(tc.want))
			for range tc.want {
				got = append(got, u.pub.next(t))
			}
			if ts := types(got); !slices.Equal(ts, tc.want) {
				t.Errorf("messages = %v, want %v", ts, tc.want)
			}
		})
	}
}

func TestTheSnapshotIsBoundedByBytes(t *testing.T) {
	u := newUnit(t, newFakeSource(f1PodObject(t)), readmodeltest.AllowAll, Options{BufferBytes: 300})
	u.activate(t, liveTopic)
	fs := u.src.open(t)
	at := u.clock.Now().Add(time.Second)
	for i := range 10 {
		fs.write(t, liveTS(at, strings.Repeat(string(rune('a'+i)), 40)))
	}
	for range 10 {
		u.pub.next(t)
	}
	items, err := u.p.Snapshot(context.Background(), mustTopic(t, liveTopic))
	if err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, it := range items {
		total += len(it.Data)
	}
	if total > 300 || len(items) == 0 || decode(t, items[len(items)-1].Data).Text != strings.Repeat("j", 40) {
		t.Errorf("snapshot = %d items, %d bytes; want the newest within 300 bytes", len(items), total)
	}
}
