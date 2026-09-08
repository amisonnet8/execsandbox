# external-connection — echoing back an external connection

Opens a listener outside of ExecSandbox (`-l/--listen`) and writes back
whatever data it receives — a minimal TCP echo server. Uses the TinyGo
SDK's `Recv`/`ConnWrite`.

Source: [`execsandbox-sdk`'s `go/examples/echo`](https://github.com/amisonnet8/execsandbox-sdk/tree/main/go/examples/echo)

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

## Build

```
$ tinygo build -target=wasip1 -o echo.wasm .
$ execsandbox-build -o echo echo.wasm
```

## Run

```
$ ./echo -l 19001 &
```

Connect and send data, and it comes right back. You can try this with no
extra tools, using bash's `/dev/tcp` feature (works even without `nc`).

```
$ exec 3<>/dev/tcp/127.0.0.1/19001
$ printf 'hello via bash' >&3
$ head -c 14 <&3
hello via bash
```

## Discussion

- **`msg.Kind` distinguishes three kinds of events.**
  `KindConnEstablished` (connection established), `KindConnData` (data
  arrived), and `KindConnClosed` (disconnected) all funnel into the same
  `Recv` loop (the very same receiving point as the `KindMessage` used for
  sandbox-to-sandbox messages). This echo ignores everything but
  `KindConnData` — already following the practice spec §5.3 recommends:
  ignore an unknown or unused `kind` rather than treating it as an error.
- **`msg.ConnID` identifies the connection.** Even with multiple clients
  connected at once, they're distinguished by connID, but since this echo
  simply writes back to whichever connID it received on, it can handle
  multiple connections concurrently (it holds no internal state).
- **`-l` accepts only one address.** If you need to listen on multiple
  ports, start multiple ExecSandbox instances and connect them with
  sandbox-to-sandbox messaging
  ([`sandbox-messaging.md`](sandbox-messaging.md)).
- The source address can't be obtained (a limitation noted in spec §4.4).

---

日本語版: [external-connection_ja.md](external-connection_ja.md)
