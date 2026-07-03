// Copyright (c) the go-ruby-set/set authors
//
// SPDX-License-Identifier: BSD-3-Clause

package set

import "maps"

// membership is the Set's presence index: it answers "is this identity key a
// member?" and nothing else (insertion order and the retained member values live
// in the Set's parallel slices). Its whole reason to exist is speed: a Ruby Set's
// members are keyed by an opaque identity key (see Hasher), so the obvious index
// is a map[any]struct{} — but a generic-any map hashes every key through the
// interface's type descriptor, roughly twice the cost of a map keyed by a
// concrete comparable type. Most real Sets are homogeneous (all Integer, or all
// String / Symbol), so membership keeps a TYPED map for the common concrete key
// types and only falls back to the generic map when a set actually holds a mix.
//
// # Exact semantics preserved
//
// The typed fast path must never change which members are considered equal. A
// generic map[any] distinguishes keys by (concrete type, value): any(1) (an int),
// any(int64(1)), any(1.0) (a float64) and any("1") are four different keys. The
// typed path reproduces this exactly by gating each fast path on ONE concrete
// type:
//
//   - kindInt   accepts only Go int    keys, stored as int64 in ints.
//   - kindInt64 accepts only Go int64  keys, stored as int64 in ints.
//   - kindStr   accepts only Go string keys, stored in strs.
//   - kindAny   the generic fallback, keys held as-is in anys.
//
// The int and int64 fast paths share the ints storage but are distinct kinds, so
// an int-keyed set that later sees an int64 key does NOT merge int(1) with
// int64(1): the mismatched concrete type forces migration to the generic map,
// where the two keys stay distinct — identical to the all-generic behaviour. The
// same holds for every other type transition (e.g. an Integer set that gains a
// String or Float member): the first key of a foreign type promotes the whole
// index to kindAny, re-boxing every existing key under its ORIGINAL concrete type
// so no member is lost or coalesced.
type membership struct {
	kind idxKind
	hint int // pre-size hint applied when the concrete map is first allocated
	ints map[int64]struct{}
	strs map[string]struct{}
	anys map[any]struct{}
}

// idxKind is the concrete-type discriminator of a membership index.
type idxKind uint8

const (
	kindEmpty idxKind = iota // no member yet; concrete map not allocated
	kindInt                  // homogeneous Go int    keys (stored as int64)
	kindInt64                // homogeneous Go int64  keys
	kindStr                  // homogeneous Go string keys
	kindAny                  // mixed / other-typed keys: generic map[any]
)

// newMembership returns an empty index that will pre-size its concrete map to
// hint members the first time one is added, so a build of a known size neither
// rehashes nor re-grows.
func newMembership(hint int) membership { return membership{hint: hint} }

// classify selects the fast path for the first key and allocates its map,
// pre-sized to at least hint (never below 1 so a single Add still gets a map).
func (m *membership) classify(key any) {
	n := m.hint
	if n < 1 {
		n = 1
	}
	switch key.(type) {
	case int:
		m.kind = kindInt
		m.ints = make(map[int64]struct{}, n)
	case int64:
		m.kind = kindInt64
		m.ints = make(map[int64]struct{}, n)
	case string:
		m.kind = kindStr
		m.strs = make(map[string]struct{}, n)
	default:
		m.kind = kindAny
		m.anys = make(map[any]struct{}, n)
	}
}

// migrate promotes a typed index to the generic kindAny map, re-boxing every
// existing key under its original concrete type so distinct-but-equal-looking
// keys stay distinct. Called the moment a key of a foreign concrete type arrives.
func (m *membership) migrate() {
	anys := make(map[any]struct{}, len(m.ints)+len(m.strs)+1)
	switch m.kind {
	case kindInt:
		for k := range m.ints {
			anys[int(k)] = struct{}{}
		}
	case kindInt64:
		for k := range m.ints {
			anys[k] = struct{}{}
		}
	case kindStr:
		for k := range m.strs {
			anys[k] = struct{}{}
		}
	}
	m.ints = nil
	m.strs = nil
	m.anys = anys
	m.kind = kindAny
}

// has reports whether key is a member. A key whose concrete type does not match a
// typed index's kind cannot be present (the index only ever admitted keys of its
// own type), so the mismatch answers false without touching the map.
func (m *membership) has(key any) bool {
	switch m.kind {
	case kindInt:
		v, ok := key.(int)
		if !ok {
			return false
		}
		_, ex := m.ints[int64(v)]
		return ex
	case kindInt64:
		v, ok := key.(int64)
		if !ok {
			return false
		}
		_, ex := m.ints[v]
		return ex
	case kindStr:
		v, ok := key.(string)
		if !ok {
			return false
		}
		_, ex := m.strs[v]
		return ex
	case kindAny:
		_, ex := m.anys[key]
		return ex
	default:
		return false
	}
}

// add inserts key and reports whether it was newly inserted (false if already
// present). It performs the dedup read and the insert on the fast path map when
// key matches the current kind, migrating to the generic map first on a type
// mismatch.
func (m *membership) add(key any) bool {
	switch m.kind {
	case kindInt:
		v, ok := key.(int)
		if !ok {
			m.migrate()
			return m.add(key)
		}
		if _, ex := m.ints[int64(v)]; ex {
			return false
		}
		m.ints[int64(v)] = struct{}{}
		return true
	case kindInt64:
		v, ok := key.(int64)
		if !ok {
			m.migrate()
			return m.add(key)
		}
		if _, ex := m.ints[v]; ex {
			return false
		}
		m.ints[v] = struct{}{}
		return true
	case kindStr:
		v, ok := key.(string)
		if !ok {
			m.migrate()
			return m.add(key)
		}
		if _, ex := m.strs[v]; ex {
			return false
		}
		m.strs[v] = struct{}{}
		return true
	case kindAny:
		if _, ex := m.anys[key]; ex {
			return false
		}
		m.anys[key] = struct{}{}
		return true
	default: // kindEmpty
		m.classify(key)
		return m.add(key)
	}
}

// putNew inserts key assuming it is not already present (the set-algebra
// combinators guarantee that by construction), skipping the dedup read. It still
// initialises the kind on the first key and migrates on a type mismatch, so a
// result set built from heterogeneous operands is indexed correctly.
func (m *membership) putNew(key any) {
	switch m.kind {
	case kindInt:
		v, ok := key.(int)
		if !ok {
			m.migrate()
			m.putNew(key)
			return
		}
		m.ints[int64(v)] = struct{}{}
	case kindInt64:
		v, ok := key.(int64)
		if !ok {
			m.migrate()
			m.putNew(key)
			return
		}
		m.ints[v] = struct{}{}
	case kindStr:
		v, ok := key.(string)
		if !ok {
			m.migrate()
			m.putNew(key)
			return
		}
		m.strs[v] = struct{}{}
	case kindAny:
		m.anys[key] = struct{}{}
	default: // kindEmpty
		m.classify(key)
		m.putNew(key)
	}
}

// del removes key from the index (a no-op if the key's type cannot be present).
func (m *membership) del(key any) {
	switch m.kind {
	case kindInt:
		if v, ok := key.(int); ok {
			delete(m.ints, int64(v))
		}
	case kindInt64:
		if v, ok := key.(int64); ok {
			delete(m.ints, v)
		}
	case kindStr:
		if v, ok := key.(string); ok {
			delete(m.strs, v)
		}
	case kindAny:
		delete(m.anys, key)
	}
}

// clone returns an independent copy of the index, using the runtime's bulk map
// clone (maps.Clone) for the active concrete map — dramatically faster than a
// key-by-key re-insert, and it copies at the map's exact size. A caller that then
// folds more keys in (Union) pays at most one re-grow, which is far cheaper than
// hand-copying every key into an over-sized map. The clone keeps kind and hint.
func (m *membership) clone() membership {
	out := membership{kind: m.kind, hint: m.hint}
	switch m.kind {
	case kindInt, kindInt64:
		out.ints = maps.Clone(m.ints)
	case kindStr:
		out.strs = maps.Clone(m.strs)
	case kindAny:
		out.anys = maps.Clone(m.anys)
	}
	return out
}
