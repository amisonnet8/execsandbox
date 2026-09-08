# 7. Connecting Two Sandboxes

From here on we use the TinyGo SDK
([`execsandbox-sdk`](https://github.com/amisonnet8/execsandbox-sdk))'s
`Send`/`Recv`. The SDK hides pointers, lengths, and buffer allocation, so
the guest-side code stays simple.

`sender` sends once to destination number 1, then exits.

```go
execsandbox.Send(1, []byte("hello from execsandbox-sdk"))
```

`receiver` blocks until one message arrives, prints it, and exits.

```go
msg, ok := execsandbox.Recv(-1) // negative = wait indefinitely
if ok {
	fmt.Printf("kind=%d data=%s\n", msg.Kind, msg.Data)
}
```

Build and embed both.

```
$ tinygo build -target=wasip1 -o sender.wasm .    # in sender/
$ tinygo build -target=wasip1 -o receiver.wasm .  # in receiver/
$ execsandbox-build -o nodeA sender.wasm
$ execsandbox-build -o nodeB receiver.wasm
```

Name your own ID with `-n`, and assign an ID to a destination number with
`-d N=ID`. Start the receiver first with `-n nodeB`, then have the sender
send with `-d 1=nodeB`.

```
$ ./nodeB -n nodeB -s out &
$ ./nodeA -d 1=nodeB
$ wait
kind=0 data=hello from execsandbox-sdk
```

`kind=0` represents `KindMessage` (a sandbox-to-sandbox message) — other
`kind` values show up later, in the external-connections chapters.

What's worth noticing is that **it's the launch options, not the guest's
code, that decide how a destination resolves.** The guest side only
specifies `Send(1, ...)` and a number; who number 1 actually refers to is
decided by the launch-time `-d 1=nodeB`. The destination can be rewired
(`-d 1=some-other-sandbox`) without touching the guest's code at all.

---
[← Previous: 6. The Mailbox Idea](06-the-mailbox-idea.md) | [Index](README.md) | [Next: 8. Delivery Is Not Guaranteed →](08-delivery-is-not-guaranteed.md)

日本語版: [07-connecting-two-sandboxes_ja.md](07-connecting-two-sandboxes_ja.md)
