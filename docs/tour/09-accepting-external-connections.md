# 9. Accepting External Connections

ExecSandbox can be reached not just by other sandboxes, but from outside
too (a TCP client, say). `-l/--listen` opens a listener.

Below is a minimal TCP echo server that does nothing but write back
whatever data it receives.

```go
func main() {
	for {
		msg, ok := execsandbox.Recv(-1)
		if !ok {
			continue
		}
		if msg.Kind == execsandbox.KindConnData {
			execsandbox.ConnWrite(msg.ConnID, msg.Data)
		}
	}
}
```

```
$ tinygo build -target=wasip1 -o echo.wasm .
$ execsandbox-build -o echo echo.wasm
$ ./echo -l 19001 &
```

Let's connect using bash's `/dev/tcp` feature, with no extra tools (works
even without `nc`).

```
$ exec 3<>/dev/tcp/127.0.0.1/19001
$ printf 'hello via bash' >&3
$ head -c 14 <&3
hello via bash
```

Whatever we sent came right back. All that's used here is `Recv` (**the
same receiving point** as the sandbox-to-sandbox messaging from the
previous chapters) and `ConnWrite`, which writes back to an external
connection.

---
[← Previous: 8. Delivery Is Not Guaranteed](08-delivery-is-not-guaranteed.md) | [Index](README.md) | [Next: 10. Telling Events Apart →](10-telling-events-apart.md)

日本語版: [09-accepting-external-connections_ja.md](09-accepting-external-connections_ja.md)
