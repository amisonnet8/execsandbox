# ExecSandbox Specification

**Version**: v1 draft

Specification for a sandboxed execution tool that bundles a WASM runtime and
a WASM module into a single executable file.

Each item is accompanied by the rationale behind the choice. If you think of
an alternative while implementing, first check the rationale and the
"alternatives considered and rejected" for that item.

For a practical reference of commands and options, see `../usage/`.

---

## 1. Terminology

| Term | Definition |
|---|---|
| **ExecSandbox** | The executable file that embeds a WASM module, and the process it runs as |
| **WASM module** | The WASM binary executed inside the sandbox |
| **Host** | ExecSandbox's Go-side implementation. Everything outside the WASM module |
| **Guest** | The WASM module side |
| **Mailbox** | The queue of arrived messages held by the receiving side |
| **Destination** | The target of `send`. Assigned to a number at launch time |
| **ID** | The logical name of an ExecSandbox instance. Resolves to a socket path |
| **connID** | The identifier assigned to one external connection |
| **Builder** | The command that turns a `.wasm` file into an executable (`execsandbox-build`) |

Use these terms consistently in code, comments, and logs. The following pairs
are especially easy to confuse.

| Use | Don't use |
|---|---|
| mailbox | queue, channel, buffer |
| destination | send target, target |
| host / guest | server / client |
| external connection | TCP (it may be a Unix domain socket) |

---

## 2. Architecture

### 2.1 Components

| Component | Implementation | Description |
|---|---|---|
| ExecSandbox core | Go | `sandbox` (policy enforcement and message delivery) + `cmd/execsandbox` (CLI). Embeds `wazero` |
| WASM module | Language-agnostic | Any language is fine as long as it produces a valid WASM binary |
| SDK library | Per language | Wraps the ABI in chapter 5. One per language. **Lives in a separate repository** (`execsandbox-sdk`) |
| Builder | Go | Embeds a `.wasm` file into an executable. Language-agnostic, a single one suffices |

`wazero` was chosen because it is a pure-Go WASM runtime that requires no CGO
and has zero dependencies, letting it directly realize the "single binary,
no CGO" distribution model.

**Distribution happens as a unit of a per-language development kit (SDK
library + builder), and the builder bundled with every kit is the same one.**

As components, the per-language SDK libraries and the language-agnostic
builder are distinct things. The builder's input is a `.wasm` binary and it
does not distinguish which language produced it, so a single builder
suffices. From the perspective of someone building a sandbox, though, the two
only make sense together — "writing a WASM module" alone isn't a deliverable;
"turning it into an executable" is part of the same piece of work.

With this shape, the cost of supporting a new language is just writing one
library. Because the builder is a single binary that requires no Go
toolchain thanks to the stamping approach (6.2), it can be bundled without,
for example, forcing a Rust developer to install Go.

#### Why the SDK library lives in a separate repository

The core/builder and the SDK libraries live in separate repositories.

- The core is "pure Go, no CGO, zero dependencies," and its CI completes with
  a single cross-compilation job. Adding each language's toolchain on top of
  that would weigh down the core's build and tests.
- Package manager conventions differ per language, and they collide when
  crammed under one repository root.

**The dependency runs one way, from SDK to core, and there is zero code-level
dependency.** The SDK only declares imports of host functions; it never
references the core's packages. The nature of the ABI — "host and guest only
exchange numbers across linear memory" — is what makes this loose coupling
possible.

Section 5 of this specification is the sole owner of the ABI definition, and
the SDK side merely refers to it. Because the ABI only guarantees backward
compatibility (5.1), existing modules keep working even when the two
repositories' versions drift apart.

### 2.2 Execution Model

- One ExecSandbox process runs exactly one WASM module.
- A system is assembled by wiring multiple ExecSandbox processes together
  through launch-time options.
- There is no mechanism-level distinction by role (core, relay, broker,
  external interface, management, etc.). Everything runs through the same
  mechanism.
- Starting, stopping, and monitoring multiple processes is outside this
  tool's responsibility.

#### Why roles aren't distinguished at the mechanism level

Distinctions such as "something that does the processing," "something that
merely relays," or "something that bundles things together" are not
introduced at the level of primitives. Every role is realized as
structurally the same single ExecSandbox instance.

This means "building a system" reduces to constructing a graph that wires
instances of differing roles together through launch-time destination
assignment. We prioritized the consistency of a system that assembles purely
from composition, without introducing a special relay primitive.

The topology (P2P mesh, hub-and-spoke, or a mix) is entirely up to whoever
assembles the system. Inserting an intermediary makes destination management
easier to centralize, at the cost of introducing a single point of failure.

#### Why there is no management interface

ExecSandbox has no management protocol for controlling itself. Even when a
management function is needed, it is expressed by building one more
ExecSandbox that plays the management role and wiring it in.

Building a special mechanism into the core for management purposes would
break the consistency described above — "every role uses the same
mechanism" — and would also bloat the host's deliberately minimal
responsibilities (3.7). We avoid this by treating management as just another
role, nothing more.

#### Why there is no process management

Because each sandbox is started as an independent process wired together
through launch-time options, a shell script is enough to start them all
together.

```sh
#!/bin/sh
./dbcore -n dbcore -v /data:/data &
./extif  -d 1=dbcore -l 5432 &
./repl   -d 1=dbcore -s in,out
```

Building a dedicated launch mechanism would reinvent Docker Compose or
supervisord, and would drag a management concept into the core. We leave it
to existing tools instead.

---

## 3. Sandbox-to-Sandbox Communication

### 3.1 Transport

- Sandbox-to-sandbox communication uses Unix domain sockets (AF_UNIX),
  uniformly across all platforms.
- Communication is limited to processes on the same host.

#### Why AF_UNIX was chosen

Since communication is scoped to the same host, a Unix domain socket that
bypasses the network stack is the most straightforward fit. On top of that,
**socket file permissions provide OS-level access control**, which meshes
well with the capability-based design philosophy (3.3).

**We don't encrypt the communication (e.g. TLS).** AF_UNIX completes as a
memory copy inside the kernel and never goes out as a packet on a network
interface, so the on-path eavesdropping/tampering threat model that TLS
addresses simply doesn't exist here. Access control comes not from
encryption but from the socket file and parent directory permissions
described above (3.2).

Windows natively supports AF_UNIX from Windows 10 (1803) onward, and Go's
`net.Listen("unix", path)` works as-is. The following were rejected:

- **Windows-only named pipes**: The most common IPC mechanism on Windows,
  but it has no implementation in Go's standard library, requiring an
  external dependency such as `go-winio`. That would break the "no CGO, zero
  dependencies" style.
- **Windows-only loopback TCP**: Can be written with the standard library
  alone, but requires managing port numbers, and loses the access control
  that file permissions provide, allowing other local users to connect too.

We prioritized having a single code path.

### 3.2 Address Resolution

Resolving an ID to a socket path follows this fixed pattern.

| Environment | Path |
|---|---|
| Unix-like (`$XDG_RUNTIME_DIR` set) | `$XDG_RUNTIME_DIR/execsandbox/<ID>.sock` |
| Unix-like (unset) | `/tmp/execsandbox-<uid>/<ID>.sock` |
| Windows | `%LOCALAPPDATA%\execsandbox\<ID>.sock` |

- When creating a directory under `/tmp`, its permissions are `0700`.
- When starting to listen, if a socket file already exists, ExecSandbox
  tries to connect to it; if there is no response, it removes the file
  before binding.

We avoid placing files directly under `/tmp` because a location visible to
other users weakens the access control that permissions provide.
`$XDG_RUNTIME_DIR` (typically `/run/user/<uid>`) is private to the launching
user, which makes it preferable on this point.

We resolve through an ID (a logical name) rather than letting callers
specify a socket path directly, to hide the implementation detail of the
path from the user.

### 3.3 Destinations and Capabilities

- One's own ID is given with the `-n` launch option. If omitted, the
  sandbox does not receive sandbox-to-sandbox messages.
- Destinations are assigned to numbers with the `-d N=ID` launch option.
  Numbers are integers of 1 or greater.
- A guest sends by specifying only a number. It has no way to learn the
  actual identity behind a destination.
- A guest cannot send to a destination that wasn't assigned at launch time.

#### Launch-time assignment as the safety boundary

Assigning destinations at launch time isn't merely a convenience setting —
it is the sandbox's **safety boundary**.

- **Launch-time assignment**: An allowlist, decided in advance by whoever
  launches the sandbox (a trusted human or system), of which peers this WASM
  module is allowed to talk to. If a guest could freely connect to any
  destination at runtime, the sandbox's isolation would be meaningless.
- **Runtime `send`**: The guest merely picks, among the destinations it was
  granted, which one to send to this time — it is physically unable to send
  to a destination it wasn't granted.

**The principle that "isolation is decided by whoever launches the sandbox"
has no exceptions.** Policy is never baked in at build time either (6.1).

#### Why destinations are referenced by number

Rather than a string identifier such as a sandbox name or role name, we
adopt a number, like Unix's file descriptors 0/1/2.

```
At launch: destination 1 = nodeB, destination 2 = nodeC, destination 3 = logger
Guest: send(1, data)   // written simply as "send to destination 1"
```

Because a guest only ever touches the number, it never needs to know the
actual identity of a destination, and the same WASM module can be
repurposed for a different system configuration just by changing the launch
wiring.

### 3.4 Send Behavior

`send` discards a message without returning an error in every one of the
following cases.

| Condition | Host-side log |
|---|---|
| Destination number is unassigned | Not emitted |
| Destination isn't running, or can't be connected to | Not emitted |
| Payload length exceeds `--max-frame` | Emitted |
| The peer's mailbox is at capacity | The receiving side emits it (3.6) |

The connection to a destination is established on the first send. If it
fails, it is retried on the next send.

#### Why an unassigned destination is silently dropped

This mirrors how Unix treats `/dev/null`. Sending to a destination that
wasn't granted simply fails to arrive, without halting the whole system.

This differs from tail-drop (3.6, which does log), because the two are
different in nature. An unassigned destination is a **static state**
visible just by looking at the launch options, and needs no notification.
Tail-drop, on the other hand, is a **dynamic phenomenon** that depends on
runtime load, and cannot be observed from outside unless it's recorded.

#### Why the connection is deferred

For sandboxes to be startable together from a shell script, the state "the
destination hasn't started yet" has to be tolerable. That's why the
connection is established on the first send, and retried on failure at the
next send.

Messages sent before the peer starts up are silently dropped, but this is a
transient state limited to right after startup.

### 3.5 Framing

Sandbox-to-sandbox messages are transferred in the following format.

```
[length: u32 little-endian][payload]
```

- The receiving side determines the boundary of one message by reading
  exactly the declared length.
- A frame whose length exceeds `--max-frame` is not accepted and is
  discarded.
- The guest is unaware of this format; the host attaches and interprets it.

#### Why a length prefix was chosen

A delimiter-based scheme (a newline, a NUL byte, etc.) breaks if the same
byte appears in the payload, requiring escaping. With a length prefix,
collision with the payload's contents is structurally impossible, and it
unconditionally guarantees that "the bytes you sent arrive exactly as sent."

Little-endian was chosen to match WASM linear memory's representation, so
the guest side needs no conversion. If the u32 width itself were the only
cap, a peer could declare a huge length and force the host to allocate a
huge amount of memory, so `--max-frame` sets a separate cap.

### 3.6 Mailbox

- The mailbox is held by the receiving process.
- Its capacity is specified as a message count (`-b, --mailbox-limit`).
- When a new message arrives while the mailbox is at capacity, **that new
  message is discarded** (tail-drop). Existing messages are never discarded.
- The discard is not reported to the guest.
- The discard is recorded to host-side stderr. When it happens repeatedly,
  it is rate-limited: after the first log line, only a running total is
  emitted at intervals.
- Exception: an external-connection disconnect event (4.2, kind=3) is always
  delivered regardless of this cap. This prevents the guest from leaking
  per-connection state it has no way to know it's safe to discard. The
  amount that can exceed the cap is bounded by the number of concurrently
  live connections, so the memory-cap intent of this section is not
  undermined.

#### Why tail-drop was chosen

Head-drop (discarding the oldest) disturbs an ordering that has already
entered the queue, which would contradict the guarantee we chose to make —
"ordering within a single sender" (3.7).

The cap is expressed as a message count so that, combined with
`--max-frame`, the memory cap is determined as count × maximum frame size.

### 3.7 Scope of Guarantees

The host guarantees only the following two things.

1. **Frame boundaries**: The bytes that were sent arrive at the receiver as
   exactly one message, unchanged.
2. **Ordering within a single sender**: Multiple messages sent by a given
   sender arrive in the order that sender sent them.

The following are not guaranteed.

- Delivery to the destination
- Ordering between different senders
- Interpretation, validation, or routing of message content

#### Why delivery isn't guaranteed

This follows from committing to a Push model, where the sender never asks
the peer whether it can receive and never waits for a response. Since a
backed-up receiver causes messages to be dropped, "guaranteed delivery"
can't be claimed.

A system that needs delivery confirmation should design its own ACK on the
guest side. This is consistent with the boundary that "the host handles only
the mechanics of delivery, and meaning is left to the guest."

#### Fan-out / fan-in come for free, without a mechanism

- **Fan-out**: Achieved purely by the design allowing multiple destinations
  to be specified at launch time. No separate relay role is needed.
- **Fan-in**: The receiving side doesn't distinguish senders and accumulates
  everything into the same mailbox, so receiving from multiple senders is
  possible from the start.

Because of this property, `send` never needs a broadcast form — an SDK-side
loop helper is enough. It also avoids the ambiguity of what should happen
when only some of several destinations are backed up.

---

## 4. External Connections

### 4.1 Listening

- The listen address is given with the `-l, --listen` launch option. If
  omitted, the sandbox does not listen.
- **At most one can be specified per instance.**
- The actual socket operations (listen, accept, connection management) are
  performed by the host.

#### Why the host owns the socket

`wazero`'s networking support — WASI Preview 1's `sock_accept` and friends
are limited, and `experimental/sys`'s outbound TCP allowlist is still
experimental and immature — led us to not rely on either, and instead
introduce our own host functions.

The actual socket is handled with the well-worn standard library
(`net.Listen` and friends), keeping safety and maturity confined to the Go
side. The guest is only given per-connection byte-stream reads and writes;
all protocol interpretation is entirely the guest's own logic.

#### Why listening is limited to one

Allowing multiple listeners would make it impossible to tell, from connID
alone, which listener a connection came through, requiring a listener number
to be added to events. When multiple are needed, this can be expressed by
standing up multiple ExecSandbox instances playing an external-interface
role, wired toward a core instance.

#### Why the listen address defaults to loopback

The syntax follows nginx's `listen` directive, but the default when only a
port number is given is inverted (nginx defaults to all interfaces).
ExecSandbox is a sandbox, not a web server, and we felt **the shortest form
to write should also be the safest setting**. This is the same policy
PostgreSQL's `listen_addresses` and Redis's `bind` take by defaulting to
loopback.

### 4.2 Connection Identification

- The host assigns a connID to each connection.
- connID is a monotonically increasing u32, never reused after
  disconnection. If u32 wraps around, still-live connIDs are skipped when
  assigning.
- The guest only ever handles the connID; it has no way to learn the actual
  identity of the connection.

Because the disconnect event is always delivered, no mix-up occurs as long
as the guest discards its state on disconnect. Designing connIDs to never be
reused means that even if the guest forgets to discard state, it won't be
confused with a different connection — the failure is cleanly attributable
to a guest-side bug.

### 4.3 Data Flow

- **Inbound direction**: Connection establishment, data arrival, and
  disconnection all flow into the mailbox as events (see 5.3). No dedicated
  receive function is provided.
- **Outbound direction**: Written back with `conn_write`.

For data originating from an external connection, the host only guarantees
ordering within a connection. Chunk boundaries carry no meaning, and the
framing in 3.5 does not apply.

#### Why receiving is merged into the mailbox

Naively placing a blocking function like `accept()` alongside things would
create two waiting points — the external connection and `recv()`. With only
a single thread and only blocking functions, waiting on one means being
unable to react to the other, creating a deadlock where a new connection is
missed while blocked in `recv()`.

Funneling receive events into the mailbox unifies the waiting point into
one, and the problem disappears. This mirrors how Erlang delivers input from
ports (external processes or sockets) into the same mailbox as ordinary
messages, and it also fits the grain of the actor model.

Only the receiving direction was merged. The sending direction has nothing
to do with the waiting-point problem, so it remains `conn_write`. As a
result, the only custom host function added is `conn_write`, just the one.

#### Why stdio isn't merged in

The same reasoning raises the question of whether stdio should also be
funneled in, but we treat it asymmetrically from external connections.

- External connections **required a custom implementation anyway**, so
  merging them cost nothing extra and purely reduced the number of waiting
  points. Stdio, on the other hand, **comes for free as a WASI standard**,
  and merging it would mean throwing that standard mechanism away the
  moment we did.
- What we'd throw away is significant: TinyGo's `fmt.Println`, Rust's
  `println!`, and C's `printf` would stop working, forcing the SDK to
  provide its own output functions. Guest-runtime panic messages go to
  stderr, so suppressing it would hide the reason a sandbox crashed. Shell
  redirection would also be lost.
- **The types don't match.** The mailbox is a mechanism that guarantees the
  boundary of one message, while stdio is a byte stream with no boundaries.
  Delimiting by line would resurrect, through the back door, the very
  delimiter-based scheme we rejected.

So the unifying principle isn't "turn everything into a message," but
rather **"only funnel into the mailbox what needed a custom implementation
anyway."**

Note that the contention over the waiting point never arises for sandboxes
that only do sequential exchanges (a REPL, say). If a configuration ever
needs to wait simultaneously for human input and asynchronous interrupts,
this can be bolted on later, by letting a launch-time option choose to wire
"funnel stdin into the mailbox" as well.

### 4.4 Restrictions

- No means is provided for a guest to initiate an outbound connection.
- No filtering by source IP is performed.
- The source address is not handed to the guest.

#### Why there is no outbound

What's restricted isn't the direction of traffic, but **which side initiates
the connection**.

```
external → ExecSandbox   opening a connection: allowed
external ← ExecSandbox   sending a response: allowed (writing back on an existing connection)
external ← ExecSandbox   opening a connection: not allowed
```

Allowing outbound would let the guest choose its own destination, defeating
the point of pinning destinations to a launch-time allowlist (3.3) — a
mounted file could be exfiltrated to an arbitrary external destination.
Inbound, by contrast, is opened by the external side, so the problem of the
guest choosing where to go never arises in the first place.

#### Why there is no source filter

An allowlist of connection sources, like PostgreSQL's `pg_hba.conf`, is a
mechanism bound up with authentication. ExecSandbox has no authentication
and takes no interest in protocol contents, so there's little reason for it
to own this alone. We leave it to a firewall or reverse proxy.

---

## 5. WASM ABI

### 5.1 Imports

The module name is `execsandbox`.

| Function | Signature |
|---|---|
| `send` | `(dest: i32, ptr: i32, len: i32)` |
| `recv` | `(meta_ptr: i32, buf_ptr: i32, buf_cap: i32, timeout_ms: i32) -> i32` |
| `conn_write` | `(conn_id: i32, ptr: i32, len: i32) -> i32` |
| `max_frame` | `() -> i32` |

The buffer is allocated by the guest. The host only ever writes into the
guest's memory; it never calls the guest's allocator.

#### Why the buffer is guest-owned

The approach where the host calls the guest's allocator through an export
(as wasm-bindgen and similar tools do) varies drastically in how the
allocator is exported across TinyGo, Rust, C, and AssemblyScript, and would
place the heaviest cost right where we wanted it lightest — the goal that
"supporting a new language just means writing one library" (2.1). Having
the guest own the buffer is also simpler in that it introduces no
re-entrancy from host back into guest.

#### The ABI never breaks compatibility

Once an SDK has been distributed, none of the following ever change: an
existing function's signature, the meaning of a return value, the layout of
`meta_ptr`, the existing values of `kind`, or the module name. This is
because guest binaries are built on the user's own machine and may be
distributed and operated independently of the core's version.

Extension happens by adding new functions. If a guest imports a function the
old core doesn't know about, instantiation fails with "undefined import."
Conversely, running an old guest against a new core is fine. So **we
guarantee only backward compatibility, not forward compatibility**.

### 5.2 `send`

Sends `len` bytes starting at `ptr` in linear memory to the given
destination number. Has no return value. See 3.4 for the conditions under
which it is discarded.

#### Why it has no return value

Given that we've decided both an unassigned destination and a full peer
mailbox are silently dropped, a guest must have no way to learn the result.
Making it void lets that semantics show up directly in the ABI's type.

We considered returning a success/failure status, but confirming whether a
message entered the peer's mailbox requires waiting for acknowledgment —
and the moment you wait, it's no longer a Push model but a synchronous RPC.
It would also raise the question of whether to block the whole call when,
under fan-out, only some of several destinations are backed up. Returning
merely "whether it was written to the local send buffer" wouldn't guarantee
anything about reaching the peer, producing the most confusing outcome of
all: "it returned success, but it never arrived."

`conn_write` being the only one with a return value may look asymmetric, but
it concerns the validity of a connID — a piece of dynamic state the guest is
itself tracking — where a mismatch clearly signals a bug, which is why we
treat it differently.

### 5.3 `recv`

Takes one message off the mailbox.

**`timeout_ms`**

| Value | Behavior |
|---|---|
| Negative | Wait indefinitely until a message arrives |
| `0` | Return immediately |
| Positive | Wait up to that many milliseconds |

**Return value**

| Value | Meaning |
|---|---|
| `0` or greater | Success. The value is the payload length, already copied into `buf_ptr`. The message is removed from the mailbox |
| `-1` | Timed out. `meta_ptr` and `buf_ptr` are left unmodified |
| `-2` or less | Buffer too small. The required size is `-(return value) - 1`. No copy happens, and the message stays in the mailbox |

**Metadata**

8 bytes are written to `meta_ptr`.

```
[kind: u32 little-endian][conn_id: u32 little-endian]
```

| `kind` | Category | `conn_id` | Payload |
|---|---|---|---|
| `0` | Sandbox-to-sandbox message | unused | present |
| `1` | External connection: established | valid | none |
| `2` | External connection: data | valid | present |
| `3` | External connection: closed | valid | none |

The three external-connection kinds split the lifetime of one connection
into "start, continue, end." When multiple clients are connected at once,
each connID's events arise independently and interleave with each other as
they flow into the single mailbox. A guest keeps a state table keyed by
connID: creating an entry on establishment, updating it on data, and
discarding it on disconnect.

A "disconnect" event is needed because, without it, a guest would have no
way to know when it's safe to discard the per-connection state it holds
(e.g. mid-parse protocol state), causing a leak.

#### Why it blocks

A WASI Preview 1 WASM module is single-threaded and has neither an event
loop nor callbacks. Making this non-blocking would leave a guest no choice
but to busy-wait, keeping a resident sandbox spinning the CPU. This is the
same reason Erlang's `receive` blocks.

The main motivation for wanting non-blocking behavior is "wanting to watch
multiple waiting points at once," but merging external connections in (4.3)
means there is only ever a single waiting point, so a situation requiring
something like `select` never arises.

A timeout, on the other hand, doesn't create busy-waiting, and without one
the following would be impossible to write:

- **Periodic work**: sending a heartbeat at regular intervals, emitting
  statistics, and so on
- **ACK with a timeout**: having urged (3.7) that delivery guarantees, if
  needed, should be designed as an ACK by the guest, that design can't be
  completed without a way to give up when no response comes back

#### Why metadata is kept separate from the payload

We also considered prepending a fixed-length header (kind + connID) to the
front of the byte string, but that would mean the host writes directly into
the payload itself, breaking the exception-free boundary that "the host
never assigns any meaning to the message's contents" (3.7). Returning it
through a separate channel keeps the payload pure.

Packing the metadata into an i64 return value was rejected because kind +
connID + length don't fit into 64 bits, and any such scheme would bake a
compromise on bit widths into the specification.

A two-phase approach (receiving just the size first, then reading the body)
was a solid runner-up where "buffer too small" is structurally impossible,
but we chose not to adopt it since a single call is simpler when it
suffices.

#### Why the return value expresses both insufficient size and timeout

Grouping them under negative numbers avoids setting up a separate error-code
scheme.

Note that, because `max_frame()` exists, the resize path is essentially
never taken in practice: a guest that calls it once at launch and allocates
a buffer accordingly will fit any message thereafter. The reason the
negative-value return remains part of the spec anyway is as an escape hatch
for configurations with an extremely large `--max-frame`, or for a guest
that deliberately wants to get by with a small buffer.

**When adding a new error condition, `-1`'s neighbor (`-2`, etc.) cannot be
used.** That range is already consumed continuously by the size expression
for buffer shortage.

#### Unknown `kind` values are ignored

Because there is room to add `4` and beyond to `kind`, SDK libraries must
**ignore an unknown kind and continue the loop**. Without this, an old guest
would break once a future version adds new event kinds.

### 5.4 `conn_write`

Writes `len` bytes starting at `ptr` in linear memory to the connection
`conn_id`.

| Return value | Meaning |
|---|---|
| `0` | Success |
| `-1` | Unknown connID |

### 5.5 `max_frame`

Returns the value configured by `--max-frame`, in bytes.

### 5.6 Scope of Custom Implementation

Taking stock of the design, the areas that require a custom implementation
narrow down to two.

| Item | Implementation |
|---|---|
| Arguments/environment variables, filesystem, memory limit, randomness/time, standard I/O | Used as-is from WASI / wazero standards |
| Sandbox-to-sandbox communication (`send`/`recv`) | Custom (WASI standards have no such concept) |
| External-connection bridging (`conn_write`) | Custom (WASI's socket support is still immature) |

The parts that are nothing more than a CLI-flag skin over standard
functionality are, put bluntly, "a thin wrapper around wazero." Where
ExecSandbox carries its own value beyond being a front end for a generic
WASM runtime is precisely these two areas.

The number of new capabilities introduced is three (`send` / `recv` /
`conn_write`); on the ABI, adding the helper function `max_frame` brings the
total to four functions. That the entire external-connection story adds
just one function, `conn_write`, is a direct consequence of the design that
merges receive events into the mailbox (4.3).

#### Usability improvements belong in the SDK

The ABI is a low-level contract, and SDK libraries wrap it in a
language-native skin on top. Things like "hide buffer allocation from the
user," "turn `kind` into an enum," or "accept a timeout as a
`time.Duration`" are all the SDK's job, and are never brought into the ABI.

---

## 6. Build

### 6.1 Command

```
execsandbox-build -o <output file> [--target <GOOS>/<GOARCH>] <input.wasm>
```

- If `--target` is omitted, the platform the builder is running on is
  targeted.
- The builder never bakes in any sandbox policy. All policy is specified at
  launch time.

#### Why policy is never baked in at build time

We also considered a design where mount permissions or a memory limit are
baked in at build time, and launch time can only specify within that
"ceiling," but rejected it.

- It avoids introducing any exception to the principle that "isolation is
  decided by whoever launches the sandbox" (3.3). This keeps a clean split:
  building shapes distribution, it doesn't decide isolation.
- Baking in launch-time wiring (destination assignment, listen address)
  would break the reusability (3.3) that lets the same WASM module be
  repurposed for a different system configuration.
- The builder's spec stays a one-liner — "input: `.wasm`, output: an
  executable" — without the complexity of managing policy in two places.

### 6.2 Output Format

```
[base binary][WASM module][footer]
```

The footer is a fixed 32 bytes containing the following. Integers are
big-endian.

```
Magic(8) + Version(4) + Offset(8) + Length(8) + Reserved(4)
```

At runtime, ExecSandbox reads its own path, reads itself, and consults the
footer to extract the WASM module. If the magic doesn't match, it prints an
error and aborts startup.

#### Why the stamping approach was chosen

Having the user's own `go build` embed the module via Go's `//go:embed`
would require WASM module authors to have a Go toolchain, clashing with the
"any language is fine" policy and the per-language development-kit concept
(2.1). With stamping, the builder is nothing more than file concatenation,
and it can be bundled as-is even in a Rust developer's kit.

Note that **the builder itself embeds six platforms' worth of base binaries
via `//go:embed`**. This produces a two-layer structure: "the builder
embeds, the output is stamped."

### 6.3 Supported Platforms

The builder bundles the following base binaries.

```
linux/amd64,   linux/arm64
darwin/amd64,  darwin/arm64
windows/amd64, windows/arm64
```

#### Why they're bundled

We do not fetch the core at runtime from a release.

- **Version mismatch becomes structurally impossible**: with a fetch
  approach, a state like "builder v1.2 downloads core v1.1" could occur;
  with bundling, the builder's version always equals the version of the
  produced executable. This is the primary reason, more than size.
  
- It works offline and in CI without a network, since nothing needs to be
  fetched.

The cost is the builder's size — roughly 10MB per platform times six
platforms, landing around 50MB. We accepted this trade-off in exchange for
eliminating fetch failures and version-management complexity.

`windows/arm64` is included because real hardware exists for it (Snapdragon
X-based Copilot+ PCs), and Go's cross-compilation (no CGO needed) makes
adding it essentially free. Since the bundling approach requires
re-releasing the builder to add a target, we start out generous.

### 6.4 Code Signing

The generated executable is left unsigned. macOS and Windows may show a
warning on first launch.

Stamping (appending to the end of the binary) breaks any existing code
signature. Since a signature is verified against a hash of the whole file,
an append is indistinguishable from tampering.

The predecessor project ExecDB has shipped and operated unsigned executables
for six platforms without signing ever becoming an issue there, so we
aligned with the same premise here. Should it become necessary, the point
at which to re-sign after stamping is a single, well-defined spot, so this
can be addressed later.

---

## 7. Launch-Time CLI Specification

```
<executable> [options...] [-- arguments to the WASM module...]
```

Since the WASM module is already embedded, there is no argument that
specifies the module.

For the detailed syntax and examples of each option, see
`../usage/execsandbox.md`.

### 7.1 Options

| Short | Long | Argument | Description | Default |
|---|---|---|---|---|
| `-n` | `--name` | `ID` | Own ID | none (does not receive) |
| `-d` | `--dest` | `N=ID` | Assign a destination number. Repeatable | none |
| `-e` | `--env` | `KEY=VALUE` | Environment variable. Repeatable | none |
| `-v` | `--volume` | `HOST:GUEST[:ro]` | Mount a directory. Repeatable | none (no access) |
| `-m` | `--mem-limit` | size | WASM linear memory limit | `512M` |
| `-b` | `--mailbox-limit` | integer | Mailbox capacity in messages | `1024` |
| `-f` | `--max-frame` | size | Maximum bytes per message | `1M` |
| `-l` | `--listen` | address | Listen for external connections. At most one | none (does not listen) |
| `-s` | `--stdio` | stream list | Streams to connect externally | none (all blocked) |
| `-t` | `--timeout` | duration | Execution time limit | none (unlimited) |
| `-a` | `--allow` | item list | Capabilities to allow | none (all denied) |
| `-q` | `--quiet` | — | Suppress host-side logging | logs |
| `-h` | `--help` | — | Help | — |
| `-V` | `--version` | — | Version | — |
| `-L` | `--print-licenses` | — | Print ExecSandbox's and wazero's copyright notices and full license text | — |

A short form and a long form are always provided as a pair. `-h` is reserved
as the fixed slot for `--help`. `-v` is for a volume (the same assignment
Docker uses), not verbose. Version is `-V`.

**Short flags are lowercase in principle, with `-V` and `-L` as the only
exceptions.** `-V` (version) is a necessary exception — it has nowhere else
to go once `-v` is taken by volume, forcing it into uppercase. `-L` (print
licenses) could have picked an unused lowercase letter (`-p`, say), but we
paired it with `-V` in uppercase instead — both share the property of being
"purely informational, uninvolved in the sandbox's actual execution," and we
judged that grouping `-V`/`-L` together in uppercase, among the three
informational options, reads better as a set than having `-L` alone sit
lowercase next to `-h`/`-V`. Everything else picks from unused lowercase
letters (this is why `-a` was assigned to `--allow`).

### 7.2 `--`

Every argument after `--` is passed to the WASM module as its arguments
(WASI's `WithArgs`).

This is standard POSIX/GNU notation, and Go's `flag` package also treats it
as a terminator by default. A bare `-` was not adopted, since on Unix it
carries a strong convention of "referring to stdin/stdout as a file," which
would collide with the stdio option.

### 7.3 Size Notation

Sizes for `-m` and `-f` are interpreted by the following rules.

- The suffixes `K`/`M`/`G` denote powers of 1024 (`1M` = 1,048,576).
- Case-insensitive.
- `Ki`/`Mi`/`Gi` are also accepted with the same meaning.
- A number with no suffix is bytes.

While SI prefixes technically denote powers of 1000, the majority of tools
in a memory context (Docker, the JVM, `dd`, etc.) interpret them as powers
of 1024, so we followed suit. Case-insensitivity accommodates the fact that
Docker uses lowercase and Kubernetes uses uppercase. `Ki`/`Mi`/`Gi` are also
accepted in case a value is copied straight from a Kubernetes manifest.

### 7.4 `-l, --listen` Syntax

| Written as | Listens on |
|---|---|
| `5432` | `127.0.0.1:5432` |
| `127.0.0.1:5432` | same |
| `192.168.1.10:5432` | only the given interface |
| `:5432` | all interfaces |
| `[::1]:5432` | IPv6 loopback |
| `/run/mydb.sock` | Unix domain socket |

- If only a port number is given, a loopback address is filled in (4.1).
- If it starts with a slash, it's treated as a Unix domain socket path. A
  `unix:` prefix is also accepted.
- The host part specifies which interface to listen on; it does not
  restrict connection sources.

We did not adopt the modifiers nginx's `listen` directive carries, such as
`default_server`, `ssl`, `http2`, or `backlog=`. There is no virtual-host
concept, and interpreting TLS or a protocol is entirely the guest's
responsibility.

### 7.5 `-s, --stdio`

Lists the streams connected to the outside (the shell), comma-separated.

| Value | Target |
|---|---|
| `in` | standard input |
| `out` | standard output |
| `err` | standard error |
| `all` | all of the above |

A stream that isn't listed is connected to `io.Discard`.

File paths are not accepted. Routing output is more naturally done via shell
redirection, and accepting a file path as a value would require reintroducing
a "`-` means stdio" notation, sending us right back into the discussion in
7.2.

### 7.6 `-a, --allow`

| Value | Allowed |
|---|---|
| `random` | random number generation |
| `time` | time retrieval |
| `all` | all of the above |

Multiple values are comma-separated. `all` was added for symmetry with
`-s, --stdio`, so the all-at-once notation keeps working even if the list of
items grows later.

Any item not listed (the default) is denied. This unifies randomness and
time with the same allowlist approach used by the other policy items
(filesystem, stdio, etc.), reversing the earlier `-x, --deny` design (list
the items to block, defaulting to allowed) that treated randomness and time
as an exception. That earlier judgment wasn't wrong on its own — it defaulted
randomness and time to allowed because the risk of leaking sensitive host
information through them is low — but we chose to prioritize consistency
across the whole capability model (the guest can do nothing unless explicitly
wired, per `.claude/rules/cli-output.md`) over keeping that one exception.

### 7.7 Host-Side Logging

Logs emitted by ExecSandbox itself are written to standard error, prefixed
with `execsandbox:`. This is kept distinct from the guest's stderr. It can
be suppressed with `-q`.

The prefix exists because, when both flow into the same terminal, it would
otherwise be impossible to tell which side produced a given line. Note that
the defaults are inverted: host-side logs are emitted by default, while the
guest's stderr is blocked by default. This follows the capability principle
that the guest side can do nothing unless explicitly wired, whereas the host
side is information the operator ought to see.

Standard output belongs to the guest; the host never uses it (except for
`--version`/`--help`). If the host wrote to stdout, it would mix with the
guest's own output and break the user's pipeline.

**All host output (logs, error messages, `--help`/`--version`) is in
English.** CLI output is a mechanical target passed through `grep`, pipes,
and log-collection infrastructure, and is also something third parties read
when pasted into an issue, say. Since a user isn't necessarily a speaker of
any particular language, we treat it differently from documentation.

---

## 8. Sandbox Policy

Everything below is specified through launch-time options. Any item left
unspecified takes its default value.

| Item | Implementation | Default |
|---|---|---|
| Filesystem | `WithFSConfig` | no access |
| Standard I/O | `WithStdin` / `WithStdout` / `WithStderr` | all blocked |
| Environment variables / arguments | `WithEnv` / `WithArgs` | none |
| Memory limit | `WithMemoryLimitPages` | `512M` |
| Execution time | `context.WithTimeout` | unlimited |
| Randomness / time | `WithRandSource` / `WithWalltime` / `WithNanotime` | denied |
| Sandbox-to-sandbox communication | custom | no destination assigned |
| External connections | custom | not listening |

Nothing is usable unless explicitly specified (an allowlist approach).
Randomness and time are allowed individually via `-a, --allow` (7.6).

The execution time limit defaults to disabled, since the baseline execution
model is a resident sandbox designed for indefinite uptime — for example, a
loop that keeps waiting in `recv()`. It is positioned as an opt-in feature
for batch-processing-style use cases.

### 8.1 Rationale for the Defaults

| Option | Default | Rationale |
|---|---|---|
| `-m` | `512M` | A generous margin against wasm32's ceiling (4GiB, though many toolchains actually top out around 2GiB). Roomy enough for something SQLite-sized without feeling cramped, while still stopping a runaway guest reasonably early |
| `-f` | `1M` | More than enough width for exchanging SQL statements or JSON. A single message over 1MB is already unusual |
| `-b` | `1024` | The product with `-f` (1M × 1024 = 1G) becomes the mailbox's memory ceiling. A depth that can absorb a fan-in burst |

**These are ceilings, not pre-allocations.** WASM linear memory grows as
needed, and the mailbox only consumes memory for what's actually queued.
The idle-time real footprint, Go runtime included, is on the order of a few
MB, so leaning generous on the defaults doesn't increase memory use under
normal operation. What it affects is purely "where a runaway guest gets
stopped."

The process's overall memory ceiling is roughly the sum of the WASM side
(`-m`) and the host-side mailbox (`-b` × `-f`), since they're separate
pools. With the defaults, that's roughly 512M + 1G ≈ 1.5G.

Depth and per-message size can be traded off depending on use case. A
configuration with a high volume of small messages could use `-f 64K -b
16384` to keep the same 1G ceiling while getting 16 times the depth.

---

## 9. Limitations

The following are out of scope for v1. Each is designed so it can be added
later without breaking the existing design.

| Item | Shape it would take if added |
|---|---|
| Cross-machine distribution (3.1) | Extend destination-ID resolution from Unix domain sockets to TCP |
| Named pipes on Windows (3.1) | Add a branch for `go-winio` or similar once a Windows-specific problem surfaces |
| Outbound connections (4.4) | Fix the destinations into an allowlist at launch time, and have the guest select by number |
| Filtering by source IP (4.4) | — (left to the OS's own mechanisms) |
| Multiple listen endpoints (4.1) | Add a listener number to event metadata |
| Passing along the source address (4.4) | Add `conn_peer(conn_id, ptr, cap) -> i32` |
| Funneling stdin into the mailbox (4.3) | Let a launch-time option choose this wiring |
| Rich terminal control (raw mode, history, completion, interrupts) | — |
| An external management interface (2.2) | — (expressed via a management-role ExecSandbox) |
| A mechanism for managing multiple processes (2.2) | — (left to supervisord or similar) |
| Code signing (6.4) | Add it as a builder option |

---

## 10. License

ExecSandbox (core and builder) and its SDK libraries are under the MIT
License.

It carries the fewest restrictions and is widely used for libraries and
tools. Given that the SDK library gets linked into the guest's own code, it
is desirable not to impose any license constraints on the user's project.

The main dependency, wazero, is under the Apache License 2.0, which is
compatible with MIT.

### 10.1 Handling of the Generated Executable

The executable produced by the builder is a base binary (containing the
ExecSandbox core and wazero) with a WASM module appended (6.2). The output
therefore contains code from both ExecSandbox and wazero.

Distributing the output to a third party carries the following notice
obligations.

| Subject | License | Required action |
|---|---|---|
| ExecSandbox | MIT | Include the copyright notice and full license text |
| wazero | Apache-2.0 | Include the copyright notice, full license text, and the NOTICE file, if any |

Because the stamping approach means "the builder just spits out a binary,"
users can easily lose sight of this fact. In addition to stating it in the
documentation, **the generated executable itself** can print the text
needed to satisfy the above obligations (copyright notices, full license
text, NOTICE) to standard output via `-L, --print-licenses` (7.1).

We put the party responsible for satisfying this obligation on the output
itself, rather than on the builder, because it's the output that gets
distributed to third parties, and a distributor isn't guaranteed to still
have access to the builder. If the output itself can print the text, the
obligation can be met even if the distributor has lost access to the
builder.

The WASM module's own license is entirely up to its author to choose.
Neither MIT nor Apache-2.0 is copyleft, so the output as a whole can be
distributed under any license, including a proprietary one.

---

日本語版: [execsandbox_spec_ja.md](execsandbox_spec_ja.md)
