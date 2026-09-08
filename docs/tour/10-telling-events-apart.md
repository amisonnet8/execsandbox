# 10. Telling Events Apart

Did you notice that the previous chapter's echo only ever checked
`msg.Kind == execsandbox.KindConnData`? `Recv` delivers several kinds of
events through a single receiving point. Let's lay out what we've seen so
far.

| `Kind` | Meaning | `ConnID` | `Data` |
| :--- | :--- | :--- | :--- |
| `KindMessage` | Sandbox-to-sandbox message (chapter 7) | — | present |
| `KindConnEstablished` | An external connection was established | valid | none |
| `KindConnData` | Data arrived on an external connection (chapter 9) | valid | present |
| `KindConnClosed` | An external connection closed | valid | none |

The previous chapter's echo ignores connection establishment and closure
(`KindConnEstablished`/`KindConnClosed`) even when it receives them. This
isn't an error — it's the recommended practice: **don't treat an unknown
or unused `kind` as an error; ignore it and keep the loop going.** A guest
that follows this practice won't break even when ExecSandbox adds a new
`kind` in the future.

`msg.ConnID` identifies the connection. Even with multiple clients
connected at once, they can be told apart by `ConnID` (this is exactly why
the previous chapter's echo could handle multiple connections
concurrently just by writing back to whichever connID it received on).

Worth mentioning too: `-l` accepts only one address. If you need multiple
listeners, start multiple ExecSandbox instances and connect them with the
sandbox-to-sandbox messaging from chapter 7. There's also no way to obtain
the source address of a connection.

---
[← Previous: 9. Accepting External Connections](09-accepting-external-connections.md) | [Index](README.md) | [Next: 11. Stopping a Runaway Guest →](11-stopping-a-runaway-guest.md)

日本語版: [10-telling-events-apart_ja.md](10-telling-events-apart_ja.md)
