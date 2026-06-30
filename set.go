// Copyright (c) the go-ruby-set/set authors
//
// SPDX-License-Identifier: BSD-3-Clause

// Package set is a pure-Go (no cgo) reimplementation of Ruby's MRI 4.0.5 Set —
// an unordered collection of unique members with full set algebra, mirroring the
// behaviour of the `set` standard library (autoloaded core in Ruby 4.0).
//
// # Element identity
//
// In MRI a Set keys its members by Ruby's hash / eql? protocol, so two distinct
// String objects with the same bytes are the *same* member, while a Symbol of the
// same name is a *different* member. Go cannot know those semantics, so the host
// supplies them through a Hasher: a function mapping a member to a comparable Go
// key under which two members are considered equal. go-embedded-ruby plugs its
// own object hashing here, exactly as it does for Hash keys.
//
// A Set built with New (no Hasher) keys members by themselves and therefore only
// accepts comparable members — convenient for plain Go programs. A Set built with
// NewWith(hasher) keys members through the host function and accepts any value.
//
// # Ordering
//
// Like MRI, iteration (Each / ToSlice / Inspect) preserves first-insertion order.
// The set algebra keeps the receiver's order first, then the argument's, so the
// results match MRI's observable ordering.
package set

import (
	"fmt"
	"sort"
	"strings"
)

// Hasher maps a member to the comparable key under which membership and equality
// are decided. Two members are the same iff their keys are ==. The host (e.g.
// go-embedded-ruby) supplies Ruby hash / eql? semantics here; a nil Hasher keys a
// member by itself (the member must then be comparable, or operations panic, just
// as a Go map does on an uncomparable key).
type Hasher func(elem any) any

// Set is an MRI-faithful Ruby Set: an unordered collection of unique members with
// set algebra, iterated in first-insertion order. The zero value is not usable;
// construct one with New or NewWith.
type Set struct {
	hash  Hasher      // member -> comparable key (nil => key by self)
	vals  map[any]any // key -> member (the canonical retained member)
	order []any       // keys in insertion order, for Ruby iteration ordering
}

// key reduces a member to its comparable identity key.
func (s *Set) key(elem any) any {
	if s.hash == nil {
		return elem
	}
	return s.hash(elem)
}

// New returns a Set keyed by member identity (members must be comparable),
// seeded with the given members. It is the convenient form for plain Go data.
func New(elems ...any) *Set {
	return NewWith(nil, elems...)
}

// NewWith returns a Set whose membership and equality are decided by h, seeded
// with the given members. A nil h keys members by themselves (like New). This is
// the form a host with Ruby hash / eql? semantics uses.
func NewWith(h Hasher, elems ...any) *Set {
	s := &Set{hash: h, vals: make(map[any]any)}
	for _, e := range elems {
		s.Add(e)
	}
	return s
}

// withSameHasher returns a new empty Set sharing the receiver's Hasher, so a
// derived Set keys members identically.
func (s *Set) withSameHasher() *Set {
	return &Set{hash: s.hash, vals: make(map[any]any)}
}

// Size returns the number of members (Ruby Set#size / #length / #count).
func (s *Set) Size() int { return len(s.order) }

// Empty reports whether the Set has no members (Ruby Set#empty?).
func (s *Set) Empty() bool { return len(s.order) == 0 }

// Add inserts elem, returning the Set for chaining (Ruby Set#add / #<<). Adding a
// member already present is a no-op that preserves the original member.
func (s *Set) Add(elem any) *Set {
	k := s.key(elem)
	if _, ok := s.vals[k]; !ok {
		s.vals[k] = elem
		s.order = append(s.order, k)
	}
	return s
}

// AddQ adds elem and reports whether it was newly inserted (Ruby Set#add?: self
// when added, nil when already present — modelled here as a bool).
func (s *Set) AddQ(elem any) bool {
	k := s.key(elem)
	if _, ok := s.vals[k]; ok {
		return false
	}
	s.vals[k] = elem
	s.order = append(s.order, k)
	return true
}

// Delete removes elem, returning the Set for chaining (Ruby Set#delete). Deleting
// an absent member is a no-op.
func (s *Set) Delete(elem any) *Set {
	s.deleteKey(s.key(elem))
	return s
}

// DeleteQ removes elem and reports whether it was present (Ruby Set#delete?: self
// when removed, nil when absent — modelled here as a bool).
func (s *Set) DeleteQ(elem any) bool {
	k := s.key(elem)
	if _, ok := s.vals[k]; !ok {
		return false
	}
	s.deleteKey(k)
	return true
}

// deleteKey removes a key from both the map and the insertion order.
func (s *Set) deleteKey(k any) {
	if _, ok := s.vals[k]; !ok {
		return
	}
	delete(s.vals, k)
	for i, ok := range s.order {
		if ok == k {
			s.order = append(s.order[:i], s.order[i+1:]...)
			break
		}
	}
}

// Include reports whether elem is a member (Ruby Set#include? / #member? / #===).
func (s *Set) Include(elem any) bool {
	_, ok := s.vals[s.key(elem)]
	return ok
}

// Clear removes every member, returning the Set for chaining (Ruby Set#clear).
func (s *Set) Clear() *Set {
	s.vals = make(map[any]any)
	s.order = nil
	return s
}

// Each calls fn for every member in insertion order (Ruby Set#each). Returning a
// non-nil error from fn stops iteration and propagates it.
func (s *Set) Each(fn func(elem any) error) error {
	for _, k := range s.order {
		if err := fn(s.vals[k]); err != nil {
			return err
		}
	}
	return nil
}

// ToSlice returns the members as a slice in insertion order (Ruby Set#to_a).
func (s *Set) ToSlice() []any {
	out := make([]any, len(s.order))
	for i, k := range s.order {
		out[i] = s.vals[k]
	}
	return out
}

// EachPair calls fn for every member in insertion order with both its identity
// key and the stored member value, in a single pass over the internal tables —
// no per-element membership re-lookup. A host that retains its own per-member
// data keyed by the identity key (as go-embedded-ruby does for the original Ruby
// value) can consume an algebra result with EachPair to rebuild its view in one
// pass, instead of re-deriving each member's key and re-testing Include. The
// members are exactly those yielded by Each / ToSlice, in the same order.
func (s *Set) EachPair(fn func(key, member any)) {
	for _, k := range s.order {
		fn(k, s.vals[k])
	}
}

// Dup returns a shallow copy with the same members in the same order and the same
// Hasher (Ruby Set#dup / #clone).
func (s *Set) Dup() *Set {
	out := s.withSameHasher()
	for _, k := range s.order {
		out.vals[k] = s.vals[k]
	}
	out.order = append(out.order, s.order...)
	return out
}

// Merge adds every member of each other Set, returning the receiver for chaining
// (Ruby Set#merge). The receiver is mutated.
func (s *Set) Merge(others ...*Set) *Set {
	for _, o := range others {
		for _, k := range o.order {
			s.Add(o.vals[k])
		}
	}
	return s
}

// MergeSlice adds every element of the slice, returning the receiver for chaining
// (Ruby Set#merge accepts any enumerable; this is the slice form).
func (s *Set) MergeSlice(elems []any) *Set {
	for _, e := range elems {
		s.Add(e)
	}
	return s
}

// Subtract removes every member of other, returning the receiver for chaining
// (Ruby Set#subtract). The receiver is mutated.
func (s *Set) Subtract(other *Set) *Set {
	for _, k := range other.order {
		s.deleteKey(k)
	}
	return s
}

// SubtractSlice removes every element of the slice, returning the receiver for
// chaining (Ruby Set#subtract accepts any enumerable; this is the slice form).
func (s *Set) SubtractSlice(elems []any) *Set {
	for _, e := range elems {
		s.deleteKey(s.key(e))
	}
	return s
}

// Union returns a new Set with the members of the receiver and other (Ruby
// Set#| / #+ / #union). The receiver's order comes first, then other's.
func (s *Set) Union(other *Set) *Set {
	out := s.Dup()
	for _, k := range other.order {
		out.Add(other.vals[k])
	}
	return out
}

// Intersection returns a new Set with the members in both (Ruby Set#& /
// #intersection). Order follows the receiver.
func (s *Set) Intersection(other *Set) *Set {
	out := s.withSameHasher()
	for _, k := range s.order {
		if _, ok := other.vals[k]; ok {
			out.vals[k] = s.vals[k]
			out.order = append(out.order, k)
		}
	}
	return out
}

// Difference returns a new Set with the receiver's members not in other (Ruby
// Set#- / #difference). Order follows the receiver.
func (s *Set) Difference(other *Set) *Set {
	out := s.withSameHasher()
	for _, k := range s.order {
		if _, ok := other.vals[k]; !ok {
			out.vals[k] = s.vals[k]
			out.order = append(out.order, k)
		}
	}
	return out
}

// XorSym returns a new Set with the members in exactly one of the two (Ruby
// Set#^, the symmetric difference (s | other) - (s & other)).
func (s *Set) XorSym(other *Set) *Set {
	out := s.withSameHasher()
	for _, k := range s.order {
		if _, ok := other.vals[k]; !ok {
			out.vals[k] = s.vals[k]
			out.order = append(out.order, k)
		}
	}
	for _, k := range other.order {
		if _, ok := s.vals[k]; !ok {
			out.vals[k] = other.vals[k]
			out.order = append(out.order, k)
		}
	}
	return out
}

// SubsetQ reports whether every member of the receiver is in other (Ruby
// Set#subset? / #<=).
func (s *Set) SubsetQ(other *Set) bool {
	if len(s.order) > len(other.order) {
		return false
	}
	for _, k := range s.order {
		if _, ok := other.vals[k]; !ok {
			return false
		}
	}
	return true
}

// ProperSubsetQ reports whether the receiver is a subset of other and not equal
// to it (Ruby Set#proper_subset? / #<).
func (s *Set) ProperSubsetQ(other *Set) bool {
	return len(s.order) < len(other.order) && s.SubsetQ(other)
}

// SupersetQ reports whether every member of other is in the receiver (Ruby
// Set#superset? / #>=).
func (s *Set) SupersetQ(other *Set) bool {
	return other.SubsetQ(s)
}

// ProperSupersetQ reports whether the receiver is a superset of other and not
// equal to it (Ruby Set#proper_superset? / #>).
func (s *Set) ProperSupersetQ(other *Set) bool {
	return other.ProperSubsetQ(s)
}

// DisjointQ reports whether the two sets share no member (Ruby Set#disjoint?).
func (s *Set) DisjointQ(other *Set) bool {
	small, large := s, other
	if len(large.order) < len(small.order) {
		small, large = large, small
	}
	for _, k := range small.order {
		if _, ok := large.vals[k]; ok {
			return false
		}
	}
	return true
}

// IntersectQ reports whether the two sets share at least one member (Ruby
// Set#intersect?), the negation of DisjointQ.
func (s *Set) IntersectQ(other *Set) bool {
	return !s.DisjointQ(other)
}

// EqualQ reports whether the two sets have the same members (Ruby Set#==).
func (s *Set) EqualQ(other *Set) bool {
	if len(s.order) != len(other.order) {
		return false
	}
	for _, k := range s.order {
		if _, ok := other.vals[k]; !ok {
			return false
		}
	}
	return true
}

// Map applies fn to each member in insertion order and returns the results as a
// slice (Ruby Set#map / #collect returns an Array, not a Set).
func (s *Set) Map(fn func(elem any) any) []any {
	out := make([]any, 0, len(s.order))
	for _, k := range s.order {
		out = append(out, fn(s.vals[k]))
	}
	return out
}

// Select returns a new Set of the members for which fn is true (Ruby Set#select /
// #filter).
func (s *Set) Select(fn func(elem any) bool) *Set {
	out := s.withSameHasher()
	for _, k := range s.order {
		if fn(s.vals[k]) {
			out.vals[k] = s.vals[k]
			out.order = append(out.order, k)
		}
	}
	return out
}

// Reject returns a new Set of the members for which fn is false (Ruby Set#reject).
func (s *Set) Reject(fn func(elem any) bool) *Set {
	return s.Select(func(e any) bool { return !fn(e) })
}

// CollectBang replaces every member with fn(member) in place, returning the
// receiver for chaining (Ruby Set#collect! / #map!). The set is rebuilt, so two
// members mapping to the same key collapse to one.
func (s *Set) CollectBang(fn func(elem any) any) *Set {
	old := s.ToSlice()
	s.Clear()
	for _, e := range old {
		s.Add(fn(e))
	}
	return s
}

// Classify groups the members by fn(member), returning a map from each block
// value's key to the Set of members that produced it (Ruby Set#classify returns a
// Hash{value => Set}). The returned map is keyed by the *key* of each block value
// (under the receiver's Hasher) so distinct-but-equal Ruby values coincide; the
// ClassifyResult also records the original block value for each group.
func (s *Set) Classify(fn func(elem any) any) *ClassifyResult {
	res := &ClassifyResult{order: nil, groups: make(map[any]*ClassGroup)}
	for _, k := range s.order {
		elem := s.vals[k]
		bv := fn(elem)
		bk := s.key(bv)
		g, ok := res.groups[bk]
		if !ok {
			g = &ClassGroup{Value: bv, Set: s.withSameHasher()}
			res.groups[bk] = g
			res.order = append(res.order, bk)
		}
		g.Set.Add(elem)
	}
	return res
}

// ClassGroup is one bucket of a Classify / GroupBy result: the block value that
// labels the bucket and the members assigned to it.
type ClassGroup struct {
	Value any
	Set   *Set
}

// ClassifyResult is the insertion-ordered Hash{value => Set} a Classify returns.
type ClassifyResult struct {
	order  []any // block-value keys, in first-seen order
	groups map[any]*ClassGroup
}

// Len returns the number of buckets.
func (r *ClassifyResult) Len() int { return len(r.order) }

// Groups returns the buckets in first-seen order (Ruby Hash insertion order).
func (r *ClassifyResult) Groups() []*ClassGroup {
	out := make([]*ClassGroup, len(r.order))
	for i, k := range r.order {
		out[i] = r.groups[k]
	}
	return out
}

// Get returns the Set for block value bv and whether it exists.
func (r *ClassifyResult) Get(bv any, h Hasher) (*Set, bool) {
	var k any = bv
	if h != nil {
		k = h(bv)
	}
	g, ok := r.groups[k]
	if !ok {
		return nil, false
	}
	return g.Set, true
}

// GroupBy groups the members by fn(member) like Classify, but each bucket holds a
// slice in insertion order rather than a Set (Ruby Enumerable#group_by returns a
// Hash{value => Array}).
func (s *Set) GroupBy(fn func(elem any) any) *GroupByResult {
	res := &GroupByResult{groups: make(map[any]*GroupByBucket)}
	for _, k := range s.order {
		elem := s.vals[k]
		bv := fn(elem)
		bk := s.key(bv)
		g, ok := res.groups[bk]
		if !ok {
			g = &GroupByBucket{Value: bv}
			res.groups[bk] = g
			res.order = append(res.order, bk)
		}
		g.Members = append(g.Members, elem)
	}
	return res
}

// GroupByBucket is one bucket of a GroupBy result.
type GroupByBucket struct {
	Value   any
	Members []any
}

// GroupByResult is the insertion-ordered Hash{value => Array} a GroupBy returns.
type GroupByResult struct {
	order  []any
	groups map[any]*GroupByBucket
}

// Len returns the number of buckets.
func (r *GroupByResult) Len() int { return len(r.order) }

// Buckets returns the buckets in first-seen order.
func (r *GroupByResult) Buckets() []*GroupByBucket {
	out := make([]*GroupByBucket, len(r.order))
	for i, k := range r.order {
		out[i] = r.groups[k]
	}
	return out
}

// Divide partitions the members into a Set of Sets by the single-argument
// relation fn: members sharing the same fn value land in the same block (Ruby
// Set#divide with an arity-1 block). For the transitive-closure (binary) form,
// use DivideRel.
func (s *Set) Divide(fn func(elem any) any) []*Set {
	res := s.Classify(fn)
	groups := res.Groups()
	out := make([]*Set, len(groups))
	for i, g := range groups {
		out[i] = g.Set
	}
	return out
}

// DivideRel partitions the members into connected components of the graph whose
// edges are the pairs (a, b) for which rel(a, b) is true (Ruby Set#divide with an
// arity-2 block — a transitive-closure partition). rel is treated as symmetric:
// an edge a—b is added when rel(a, b) or rel(b, a) holds.
func (s *Set) DivideRel(rel func(a, b any) bool) []*Set {
	elems := s.ToSlice()
	n := len(elems)
	parent := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(x int) int {
		for parent[x] != x {
			parent[x] = parent[parent[x]]
			x = parent[x]
		}
		return x
	}
	union := func(a, b int) { parent[find(a)] = find(b) }
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if rel(elems[i], elems[j]) || rel(elems[j], elems[i]) {
				union(i, j)
			}
		}
	}
	// Build a component Set per root, in first-member order.
	byRoot := make(map[int]*Set)
	var order []int
	for i := 0; i < n; i++ {
		r := find(i)
		c, ok := byRoot[r]
		if !ok {
			c = s.withSameHasher()
			byRoot[r] = c
			order = append(order, r)
		}
		c.Add(elems[i])
	}
	out := make([]*Set, len(order))
	for i, r := range order {
		out[i] = byRoot[r]
	}
	return out
}

// FlattenSet returns a new Set in which every member that is itself a *Set is
// replaced by its members, recursively (Ruby Set#flatten). The receiver is
// unchanged; the result uses the receiver's Hasher.
func (s *Set) FlattenSet() *Set {
	out := s.withSameHasher()
	var walk func(cur *Set)
	walk = func(cur *Set) {
		for _, k := range cur.order {
			if nested, ok := cur.vals[k].(*Set); ok {
				walk(nested)
			} else {
				out.Add(cur.vals[k])
			}
		}
	}
	walk(s)
	return out
}

// Inspect renders the Set in MRI 4.0 form: "Set[1, 2, 3]" (an empty set is
// "Set[]"), members in insertion order via stringFn. stringFn produces each
// member's Ruby inspect; pass DefaultInspect for Go's %#v rendering.
func (s *Set) Inspect(stringFn func(elem any) string) string {
	var b strings.Builder
	b.WriteString("Set[")
	for i, k := range s.order {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(stringFn(s.vals[k]))
	}
	b.WriteByte(']')
	return b.String()
}

// DefaultInspect renders a member with Go's %#v, a reasonable default when the
// host supplies no Ruby inspect.
func DefaultInspect(elem any) string { return fmt.Sprintf("%#v", elem) }

// String renders the Set with DefaultInspect, satisfying fmt.Stringer.
func (s *Set) String() string { return s.Inspect(DefaultInspect) }

// SortedSlice returns the members sorted by less, leaving the Set unchanged. MRI
// 4.0 dropped the SortedSet class from core (Set#sort returns an Array), so this
// is the faithful equivalent of Ruby's Set#sort.
func (s *Set) SortedSlice(less func(a, b any) bool) []any {
	out := s.ToSlice()
	sort.SliceStable(out, func(i, j int) bool { return less(out[i], out[j]) })
	return out
}
