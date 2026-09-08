日本語版: [sandbox-messaging_ja.md](sandbox-messaging_ja.md)

# sandbox-messaging — sandbox-to-sandbox messaging

A minimal setup connecting two ExecSandbox instances through an
Erlang-style mailbox. Uses the TinyGo SDK
([`execsandbox-sdk`](https://github.com/amisonnet8/execsandbox-sdk))'s
`Send`/`Recv`.

Source: [`execsandbox-sdk`'s `go/examples/sender`](https://github.com/amisonnet8/execsandbox-sdk/tree/main/go/examples/sender) ·
[`go/examples/receiver`](https://github.com/amisonnet8/execsandbox-sdk/tree/main/go/examples/receiver)

```go
// sender: sends once to destination number 1, then exits
execsandbox.Send(1, []byte("hello from execsandbox-sdk"))
```

```go
// receiver: blocks until one message arrives, prints it, and exits
msg, ok := execsandbox.Recv(-1) // negative = wait indefinitely
if ok {
	fmt.Printf("kind=%d data=%s\n", msg.Kind, msg.Data)
}
```

## Build

```
$ tinygo build -target=wasip1 -o sender.wasm .    # in sender/
$ tinygo build -target=wasip1 -o receiver.wasm .  # in receiver/
$ execsandbox-build -o nodeA sender.wasm
$ execsandbox-build -o nodeB receiver.wasm
```

## Run

Name your own ID with `-n`, and assign an ID to a destination number with
`-d N=ID`. Start the receiver first with `-n nodeB`, then have the sender
send with `-d 1=nodeB`.

```
$ ./nodeB -n nodeB -s out &
$ ./nodeA -d 1=nodeB
$ wait
kind=0 data=hello from execsandbox-sdk
```

## Discussion

- **`kind=0` is `KindMessage`** (a sandbox-to-sandbox message). It's
  distinguished through the same `Recv` return value as external-connection
  events (`KindConnEstablished`/`KindConnData`/`KindConnClosed`, kind=1
  through 3; spec §5.3).
- **The launch options decide how a destination resolves.** The guest side
  only ever specifies `Send(1, ...)` and a number; who number 1 actually
  refers to is decided by the launch-time `-d 1=nodeB`. The destination
  wiring can be rearranged without touching the guest's code.
- **`Send` doesn't guarantee delivery.** If the sender starts first and
  sends before the receiver has started listening via `-n`, the message is
  silently dropped as an unassigned destination (spec §3.4). As in the
  command example above, retrying the sender until it gets through is
  necessary in practice (a production orchestration would design its own
  ACK at the application level).
