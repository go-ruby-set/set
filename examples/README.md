# set examples

Runnable pure-Ruby usage of the `set` unordered, unique collection with full set algebra, verified under the [rbgo](https://github.com/go-embedded-ruby) interpreter.

```sh
rbgo examples/set_usage.rb
```

| File | Shows |
| --- | --- |
| `set_usage.rb` | Build sets with the `Set[...]` literal, `Set.new`, and `add?`; combine them with union `\|`, intersection `&`, difference `-` and symmetric difference `^`; test `subset?` / `superset?` / `disjoint?` and order-independent `==`; and enumerate with `each` / `select` / `map`. |
