// Copyright (c) the go-ruby-set/set authors
//
// SPDX-License-Identifier: BSD-3-Clause

package set

import (
	"reflect"
	"testing"
)

// order returns the members of s in insertion order, as-is (no type coercion),
// so a heterogeneous set's exact members and order can be asserted.
func order(s *Set) []any { return s.ToSlice() }

// TestKindInt exercises the homogeneous Go-int fast path end to end.
func TestKindInt(t *testing.T) {
	s := New(3, 1, 2, 1) // duplicate collapses
	if s.midx.kind != kindInt {
		t.Fatalf("kind = %d, want kindInt", s.midx.kind)
	}
	if s.Size() != 3 {
		t.Fatalf("Size = %d, want 3", s.Size())
	}
	if !s.Include(3) || s.Include(9) {
		t.Fatal("Include on int set wrong")
	}
	// A key of a foreign concrete type can never be present in an int-only set.
	if s.Include(int64(3)) || s.Include("3") || s.Include(3.0) {
		t.Fatal("int set matched a foreign-typed key")
	}
}

// TestKindInt64 exercises the homogeneous Go-int64 fast path, kept distinct from
// the int path so int(1) and int64(1) never coalesce.
func TestKindInt64(t *testing.T) {
	s := New(int64(10), int64(20), int64(10))
	if s.midx.kind != kindInt64 {
		t.Fatalf("kind = %d, want kindInt64", s.midx.kind)
	}
	if s.Size() != 2 {
		t.Fatalf("Size = %d, want 2", s.Size())
	}
	if !s.Include(int64(20)) || s.Include(int64(99)) {
		t.Fatal("Include on int64 set wrong")
	}
	if s.Include(10) { // Go int, not int64 — must miss
		t.Fatal("int64 set matched a Go int key")
	}
}

// TestKindString exercises the homogeneous string fast path.
func TestKindString(t *testing.T) {
	s := New("b", "a", "b", "c")
	if s.midx.kind != kindStr {
		t.Fatalf("kind = %d, want kindStr", s.midx.kind)
	}
	if got, want := order(s), []any{"b", "a", "c"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
	if !s.Include("a") || s.Include("z") {
		t.Fatal("Include on string set wrong")
	}
	if s.Include(1) { // foreign type
		t.Fatal("string set matched an int key")
	}
}

// TestKindAnyDirect exercises the generic fallback for a comparable type that has
// no dedicated fast path (bool here), including via a first-element default.
func TestKindAnyDirect(t *testing.T) {
	s := New(true, false, true)
	if s.midx.kind != kindAny {
		t.Fatalf("kind = %d, want kindAny", s.midx.kind)
	}
	if s.Size() != 2 {
		t.Fatalf("Size = %d, want 2", s.Size())
	}
	if !s.Include(true) || !s.Include(false) || s.Include(0) {
		t.Fatal("Include on generic set wrong")
	}
}

// TestEmptyKind covers membership queries on a never-populated (kindEmpty) index.
func TestEmptyKind(t *testing.T) {
	s := New()
	if s.midx.kind != kindEmpty {
		t.Fatalf("kind = %d, want kindEmpty", s.midx.kind)
	}
	if s.Include(1) || s.Include("x") {
		t.Fatal("empty set reported a member")
	}
	// Delete / DeleteQ on empty are no-ops that must not panic.
	if s.DeleteQ(1) {
		t.Fatal("DeleteQ on empty returned true")
	}
	s.Delete(1)
}

// TestMigrateIntToInt64 is the core heterogeneous-safety case: a set that starts
// homogeneous (int) then gains an int64 must NOT merge int(1) with int64(1); it
// switches to the generic index while keeping both distinct members in order.
func TestMigrateIntToInt64(t *testing.T) {
	s := New(1, 2)
	if s.midx.kind != kindInt {
		t.Fatalf("pre-migrate kind = %d, want kindInt", s.midx.kind)
	}
	if !s.AddQ(int64(1)) { // same numeric value, different Go type — new member
		t.Fatal("AddQ(int64(1)) should be a new member")
	}
	if s.midx.kind != kindAny {
		t.Fatalf("post-migrate kind = %d, want kindAny", s.midx.kind)
	}
	if s.Size() != 3 {
		t.Fatalf("Size = %d, want 3 (1, 2, int64(1))", s.Size())
	}
	if got, want := order(s), []any{1, 2, int64(1)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %v, want %v (no member lost or merged)", got, want)
	}
	if !s.Include(1) || !s.Include(int64(1)) || !s.Include(2) {
		t.Fatal("a member was lost across migration")
	}
	if s.AddQ(1) || s.AddQ(int64(1)) { // both already present post-migration
		t.Fatal("dedup broke after migration")
	}
}

// TestMigrateIntToFloatAndString drives the other type transitions and checks
// that 1 (int), 1.0 (float64) and "1" (string) are three distinct members.
func TestMigrateIntToFloatAndString(t *testing.T) {
	s := New(1)
	s.Add(1.0) // float64 — migrates int -> any
	s.Add("1") // string on a now-generic set
	if s.Size() != 3 {
		t.Fatalf("Size = %d, want 3", s.Size())
	}
	if got, want := order(s), []any{1, 1.0, "1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
}

// TestMigrateInt64ToInt covers the int64 -> generic transition (foreign int key).
func TestMigrateInt64ToInt(t *testing.T) {
	s := New(int64(5))
	s.Add(5) // Go int — migrates int64 -> any
	if s.midx.kind != kindAny {
		t.Fatalf("kind = %d, want kindAny", s.midx.kind)
	}
	if got, want := order(s), []any{int64(5), 5}; !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
}

// TestMigrateStringToInt covers the string -> generic transition.
func TestMigrateStringToInt(t *testing.T) {
	s := New("a")
	s.Add(1) // migrates str -> any
	if s.midx.kind != kindAny {
		t.Fatalf("kind = %d, want kindAny", s.midx.kind)
	}
	if got, want := order(s), []any{"a", 1}; !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
	if !s.Include("a") || !s.Include(1) {
		t.Fatal("member lost across str->any migration")
	}
}

// TestDeleteAcrossKinds exercises del on each typed path, a mismatched-type
// delete (no-op), and delete after migration.
func TestDeleteAcrossKinds(t *testing.T) {
	// int path
	si := New(1, 2, 3)
	if !si.DeleteQ(2) || si.DeleteQ(2) {
		t.Fatal("int DeleteQ wrong")
	}
	si.Delete(int64(1)) // foreign type: no-op, member 1 stays
	if !si.Include(1) {
		t.Fatal("foreign-typed delete removed a member")
	}
	// int64 path
	s64 := New(int64(1), int64(2))
	if !s64.DeleteQ(int64(1)) {
		t.Fatal("int64 DeleteQ wrong")
	}
	s64.Delete(1) // foreign no-op
	if !s64.Include(int64(2)) {
		t.Fatal("int64 member lost")
	}
	// string path
	ss := New("a", "b")
	if !ss.DeleteQ("a") {
		t.Fatal("string DeleteQ wrong")
	}
	ss.Delete(1) // foreign no-op
	// generic path after migration
	sm := New(1)
	sm.Add(int64(1))
	if !sm.DeleteQ(int64(1)) || !sm.Include(1) {
		t.Fatal("generic delete wrong")
	}
}

// TestDupPerKind covers clone on every kind.
func TestDupPerKind(t *testing.T) {
	cases := []*Set{
		New(1, 2, 3),
		New(int64(1), int64(2)),
		New("a", "b"),
		New(true, false),
		New(),
	}
	for i, s := range cases {
		d := s.Dup()
		if !d.EqualQ(s) || d.Size() != s.Size() {
			t.Fatalf("case %d: Dup not equal", i)
		}
		// Mutating the dup must not touch the original.
		d.Add(99)
		if s.Include(99) {
			t.Fatalf("case %d: Dup shares state with original", i)
		}
	}
}

// TestUnionPerKind covers the maps.Clone-backed Union clone on each kind, plus a
// union whose operands force a migration in the result.
func TestUnionPerKind(t *testing.T) {
	if got := New(int64(1), int64(2)).Union(New(int64(2), int64(3))); got.Size() != 3 {
		t.Fatalf("int64 union size = %d, want 3", got.Size())
	}
	if got := New("a", "b").Union(New("b", "c")); got.Size() != 3 {
		t.Fatalf("string union size = %d, want 3", got.Size())
	}
	if got := New(true).Union(New(false)); got.Size() != 2 {
		t.Fatalf("generic union size = %d, want 2", got.Size())
	}
	// Result starts int (from receiver) and must migrate when other's int64 folds in.
	mixed := New(1, 2).Union(New(int64(1)))
	if mixed.Size() != 3 || mixed.midx.kind != kindAny {
		t.Fatalf("mixed union: size=%d kind=%d, want 3/kindAny", mixed.Size(), mixed.midx.kind)
	}
}

// TestAlgebraPutNewMigration drives the putNew fast paths and their migration
// branch through the set-algebra combinators on typed and mixed operands.
func TestAlgebraPutNewMigration(t *testing.T) {
	// int64 intersection/difference/xor exercise putNew on the int64 path.
	a := New(int64(1), int64(2), int64(3))
	b := New(int64(2), int64(3), int64(4))
	if a.Intersection(b).Size() != 2 {
		t.Fatal("int64 intersection wrong")
	}
	if a.Difference(b).Size() != 1 {
		t.Fatal("int64 difference wrong")
	}
	if a.XorSym(b).Size() != 2 {
		t.Fatal("int64 xor wrong")
	}
	// string path through Select (putNew on strings).
	ss := New("aa", "b", "cc")
	if got := ss.Select(func(e any) bool { return len(e.(string)) == 2 }); got.Size() != 2 {
		t.Fatal("string select wrong")
	}
	// A heterogeneous receiver makes the algebra result migrate via putNew.
	het := New(1)
	het.Add(int64(1)) // now kindAny with members 1, int64(1)
	if dup := het.Union(New(2)); dup.Size() != 3 || dup.midx.kind != kindAny {
		t.Fatalf("heterogeneous union size=%d kind=%d, want 3/kindAny", dup.Size(), dup.midx.kind)
	}
	// Union results that start typed (int64 / string receiver) then fold a
	// foreign-typed key drive the int64->migrate and str->migrate putNew branches.
	if u := New(int64(1)).Union(New(1)); u.Size() != 2 || u.midx.kind != kindAny {
		t.Fatalf("int64->foreign union size=%d kind=%d, want 2/kindAny", u.Size(), u.midx.kind)
	}
	if u := New("a").Union(New(1)); u.Size() != 2 || u.midx.kind != kindAny {
		t.Fatalf("str->foreign union size=%d kind=%d, want 2/kindAny", u.Size(), u.midx.kind)
	}
}
