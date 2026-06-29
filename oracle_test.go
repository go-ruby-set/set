// Copyright (c) the go-ruby-set/set authors
//
// SPDX-License-Identifier: BSD-3-Clause

package set

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"testing"
)

// rubyBin locates a usable `ruby` once and gates the oracle on MRI >= 4.0 (Set is
// autoloaded core there; older rubies require `require "set"` and render Set with
// the old "#<Set: {...}>" inspect, so the oracle would not match). The oracle
// tests skip themselves when ruby is absent (the qemu cross-arch lanes and the
// Windows lane), so the deterministic suite alone drives the 100% coverage gate.
func rubyBin(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("ruby")
	if err != nil {
		t.Skip("ruby not on PATH; skipping MRI oracle")
	}
	out, err := exec.Command(path, "-e", "print RUBY_VERSION").Output()
	if err != nil {
		t.Skipf("cannot read RUBY_VERSION: %v", err)
	}
	if !versionAtLeast4(string(out)) {
		t.Skipf("ruby %s < 4.0; skipping MRI 4.0 Set oracle", out)
	}
	return path
}

// versionAtLeast4 reports whether a "MAJOR.MINOR.PATCH" version string is >= 4.0.
func versionAtLeast4(v string) bool {
	major := strings.SplitN(strings.TrimSpace(v), ".", 2)[0]
	return major >= "4" // string compare is fine for single-digit-vs-larger majors here
}

// rubyEval runs a Ruby script with stdin/stdout in binary mode (the go-ruby-erb
// Windows lesson) and returns its trimmed stdout. Set is core in Ruby 4.0, so no
// `require "set"` is needed.
func rubyEval(t *testing.T, bin, script string) string {
	t.Helper()
	cmd := exec.Command(bin, "-e", "$stdin.binmode\n$stdout.binmode\n"+script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ruby error: %v\nscript:\n%s\noutput:\n%s", err, script, out)
	}
	return strings.TrimRight(string(out), "\n")
}

// intLit renders a Set of ints as the Ruby literal `Set[1, 2, 3]`.
func intLit(elems ...int) string {
	parts := make([]string, len(elems))
	for i, e := range elems {
		parts[i] = fmt.Sprintf("%d", e)
	}
	return "Set[" + strings.Join(parts, ", ") + "]"
}

// sortInts sorts a copy and returns it as a comma string, for order-independent
// comparison against `to_a.sort`.
func sortIntStr(s *Set) string {
	xs := make([]int, 0, s.Size())
	for _, v := range s.ToSlice() {
		xs = append(xs, v.(int))
	}
	sort.Ints(xs)
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = fmt.Sprintf("%d", x)
	}
	return strings.Join(parts, ",")
}

// TestOracleAlgebra checks that the union / intersection / difference / symmetric
// difference computed here agree with MRI's, comparing sorted member lists.
func TestOracleAlgebra(t *testing.T) {
	bin := rubyBin(t)
	a := New(1, 2, 3, 4)
	b := New(3, 4, 5, 6)
	cases := []struct {
		name string
		got  *Set
		expr string
	}{
		{"union", a.Union(b), intLit(1, 2, 3, 4) + " | " + intLit(3, 4, 5, 6)},
		{"intersection", a.Intersection(b), intLit(1, 2, 3, 4) + " & " + intLit(3, 4, 5, 6)},
		{"difference", a.Difference(b), intLit(1, 2, 3, 4) + " - " + intLit(3, 4, 5, 6)},
		{"xor", a.XorSym(b), intLit(1, 2, 3, 4) + " ^ " + intLit(3, 4, 5, 6)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			want := rubyEval(t, bin, fmt.Sprintf("print (%s).to_a.sort.join(',')", c.expr))
			if got := sortIntStr(c.got); got != want {
				t.Errorf("%s = %q, MRI = %q", c.name, got, want)
			}
		})
	}
}

// TestOraclePredicates checks subset / superset / proper / disjoint / intersect /
// equality predicates against MRI.
func TestOraclePredicates(t *testing.T) {
	bin := rubyBin(t)
	a := New(1, 2)
	b := New(1, 2, 3)
	c := New(1, 2)
	cases := []struct {
		name string
		got  bool
		expr string
	}{
		{"subset", a.SubsetQ(b), intLit(1, 2) + " <= " + intLit(1, 2, 3)},
		{"subset_eq", a.SubsetQ(c), intLit(1, 2) + " <= " + intLit(1, 2)},
		{"proper_subset", a.ProperSubsetQ(b), intLit(1, 2) + " < " + intLit(1, 2, 3)},
		{"proper_subset_eq", a.ProperSubsetQ(c), intLit(1, 2) + " < " + intLit(1, 2)},
		{"superset", b.SupersetQ(a), intLit(1, 2, 3) + " >= " + intLit(1, 2)},
		{"proper_superset", b.ProperSupersetQ(a), intLit(1, 2, 3) + " > " + intLit(1, 2)},
		{"disjoint", a.DisjointQ(New(7, 8)), intLit(1, 2) + ".disjoint?(" + intLit(7, 8) + ")"},
		{"intersect", a.IntersectQ(b), intLit(1, 2) + ".intersect?(" + intLit(1, 2, 3) + ")"},
		{"equal", a.EqualQ(c), intLit(1, 2) + " == " + intLit(1, 2)},
		{"equal_false", a.EqualQ(b), intLit(1, 2) + " == " + intLit(1, 2, 3)},
	}
	for _, cse := range cases {
		t.Run(cse.name, func(t *testing.T) {
			want := rubyEval(t, bin, fmt.Sprintf("print (%s)", cse.expr))
			if got := fmt.Sprintf("%v", cse.got); got != want {
				t.Errorf("%s = %s, MRI = %s", cse.name, got, want)
			}
		})
	}
}

// TestOracleClassify checks Set#classify: members grouped by a block value into a
// Hash{value => Set}. We compare each bucket's sorted members.
func TestOracleClassify(t *testing.T) {
	bin := rubyBin(t)
	s := New(1, 2, 3, 4, 5, 6)
	res := s.Classify(func(e any) any { return e.(int) % 3 })
	// Build a "mod => sorted members" map from our result.
	for _, g := range res.Groups() {
		mod := g.Value.(int)
		want := rubyEval(t, bin, fmt.Sprintf(
			"print %s.classify { |x| x %% 3 }[%d].to_a.sort.join(',')",
			intLit(1, 2, 3, 4, 5, 6), mod))
		if got := sortIntStr(g.Set); got != want {
			t.Errorf("classify[%d] = %q, MRI = %q", mod, got, want)
		}
	}
}

// TestOracleDivide checks Set#divide with a 1-arg block (partition by block value)
// and a 2-arg block (transitive-closure components), comparing the multiset of
// sorted components.
func TestOracleDivide(t *testing.T) {
	bin := rubyBin(t)

	// 1-arg: group by leading digit.
	parts := New(1, 2, 3, 10, 11, 20).Divide(func(e any) any {
		return fmt.Sprintf("%d", e.(int))[0:1]
	})
	if got := componentsStr(parts); got != componentsRuby(t, bin,
		intLit(1, 2, 3, 10, 11, 20)+".divide { |x| x.to_s[0] }") {
		t.Errorf("divide(1-arg) = %q, MRI = %q", got,
			componentsRuby(t, bin, intLit(1, 2, 3, 10, 11, 20)+".divide { |x| x.to_s[0] }"))
	}

	// 2-arg: adjacency relation forms runs of consecutive integers.
	rel := New(1, 2, 5, 6, 10).DivideRel(func(a, b any) bool {
		d := a.(int) - b.(int)
		return d == 1 || d == -1
	})
	if got := componentsStr(rel); got != componentsRuby(t, bin,
		intLit(1, 2, 5, 6, 10)+".divide { |i, j| (i - j).abs == 1 }") {
		t.Errorf("divide(2-arg) = %q, MRI = %q", got,
			componentsRuby(t, bin, intLit(1, 2, 5, 6, 10)+".divide { |i, j| (i - j).abs == 1 }"))
	}
}

// componentsStr renders a slice of int Sets as a canonical "1,2|5,6|10" string
// (each component sorted, components sorted by first member).
func componentsStr(parts []*Set) string {
	comps := make([]string, len(parts))
	for i, p := range parts {
		comps[i] = sortIntStr(p)
	}
	sort.Slice(comps, func(i, j int) bool { return comps[i] < comps[j] })
	return strings.Join(comps, "|")
}

// componentsRuby evaluates a Ruby `divide` expression and renders its components
// the same canonical way componentsStr does.
func componentsRuby(t *testing.T, bin, expr string) string {
	return rubyEval(t, bin, fmt.Sprintf(
		"print (%s).map { |s| s.to_a.sort.join(',') }.sort.join('|')", expr))
}

// TestOracleInspect checks the MRI 4.0 inspect form "Set[1, 2, 3]".
func TestOracleInspect(t *testing.T) {
	bin := rubyBin(t)
	got := New(1, 2, 3).Inspect(func(e any) string { return fmt.Sprintf("%d", e) })
	want := rubyEval(t, bin, "print Set[1, 2, 3].inspect")
	if got != want {
		t.Errorf("inspect = %q, MRI = %q", got, want)
	}
	if e := New().Inspect(DefaultInspect); e != rubyEval(t, bin, "print Set[].inspect") {
		t.Errorf("empty inspect = %q, MRI = %q", e, rubyEval(t, bin, "print Set[].inspect"))
	}
}
