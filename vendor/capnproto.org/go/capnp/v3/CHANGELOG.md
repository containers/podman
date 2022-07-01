# Go Cap'n Proto Release Notes

## 2.17.0

- Add `capnp.Canonicalize` function that implements the
  [canonicalization](https://capnproto.org/encoding.html#canonicalization)
  algorithm.  ([#92](https://github.com/capnproto/go-capnproto2/issues/92))
- Zero-sized struct pointers are now written with an offset of
  -1 to distinguish them from a null pointer.
  ([#92](https://github.com/capnproto/go-capnproto2/issues/92))
- Better support for alternate `Arena` implementations
  - [Document `Arena` contract](https://godoc.org/capnproto.org/go/capnp/v3#Arena)
    in more detail
  - Permit an `Arena` to have a single empty segment in `NewMessage`
- `Arena` allocation optimizations: both `SingleSegment` and
  `MultiSegment` now gradually ramp up the amount of space allocated in
  a single allocation as the message grows.  This is similar to how
  built-in Go `append` function works.  Workloads with medium to large
  messages should expect a decrease in number of allocations, while
  small message workloads should remain about the same.  Please file an
  issue if you encounter any performance regressions.
  ([#96](https://github.com/capnproto/go-capnproto2/issues/96))
- Fix double-far pointer logic.  ([#97](https://github.com/capnproto/go-capnproto2/issues/97))
  This is a long-standing bug with reading and writing multi-segment
  messages.  I've added broader test coverage for multi-segment messages
  and far pointers, so it's unlikely that such a failure will persist in
  the future.
- Accessing a field in a union when that field is not the one set now
  results in a panic.  ([#56](https://github.com/capnproto/go-capnproto2/issues/56))
  This is intended to help uncover programming mistakes where a union
  field is accessed without checking `Which()`.  Prior to this change,
  unset union field accessors would silently return garbage.

## 2.16.0

- Add BUILD.bazel files ([#88](https://github.com/capnproto/go-capnproto2/issues/88))

## 2.15.0

- capnpc-go now fails when a file does not include an import annotation.
  ([#41](https://github.com/capnproto/go-capnproto2/issues/41))
- Remove rbtree dependency ([#80](https://github.com/capnproto/go-capnproto2/issues/80))
- Add option to reduce allocations in `capnp.Decoder`
  ([#79](https://github.com/capnproto/go-capnproto2/issues/79))
- Add `String()` methods for lists
  ([#85](https://github.com/capnproto/go-capnproto2/issues/85))
- Add `String()` methods to schema.capnp.go
  ([#83](https://github.com/capnproto/go-capnproto2/issues/83))

## 2.14.1

- Use [new Go generated code convention](https://golang.org/s/generatedcode) in
  capnpc-go output ([#78](https://github.com/capnproto/go-capnproto2/issues/78))

## Retroactive Releases

go-capnproto2 was originally a "build from HEAD" sort of library, as was
convention for most Go libraries at the time.  Before 2.14.1, Semantic
Versioning tags were retroactively added so that it would be clear what the
differences were since original release, since marking it as "2.0.0" would seem
awkward.

The general process was: any significant new feature was given a minor release,
and then any bugfixes before the next minor release were given a "2.X.1"
release.

<table>
  <thead>
    <tr>
      <th scope="col">Version</th>
      <th scope="col">Description</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <th scope="row"><a href="https://github.com/capnproto/go-capnproto2/releases/tag/v2.14.0">2.14.0</a></th>
      <td>Add support to <code>pogs</code> for interface types (<a href="https://github.com/capnproto/go-capnproto2/issues/66">#66</a> and <a href="https://github.com/capnproto/go-capnproto2/issues/74">#74</a>)</td>
    </tr>
    <tr>
      <th scope="row"><a href="https://github.com/capnproto/go-capnproto2/releases/tag/v2.13.1">2.13.1</a></th>
      <td>Fix bug with far far pointers (<a href="https://github.com/capnproto/go-capnproto2/issues/71">#71</a>), use <code>writev</code> system call to encode multi-segment messages efficiently in Go 1.8+ (<a href="https://github.com/capnproto/go-capnproto2/issues/70">#70</a>), and add GitHub-Linguist-compatible code generation comment</td>
    </tr>
    <tr>
      <th scope="row"><a href="https://github.com/capnproto/go-capnproto2/releases/tag/v2.13.0">2.13.0</a></th>
      <td>Add <code>Conn.Done</code> and <code>Conn.Err</code> methods</td>
    </tr>
    <tr>
      <th scope="row"><a href="https://github.com/capnproto/go-capnproto2/releases/tag/v2.12.4">2.12.4</a></th>
      <td>Fix size of created <code>List(Float32)</code></td>
    </tr>
    <tr>
      <th scope="row"><a href="https://github.com/capnproto/go-capnproto2/releases/tag/v2.12.3">2.12.3</a></th>
      <td>Fix bugs from fuzz tests: mismatched element size on list access causing crashes (<a href="https://github.com/capnproto/go-capnproto2/issues/59">#59</a>) and miscellaneous packed reader issues</td>
    </tr>
    <tr>
      <th scope="row"><a href="https://github.com/capnproto/go-capnproto2/releases/tag/v2.12.2">2.12.2</a></th>
      <td>Fix another shutdown race condition (<a href="https://github.com/capnproto/go-capnproto2/issues/54">#54</a>)</td>
    </tr>
    <tr>
      <th scope="row"><a href="https://github.com/capnproto/go-capnproto2/releases/tag/v2.12.1">2.12.1</a></th>
      <td>Fix ownership bug with receiver-hosted capabilities, add discriminant check to <code>HasField</code> (<a href="https://github.com/capnproto/go-capnproto2/issues/55">#55</a>), fix multi-segment bug for data/text lists, and use nulls for setting empty data/text</td>
    </tr>
    <tr>
      <th scope="row"><a href="https://github.com/capnproto/go-capnproto2/releases/tag/v2.12.0">2.12.0</a></th>
      <td>Add <code>rpc.ConnLog</code> option and fix race conditions and edge cases in RPC implementation</td>
    </tr>
    <tr>
      <th scope="row"><a href="https://github.com/capnproto/go-capnproto2/releases/tag/v2.11.1">2.11.1</a></th>
      <td>Fix packed reader behavior on certain readers (<a href="https://github.com/capnproto/go-capnproto2/issues/49">#49</a>), add <code>capnp.UnmarshalPacked</code> function that performs faster, and reduce locking overhead of segment maps</td>
    </tr>
    <tr>
      <th scope="row"><a href="https://github.com/capnproto/go-capnproto2/releases/tag/v2.11.0">2.11.0</a></th>
      <td>Fix shutdown deadlock in RPC shutdown (<a href="https://github.com/capnproto/go-capnproto2/issues/45">#45</a>)</td>
    </tr>
    <tr>
      <th scope="row"><a href="https://github.com/capnproto/go-capnproto2/releases/tag/v2.10.1">2.10.1</a></th>
      <td>Work around lack of support for RPC-level promise capabilities (<a href="https://github.com/capnproto/go-capnproto2/issues/2">#2</a>)</td>
    </tr>
    <tr>
      <th scope="row"><a href="https://github.com/capnproto/go-capnproto2/releases/tag/v2.10.0">2.10.0</a></th>
      <td>Add <code>pogs</code> package (<a href="https://github.com/capnproto/go-capnproto2/issues/33">#33</a>)</td>
    </tr>
    <tr>
      <th scope="row"><a href="https://github.com/capnproto/go-capnproto2/releases/tag/v2.9.1">2.9.1</a></th>
      <td>Fix not-found behavior in schemas and add missing group IDs in generated embedded schemas</td>
    </tr>
    <tr>
      <th scope="row"><a href="https://github.com/capnproto/go-capnproto2/releases/tag/v2.9.0">2.9.0</a></th>
      <td>Add <code>encoding/text</code> package (<a href="https://github.com/capnproto/go-capnproto2/issues/20">#20</a>)</td>
    </tr>
    <tr>
      <th scope="row"><a href="https://github.com/capnproto/go-capnproto2/releases/tag/v2.8.0">2.8.0</a></th>
      <td>Reduce generated code size for text fields and correct NUL check</td>
    </tr>
    <tr>
      <th scope="row"><a href="https://github.com/capnproto/go-capnproto2/releases/tag/v2.7.0">2.7.0</a></th>
      <td>Insert compressed schema data into generated code</td>
    </tr>
    <tr>
      <th scope="row"><a href="https://github.com/capnproto/go-capnproto2/releases/tag/v2.6.1">2.6.1</a></th>
      <td>Strip NUL byte from <code>TextList.BytesAt</code> and fix capnpc-go output for struct groups</td>
    </tr>
    <tr>
      <th scope="row"><a href="https://github.com/capnproto/go-capnproto2/releases/tag/v2.6.0">2.6.0</a></th>
      <td>Add packages for predefined Cap'n Proto schemas</td>
    </tr>
    <tr>
      <th scope="row"><a href="https://github.com/capnproto/go-capnproto2/releases/tag/v2.5.1">2.5.1</a></th>
      <td>Fix capnpc-go regression (<a href="https://github.com/capnproto/go-capnproto2/issues/29">#29</a>) and strip trailing NUL byte in TextBytes accessor</td>
    </tr>
    <tr>
      <th scope="row"><a href="https://github.com/capnproto/go-capnproto2/releases/tag/v2.5.0">2.5.0</a></th>
      <td>Add <code>NewFoo</code> method for list fields in generated structs (<a href="https://github.com/capnproto/go-capnproto2/issues/7">#7</a>)</td>
    </tr>
    <tr>
      <th scope="row"><a href="https://github.com/capnproto/go-capnproto2/releases/tag/v2.4.0">2.4.0</a></th>
      <td>Add maximum segment limit (<a href="https://github.com/capnproto/go-capnproto2/issues/25">#25</a>)</td>
    </tr>
    <tr>
      <th scope="row"><a href="https://github.com/capnproto/go-capnproto2/releases/tag/v2.3.0">2.3.0</a></th>
      <td>Add depth and traversal limit security checks</td>
    </tr>
    <tr>
      <th scope="row"><a href="https://github.com/capnproto/go-capnproto2/releases/tag/v2.2.1">2.2.1</a></th>
      <td>Fix data race in reading <code>Message</code> from multiple goroutines</td>
    </tr>
    <tr>
      <th scope="row"><a href="https://github.com/capnproto/go-capnproto2/releases/tag/v2.2.0">2.2.0</a></th>
      <td>Add <code>HasFoo</code> pointer field methods to generated code (<a href="https://github.com/capnproto/go-capnproto2/issues/24">#24</a>)</td>
    </tr>
    <tr>
      <th scope="row"><a href="https://github.com/capnproto/go-capnproto2/releases/tag/v2.1.0">2.1.0</a></th>
      <td><a href="https://github.com/capnproto/go-capnproto2/wiki/New-Ptr-Type">Introduce <code>Ptr</code> type</a> and reduce allocations in single-segment cases</td>
    </tr>
    <tr>
      <th scope="row"><a href="https://github.com/capnproto/go-capnproto2/releases/tag/v2.0.2">2.0.2</a></th>
      <td>Allow allocation-less string field access via <code>TextList.BytesAt()</code> and <code>StringBytes()</code> (<a href="https://github.com/capnproto/go-capnproto2/issues/17">#17</a>)</td>
    </tr>
    <tr>
      <th scope="row"><a href="https://github.com/capnproto/go-capnproto2/releases/tag/v2.0.1">2.0.1</a></th>
      <td>Allow nil params in client wrappers (<a href="https://github.com/capnproto/go-capnproto2/issues/9">#9</a>) and fix integer underflow on compare function (<a href="https://github.com/capnproto/go-capnproto2/issues/12">#12</a>)</td>
    </tr>
    <tr>
      <th scope="row"><a href="https://github.com/capnproto/go-capnproto2/releases/tag/v2.0.0">2.0.0</a></th>
      <td>First release under <code>capnproto.org/go/capnp/v3</code></td>
    </tr>
  </tbody>
</table>
