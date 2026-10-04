package stream

import (
	"slices"
	"testing"
)

func seqs(es []ringEntry) []uint64 {
	out := make([]uint64, 0, len(es))
	for i := range es {
		out = append(out, es[i].seq)
	}
	return out
}

func TestRing(t *testing.T) {
	tests := []struct {
		name      string
		size      int
		floor     uint64
		add       []uint64
		after     uint64
		covers    bool
		wantSince []uint64
	}{
		{"empty ring covers from its floor", 3, 10, nil, 10, true, []uint64{}},
		{"before the floor is a gap", 3, 10, nil, 9, false, []uint64{}},
		{"partial fill", 3, 10, []uint64{11, 13}, 10, true, []uint64{11, 13}},
		{"since skips what was seen", 3, 10, []uint64{11, 13}, 11, true, []uint64{13}},
		{"exact fill keeps the floor", 3, 10, []uint64{11, 12, 13}, 10, true, []uint64{11, 12, 13}},
		{"overflow raises the floor", 3, 10, []uint64{11, 12, 13, 14}, 10, false, []uint64{12, 13, 14}},
		{"overflow resumes from the raised floor", 3, 10, []uint64{11, 12, 13, 14}, 11, true, []uint64{12, 13, 14}},
		{"wrap twice", 2, 0, []uint64{1, 2, 3, 4, 5}, 3, true, []uint64{4, 5}},
		{"wrap twice gap", 2, 0, []uint64{1, 2, 3, 4, 5}, 2, false, []uint64{4, 5}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := newRing(tc.size, 1<<20, tc.floor)
			for _, s := range tc.add {
				r.add(ringEntry{seq: s})
			}
			if got := r.covers(tc.after); got != tc.covers {
				t.Errorf("covers(%d) = %v, want %v", tc.after, got, tc.covers)
			}
			if got := seqs(r.since(tc.after)); !slices.Equal(got, tc.wantSince) {
				t.Errorf("since(%d) = %v, want %v", tc.after, got, tc.wantSince)
			}
		})
	}
}

func TestRingReturnsTheItems(t *testing.T) {
	r := newRing(2, 1<<20, 0)
	r.add(ringEntry{seq: 1, item: Item{Event: EventUpsert}})
	r.add(ringEntry{seq: 2, item: Item{Event: EventDelete}})
	got := r.since(0)
	if len(got) != 2 || got[0].item.Event != EventUpsert || got[1].item.Event != EventDelete {
		t.Errorf("since(0) = %+v", got)
	}
}

func TestRingIsBoundedByBytes(t *testing.T) {
	r := newRing(10, 10, 0)
	for seq := uint64(1); seq <= 4; seq++ {
		r.add(ringEntry{seq: seq, item: Item{Data: []byte("1234")}})
	}
	if got := seqs(r.since(0)); !slices.Equal(got, []uint64{3, 4}) {
		t.Errorf("since(0) = %v, want the newest entries within 10 bytes", got)
	}
	if r.covers(1) || !r.covers(2) {
		t.Errorf("floor = %d, want 2 after evicting 1 and 2", r.floor)
	}
	r.add(ringEntry{seq: 5, item: Item{Data: []byte("an entry larger than the bound")}})
	if got := seqs(r.since(0)); !slices.Equal(got, []uint64{5}) {
		t.Errorf("since(0) = %v, want only the oversize newest entry", got)
	}
}
