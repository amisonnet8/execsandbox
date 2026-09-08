# 13. Exposing Files

`-v/--volume` mounts a host directory into the guest. By default, there is
no access path to the filesystem at all.

```
$ tinygo build -target=wasip1 -o file-access.wasm .
$ execsandbox-build -o file-access file-access.wasm
$ ./file-access -s out
write failed: open /data/hello.txt: file does not exist
```

The "nothing by default" we saw in chapter 3 holds consistently for the
filesystem too. Mounting with `-v HOST:GUEST` makes only the guest-side
`GUEST` path (here, `/data`) and everything under it visible.

```
$ mkdir hostdata
$ ./file-access -v "$PWD/hostdata:/data" -s out
write ok
read back: written by the guest
$ cat hostdata/hello.txt
written by the guest
```

Appending `:ro`, as in `HOST:GUEST:ro`, makes it read-only. What the guest
sees is only the contents of the mounted directory — never the host's
filesystem as a whole.

---
[← Previous: 12. Capping Memory](12-capping-memory.md) | [Index](README.md) | [Next: 14. Denying Randomness and Time →](14-denying-random-and-time.md)

日本語版: [13-exposing-files_ja.md](13-exposing-files_ja.md)
