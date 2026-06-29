<p align="center"><img src="https://raw.githubusercontent.com/go-ruby-set/brand/main/social/go-ruby-set-set.png" alt="go-ruby-set/set" width="720"></p>

# set — go-ruby-set

[![Docs](https://img.shields.io/badge/docs-mkdocs--material-DC2626)](https://go-ruby-set.github.io/docs/)
[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26.4%2B-00ADD8)](https://go.dev/dl/)
[![Coverage](https://img.shields.io/badge/coverage-100%25-1a7f37)](#tests--coverage)

**A pure-Go (no cgo) reimplementation of Ruby's [`Set`](https://docs.ruby-lang.org/en/master/Set.html)**
— the unordered, unique collection with full set algebra from MRI 4.0.5's `set`
standard library (autoloaded core in Ruby 4.0). It mirrors `Set`'s observable
behaviour — insertion-ordered iteration, `add?`/`delete?`, `|` `&` `-` `^`,
subset/superset predicates, `classify` / `divide` / `group_by` / `flatten` —
**without any Ruby runtime**.

It is the `Set` backend for
[go-embedded-ruby](https://github.com/go-embedded-ruby/ruby), but is a
**standalone, reusable** module with no dependency on the Ruby runtime — a sibling
of [go-ruby-yaml](https://github.com/go-ruby-yaml/yaml) (the Psych emitter/loader)
and [go-ruby-bigdecimal](https://github.com/go-ruby-bigdecimal/bigdecimal).

> **MRI-faithful, not Composition-Oriented.** This is the *stdlib* `Set` — the same
> philosophy distinction as [go-ruby-bigdecimal](https://github.com/go-ruby-bigdecimal/bigdecimal)
> vs `go-composites/bigfloat`. It is intentionally distinct from
> [`go-composites/set`](https://github.com/go-composites/set), which is a
> Composition-Oriented set with a different design. Reach for this one when you
> want Ruby `Set` semantics; reach for the composite when you want composition.

> **SortedSet.** MRI 4.0 **dropped `SortedSet` from core** (it was removed from the
> `set` library), so this package does not ship a `SortedSet` type. The faithful
> equivalent of Ruby's `Set#sort` is [`SortedSlice`](#api), which returns the
> members ordered by a comparator without mutating the set.

## Element identity — the host hash/eql plug

In MRI a `Set` keys its members by Ruby's `hash` / `eql?` protocol: two distinct
`String` objects with the same bytes are the **same** member, while a `Symbol` of
the same name is a **different** member. Go cannot know those semantics, so the
host supplies them through a **`Hasher`** — a function mapping a member to the
comparable Go key under which two members are considered equal:

```go
type Hasher func(elem any) any
```

go-embedded-ruby plugs its own object hashing here, exactly as it does for `Hash`
keys. A `Set` built with `New` (no `Hasher`) keys members **by themselves** — handy
for plain Go data, but the members must be comparable. A `Set` built with
`NewWith(hasher, …)` keys members **through the host function** and accepts any
value. Iteration always preserves first-insertion order, as MRI does.

## Install

```sh
go get github.com/go-ruby-set/set
```

## Usage

```go
package main

import (
	"fmt"

	"github.com/go-ruby-set/set"
)

func main() {
	a := set.New(1, 2, 3, 4)
	b := set.New(3, 4, 5, 6)

	fmt.Println(a.Union(b))        // Set[1, 2, 3, 4, 5, 6]
	fmt.Println(a.Intersection(b)) // Set[3, 4]
	fmt.Println(a.Difference(b))   // Set[1, 2]
	fmt.Println(a.XorSym(b))       // Set[1, 2, 5, 6]
	fmt.Println(a.SubsetQ(set.New(1, 2, 3, 4, 5))) // true

	// classify: Hash{ block-value => Set }
	for _, g := range set.New(1, 2, 3, 4, 5, 6).
		Classify(func(e any) any { return e.(int) % 3 }).Groups() {
		fmt.Printf("%v => %v\n", g.Value, g.Set)
	}

	// A host (go-embedded-ruby) supplies Ruby hash/eql? via a Hasher:
	byContent := func(e any) any { return fmt.Sprintf("%v", e) }
	s := set.NewWith(byContent, "a", "a", "b") // the two "a"s coincide
	fmt.Println(s.Size())                       // 2
}
```

## API

```go
type Hasher func(elem any) any

func New(elems ...any) *Set                 // identity-keyed (comparable members)
func NewWith(h Hasher, elems ...any) *Set   // host hash/eql? semantics

// membership & mutation
func (s *Set) Add(elem any) *Set            // add / <<
func (s *Set) AddQ(elem any) bool           // add?  (true if newly inserted)
func (s *Set) Delete(elem any) *Set         // delete
func (s *Set) DeleteQ(elem any) bool        // delete? (true if was present)
func (s *Set) Include(elem any) bool        // include? / member? / ===
func (s *Set) Size() int                    // size / length / count
func (s *Set) Empty() bool                  // empty?
func (s *Set) Clear() *Set                  // clear
func (s *Set) Each(fn func(any) error) error
func (s *Set) ToSlice() []any               // to_a (insertion order)
func (s *Set) Dup() *Set                    // dup / clone
func (s *Set) Merge(others ...*Set) *Set    // merge
func (s *Set) MergeSlice(elems []any) *Set
func (s *Set) Subtract(other *Set) *Set     // subtract
func (s *Set) SubtractSlice(elems []any) *Set

// set algebra
func (s *Set) Union(other *Set) *Set        // | / + / union
func (s *Set) Intersection(other *Set) *Set // & / intersection
func (s *Set) Difference(other *Set) *Set   // - / difference
func (s *Set) XorSym(other *Set) *Set       // ^ (symmetric difference)
func (s *Set) SubsetQ(other *Set) bool      // subset? / <=
func (s *Set) ProperSubsetQ(other *Set) bool   // proper_subset? / <
func (s *Set) SupersetQ(other *Set) bool       // superset? / >=
func (s *Set) ProperSupersetQ(other *Set) bool // proper_superset? / >
func (s *Set) DisjointQ(other *Set) bool    // disjoint?
func (s *Set) IntersectQ(other *Set) bool   // intersect?
func (s *Set) EqualQ(other *Set) bool       // ==

// enumeration & higher-order
func (s *Set) Map(fn func(any) any) []any        // map / collect (-> Array)
func (s *Set) Select(fn func(any) bool) *Set     // select / filter
func (s *Set) Reject(fn func(any) bool) *Set     // reject
func (s *Set) CollectBang(fn func(any) any) *Set // collect! / map! (in place)
func (s *Set) Classify(fn func(any) any) *ClassifyResult // Hash{v => Set}
func (s *Set) GroupBy(fn func(any) any) *GroupByResult   // Hash{v => Array}
func (s *Set) Divide(fn func(any) any) []*Set            // divide {|x| ...}
func (s *Set) DivideRel(rel func(a, b any) bool) []*Set  // divide {|i,j| ...}
func (s *Set) FlattenSet() *Set                          // flatten (recursive)
func (s *Set) SortedSlice(less func(a, b any) bool) []any // sort -> Array
func (s *Set) Inspect(stringFn func(any) string) string  // "Set[1, 2, 3]"
func (s *Set) String() string
```

`Map` / `Collect` and `SortedSlice` return slices, matching MRI (`Set#map` and
`Set#sort` return an `Array`, not a `Set`). `Classify` and `GroupBy` return
insertion-ordered result objects (`Groups()` / `Buckets()`), matching MRI's
`Hash` insertion ordering; `Classify` buckets hold `Set`s, `GroupBy` buckets hold
slices, exactly as Ruby's `Set#classify` and `Enumerable#group_by` do. `Divide`
takes the 1-argument block form (partition by block value); `DivideRel` takes the
2-argument relation form (transitive-closure connected components).

## Tests & coverage

The suite pairs deterministic, ruby-free tests (which alone hold coverage at
**100%**, so the qemu cross-arch and Windows lanes pass the gate) with a
**differential MRI oracle**: set algebra, subset/superset/disjoint predicates,
`classify`, both `divide` forms, and the `Set[…]` inspect are computed here and
checked against the system `ruby`. The oracle is gated on `RUBY_VERSION >= "4.0"`
(where `Set` is core and renders as `Set[…]`), binmodes stdin/stdout so Windows
text-mode never pollutes the bytes, and skips itself where `ruby` is absent.

```sh
COVERPKG=$(go list ./... | paste -sd, -)
go test -race -coverpkg="$COVERPKG" -coverprofile=cover.out ./...
go tool cover -func=cover.out | tail -1   # 100.0%
```

CGO-free, dependency-free, `gofmt` + `go vet` clean, and green across the six
64-bit Go targets (amd64, arm64, riscv64, loong64, ppc64le, s390x) and three OSes
(Linux, macOS, Windows).

## License

BSD-3-Clause — see [LICENSE](LICENSE). Copyright the go-ruby-set/set authors.
