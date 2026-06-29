// Copyright (c) the go-ruby-set/set authors
//
// SPDX-License-Identifier: BSD-3-Clause

package set

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"testing"
)

// asSlice returns a stable sorted []int view of an int Set (members are ints in
// these tests), for order-independent comparison.
func asInts(s *Set) []int {
	out := make([]int, 0, s.Size())
	for _, v := range s.ToSlice() {
		out = append(out, v.(int))
	}
	return out
}

func sortedInts(s *Set) []int {
	out := asInts(s)
	sort.Ints(out)
	return out
}

func TestNewAndBasics(t *testing.T) {
	s := New(1, 2, 3, 2, 1) // duplicates collapse
	if s.Size() != 3 {
		t.Fatalf("Size = %d, want 3", s.Size())
	}
	if s.Empty() {
		t.Fatal("Empty = true, want false")
	}
	if !New().Empty() {
		t.Fatal("New().Empty = false, want true")
	}
	if got, want := asInts(s), []int{1, 2, 3}; !reflect.DeepEqual(got, want) {
		t.Fatalf("insertion order = %v, want %v", got, want)
	}
	for _, m := range []int{1, 2, 3} {
		if !s.Include(m) {
			t.Errorf("Include(%d) = false", m)
		}
	}
	if s.Include(9) {
		t.Error("Include(9) = true")
	}
}

func TestAddDelete(t *testing.T) {
	s := New()
	if !s.AddQ(1) {
		t.Error("AddQ(1) new = false")
	}
	if s.AddQ(1) {
		t.Error("AddQ(1) dup = true")
	}
	s.Add(2).Add(3) // chaining
	if got, want := asInts(s), []int{1, 2, 3}; !reflect.DeepEqual(got, want) {
		t.Fatalf("after adds = %v, want %v", got, want)
	}
	if !s.DeleteQ(2) {
		t.Error("DeleteQ(2) present = false")
	}
	if s.DeleteQ(2) {
		t.Error("DeleteQ(2) absent = true")
	}
	s.Delete(1).Delete(99) // chaining + absent no-op
	if got, want := asInts(s), []int{3}; !reflect.DeepEqual(got, want) {
		t.Fatalf("after deletes = %v, want %v", got, want)
	}
	s.Clear()
	if !s.Empty() {
		t.Error("after Clear not empty")
	}
}

func TestEach(t *testing.T) {
	s := New(10, 20, 30)
	var seen []int
	if err := s.Each(func(e any) error { seen = append(seen, e.(int)); return nil }); err != nil {
		t.Fatalf("Each err = %v", err)
	}
	if !reflect.DeepEqual(seen, []int{10, 20, 30}) {
		t.Fatalf("Each order = %v", seen)
	}
	// Error short-circuits.
	boom := errors.New("boom")
	var count int
	err := s.Each(func(e any) error { count++; return boom })
	if !errors.Is(err, boom) || count != 1 {
		t.Fatalf("Each error: err=%v count=%d", err, count)
	}
}

func TestDupIndependence(t *testing.T) {
	s := New(1, 2, 3)
	d := s.Dup()
	d.Add(4)
	if s.Include(4) {
		t.Error("Dup not independent: original mutated")
	}
	if got, want := asInts(s.Dup()), []int{1, 2, 3}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Dup order = %v", got)
	}
}

func TestMergeSubtract(t *testing.T) {
	s := New(1, 2)
	s.Merge(New(2, 3), New(4))
	if got, want := sortedInts(s), []int{1, 2, 3, 4}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Merge = %v", got)
	}
	s.MergeSlice([]any{5, 1})
	if got, want := sortedInts(s), []int{1, 2, 3, 4, 5}; !reflect.DeepEqual(got, want) {
		t.Fatalf("MergeSlice = %v", got)
	}
	s.Subtract(New(2, 4))
	if got, want := sortedInts(s), []int{1, 3, 5}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Subtract = %v", got)
	}
	s.SubtractSlice([]any{1, 99})
	if got, want := sortedInts(s), []int{3, 5}; !reflect.DeepEqual(got, want) {
		t.Fatalf("SubtractSlice = %v", got)
	}
}

func TestAlgebra(t *testing.T) {
	a := New(1, 2, 3)
	b := New(2, 3, 4)
	if got, want := asInts(a.Union(b)), []int{1, 2, 3, 4}; !reflect.DeepEqual(got, want) {
		t.Errorf("Union = %v, want %v", got, want)
	}
	if got, want := asInts(a.Intersection(b)), []int{2, 3}; !reflect.DeepEqual(got, want) {
		t.Errorf("Intersection = %v, want %v", got, want)
	}
	if got, want := asInts(a.Difference(b)), []int{1}; !reflect.DeepEqual(got, want) {
		t.Errorf("Difference = %v, want %v", got, want)
	}
	if got, want := asInts(a.XorSym(b)), []int{1, 4}; !reflect.DeepEqual(got, want) {
		t.Errorf("XorSym = %v, want %v", got, want)
	}
	// Union must not mutate the receiver.
	if a.Size() != 3 {
		t.Error("Union mutated receiver")
	}
}

func TestSubsetSuperset(t *testing.T) {
	a := New(1, 2)
	b := New(1, 2, 3)
	c := New(1, 2)
	if !a.SubsetQ(b) {
		t.Error("a <= b false")
	}
	if !a.SubsetQ(c) {
		t.Error("a <= c (equal) false")
	}
	if !a.ProperSubsetQ(b) {
		t.Error("a < b false")
	}
	if a.ProperSubsetQ(c) {
		t.Error("a < c (equal) true")
	}
	if b.SubsetQ(a) {
		t.Error("b <= a true (larger)")
	}
	if !b.SupersetQ(a) {
		t.Error("b >= a false")
	}
	if !b.ProperSupersetQ(a) {
		t.Error("b > a false")
	}
	if a.ProperSupersetQ(c) {
		t.Error("a > c (equal) true")
	}
	// subset of equal size but different members
	if New(1, 9).SubsetQ(b) {
		t.Error("{1,9} <= {1,2,3} true")
	}
}

func TestDisjointIntersectEqual(t *testing.T) {
	a := New(1, 2, 3)
	if !a.DisjointQ(New(4, 5)) {
		t.Error("disjoint false")
	}
	if a.DisjointQ(New(3, 4)) {
		t.Error("disjoint true for overlapping")
	}
	if !a.IntersectQ(New(3, 4)) {
		t.Error("intersect false")
	}
	// DisjointQ small/large swap branch: larger receiver, smaller arg.
	if !New(1, 2, 3, 4, 5).DisjointQ(New(8, 9)) {
		t.Error("disjoint swap false")
	}
	if !a.EqualQ(New(3, 2, 1)) {
		t.Error("equal (reordered) false")
	}
	if a.EqualQ(New(1, 2)) {
		t.Error("equal different size true")
	}
	if a.EqualQ(New(1, 2, 9)) {
		t.Error("equal same size diff members true")
	}
}

func TestMapSelectReject(t *testing.T) {
	s := New(1, 2, 3, 4)
	got := s.Map(func(e any) any { return e.(int) * 10 })
	if !reflect.DeepEqual(got, []any{10, 20, 30, 40}) {
		t.Errorf("Map = %v", got)
	}
	even := s.Select(func(e any) bool { return e.(int)%2 == 0 })
	if g, w := asInts(even), []int{2, 4}; !reflect.DeepEqual(g, w) {
		t.Errorf("Select = %v", g)
	}
	odd := s.Reject(func(e any) bool { return e.(int)%2 == 0 })
	if g, w := asInts(odd), []int{1, 3}; !reflect.DeepEqual(g, w) {
		t.Errorf("Reject = %v", g)
	}
}

func TestCollectBang(t *testing.T) {
	s := New(1, 2, 3)
	s.CollectBang(func(e any) any { return e.(int) * 10 })
	if g, w := sortedInts(s), []int{10, 20, 30}; !reflect.DeepEqual(g, w) {
		t.Errorf("CollectBang = %v", g)
	}
	// Collision: two members map to same key -> collapse.
	t2 := New(1, 2, 3)
	t2.CollectBang(func(e any) any { return e.(int) % 2 })
	if g, w := sortedInts(t2), []int{0, 1}; !reflect.DeepEqual(g, w) {
		t.Errorf("CollectBang collapse = %v", g)
	}
}

func TestClassify(t *testing.T) {
	s := New(1, 2, 3, 4, 5, 6)
	res := s.Classify(func(e any) any { return e.(int) % 3 })
	if res.Len() != 3 {
		t.Fatalf("Classify Len = %d, want 3", res.Len())
	}
	// First-seen order of block values: 1%3=1, 2%3=2, 3%3=0.
	groups := res.Groups()
	wantVals := []int{1, 2, 0}
	for i, g := range groups {
		if g.Value.(int) != wantVals[i] {
			t.Errorf("group %d value = %v, want %d", i, g.Value, wantVals[i])
		}
	}
	g1, ok := res.Get(1, nil)
	if !ok || !reflect.DeepEqual(asInts(g1), []int{1, 4}) {
		t.Errorf("Get(1) = %v ok=%v", g1, ok)
	}
	if _, ok := res.Get(99, nil); ok {
		t.Error("Get(99) ok = true")
	}
	// Get with a Hasher.
	res2 := New("a", "ab", "b").Classify(func(e any) any { return len(e.(string)) })
	if g, ok := res2.Get(1, func(v any) any { return v }); !ok || g.Size() != 2 {
		t.Errorf("Classify by len Get(1) = %v ok=%v", g, ok)
	}
}

func TestGroupBy(t *testing.T) {
	s := New(1, 2, 3, 4)
	res := s.GroupBy(func(e any) any { return e.(int)%2 == 0 })
	if res.Len() != 2 {
		t.Fatalf("GroupBy Len = %d", res.Len())
	}
	bs := res.Buckets()
	// First value seen is false (1 is odd).
	if bs[0].Value != false || !reflect.DeepEqual(bs[0].Members, []any{1, 3}) {
		t.Errorf("bucket0 = %+v", bs[0])
	}
	if bs[1].Value != true || !reflect.DeepEqual(bs[1].Members, []any{2, 4}) {
		t.Errorf("bucket1 = %+v", bs[1])
	}
}

func TestDivide(t *testing.T) {
	s := New(1, 2, 3, 10, 11, 20)
	parts := s.Divide(func(e any) any { return fmt.Sprintf("%d", e.(int))[0:1] })
	// Group by leading digit char: "1"->{1,10,11}, "2"->{2,20}, "3"->{3}.
	var got [][]int
	for _, p := range parts {
		got = append(got, sortedInts(p))
	}
	sort.Slice(got, func(i, j int) bool { return got[i][0] < got[j][0] })
	want := [][]int{{1, 10, 11}, {2, 20}, {3}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Divide = %v, want %v", got, want)
	}
}

func TestDivideRel(t *testing.T) {
	s := New(1, 2, 5, 6, 10)
	parts := s.DivideRel(func(a, b any) bool { d := a.(int) - b.(int); return d == 1 || d == -1 })
	var got [][]int
	for _, p := range parts {
		got = append(got, sortedInts(p))
	}
	sort.Slice(got, func(i, j int) bool { return got[i][0] < got[j][0] })
	want := [][]int{{1, 2}, {5, 6}, {10}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DivideRel = %v, want %v", got, want)
	}
	// Empty set: no components.
	if len(New().DivideRel(func(a, b any) bool { return true })) != 0 {
		t.Error("DivideRel(empty) not empty")
	}
	// Reverse-direction edge (rel(b,a) true) still connects.
	asym := New(1, 2).DivideRel(func(a, b any) bool { return a.(int) == 2 && b.(int) == 1 })
	if len(asym) != 1 {
		t.Errorf("DivideRel asymmetric components = %d, want 1", len(asym))
	}
}

func TestFlattenSet(t *testing.T) {
	s := New()
	s.Add(New(1, 2))
	s.Add(New(2, 3))
	s.Add(4)
	flat := s.FlattenSet()
	if g, w := sortedInts(flat), []int{1, 2, 3, 4}; !reflect.DeepEqual(g, w) {
		t.Errorf("FlattenSet = %v, want %v", g, w)
	}
	// Recursive nesting.
	deep := New()
	inner := New()
	inner.Add(New(7, 8))
	deep.Add(inner)
	deep.Add(9)
	if g, w := sortedInts(deep.FlattenSet()), []int{7, 8, 9}; !reflect.DeepEqual(g, w) {
		t.Errorf("FlattenSet deep = %v, want %v", g, w)
	}
}

func TestInspectAndString(t *testing.T) {
	if got := New(1, 2, 3).Inspect(func(e any) string { return fmt.Sprintf("%d", e) }); got != "Set[1, 2, 3]" {
		t.Errorf("Inspect = %q", got)
	}
	if got := New().Inspect(DefaultInspect); got != "Set[]" {
		t.Errorf("Inspect empty = %q", got)
	}
	if got := New(1).String(); got != "Set[1]" {
		t.Errorf("String = %q", got)
	}
	if got := DefaultInspect("x"); got != `"x"` {
		t.Errorf("DefaultInspect = %q", got)
	}
}

func TestSortedSlice(t *testing.T) {
	s := New(3, 1, 2)
	got := s.SortedSlice(func(a, b any) bool { return a.(int) < b.(int) })
	if !reflect.DeepEqual(got, []any{1, 2, 3}) {
		t.Errorf("SortedSlice = %v", got)
	}
	// Receiver unchanged (insertion order preserved).
	if !reflect.DeepEqual(asInts(s), []int{3, 1, 2}) {
		t.Error("SortedSlice mutated receiver")
	}
}

// TestHasher exercises the host-supplied identity: keying strings by content so
// two equal strings coincide, and a custom struct keyed by an id field.
func TestHasher(t *testing.T) {
	type node struct {
		id   int
		name string
	}
	byID := func(e any) any { return e.(node).id }
	s := NewWith(byID, node{1, "a"}, node{2, "b"}, node{1, "z"}) // id 1 dedups
	if s.Size() != 2 {
		t.Fatalf("Hasher dedup Size = %d, want 2", s.Size())
	}
	// The first-inserted member is retained.
	first := s.ToSlice()[0].(node)
	if first.name != "a" {
		t.Errorf("retained member = %+v, want name a", first)
	}
	if !s.Include(node{1, "anything"}) {
		t.Error("Include by id false")
	}
	// Derived sets keep the same Hasher.
	u := s.Union(NewWith(byID, node{3, "c"}))
	if u.Size() != 3 || !u.Include(node{3, "x"}) {
		t.Errorf("Union with Hasher = %d members", u.Size())
	}
}
