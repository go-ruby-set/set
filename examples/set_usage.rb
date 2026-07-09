# frozen_string_literal: true
#
# Usage of Set — the unordered, unique collection with full set algebra
# added by `require "set"`. Runs under go-embedded-ruby (rbgo);
# see examples/README.md.

require "set"

# Construction: the Set[...] literal, Set.new(enumerable), and << / add?.
a = Set[1, 2, 3, 4]
b = Set[3, 4, 5, 6]
p Set.new([1, 1, 2, 3]).to_a          # => [1, 2, 3]   (duplicates collapse)
p a.include?(2)                       # => true
p Set[1, 2, 3].add?(9).to_a           # => [1, 2, 3, 9]   (self when new)
p Set[1, 2, 3].add?(2)                # => nil            (already present)

# Set algebra: union | , intersection & , difference - , symmetric diff ^ .
p (a | b).to_a                        # => [1, 2, 3, 4, 5, 6]
p (a & b).to_a                        # => [3, 4]
p (a - b).to_a                        # => [1, 2]
p (a ^ b).to_a                        # => [1, 2, 5, 6]

# Predicates: subset?/superset?, disjoint?/intersect?, and value equality.
p a.subset?(Set[1, 2, 3, 4, 5])       # => true
p a.superset?(Set[1, 2])              # => true
p a.disjoint?(Set[7, 8])              # => true
p (Set[1, 2] == Set[2, 1])            # => true   (order does not matter)

# Enumeration: each preserves insertion order; select/map derive new values.
Set[1, 2, 3].each { |x| print x, " " } # => 1 2 3
puts
p Set[1, 2, 3, 4].select { |x| x.even? }.to_a  # => [2, 4]
p Set[1, 2, 3].map { |x| x * 10 }               # => [10, 20, 30]
