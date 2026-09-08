# 6. The Mailbox Idea

Every ExecSandbox instance holds one **mailbox** of its own — the same
idea as an Erlang process. Anything arriving from another sandbox or an
external connection is first queued into this mailbox, and the guest takes
messages out one at a time with `Recv`.

Two properties matter here.

- **Sending is one-way (a Push model).** `Send` has no return value. There
  is no way to confirm whether the peer received it. If you need delivery
  confirmation, design your own ACK at the application level.
- **The mailbox has a cap.** Once it's full, a newly arriving message is
  the one that gets discarded (tail-drop), not an existing one — the order
  of what's already queued is never disturbed. It also never grows
  without bound and eats up memory.

Next, we'll use this mechanism to connect two sandboxes.

---
[← Previous: 5. Arguments and Environment Variables](05-args-and-env.md) | [Index](README.md) | [Next: 7. Connecting Two Sandboxes →](07-connecting-two-sandboxes.md)

日本語版: [06-the-mailbox-idea_ja.md](06-the-mailbox-idea_ja.md)
