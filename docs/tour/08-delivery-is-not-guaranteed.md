日本語版: [08-delivery-is-not-guaranteed_ja.md](08-delivery-is-not-guaranteed_ja.md)

# 8. Delivery Is Not Guaranteed

Let's run just `nodeA` (the sender) from the previous chapter, without
starting `nodeB` (the receiver).

```
$ ./nodeA -d 1=nodeB
$ echo $?
0
```

No error appears at all. It just quietly exits — **when the destination
isn't running, `Send` silently drops the message.** This isn't a bug; it's
the specification.

- The destination is unassigned
- The peer sandbox isn't running
- The peer's mailbox is full (tail-drop)

None of these can be distinguished from the sender's side. `Send` is a
Push model, fire-and-forget — the mechanism for returning an
acknowledgment simply doesn't exist.

That's why it wasn't a coincidence that the previous chapter's command
example started `nodeB` first. When timing can't be guaranteed in
practice, you need to either retry the sender until it gets through, or
design an ACK message at the application level.

---
[← Previous: 7. Connecting Two Sandboxes](07-connecting-two-sandboxes.md) | [Index](README.md) | [Next: 9. Accepting External Connections →](09-accepting-external-connections.md)
