# Help

## Windows

### SFTP

Due to the limitations of the SFTP library. Paths need a little more effort on windows.

```sh
sftp -r -J your.rssh.server.internal:3232 test-pc.user.test-pc:'/C:/Windows/system32'
```

Note the `/` before the starting character.

## Session spawn errors (`0xc0000142`)

Under some execution circumstances connecting to an RSSH client on windows may fail with no error. 

```sh
catcher$ connect windows-system
Session has terminated.
```

Client logs:
```sh
2025/08/24 18:25:39 [client] INFO session.go:52 func16() : Session got request: "shell"
2025/08/24 18:25:39 [client] INFO shell_windows.go:137 runWithConpty() : New process with pid 3427 spawned
2025/08/24 18:25:39 [client] INFO session.go:122 func16() : Session disconnected
```

There are two common causes for this, the first being antivirus has killed the spawned powershell, and the other `0xc0000142` is when the resulting process does not have the permissions to access the Windows Station or Desktop [source](https://stackoverflow.com/questions/677874/starting-a-process-with-credentials-from-a-windows-service/30687230#30687230).


To determine which is causing this issue, execute any command without a pty: 

```sh
ssh -J rssh windows-system cmd /c dir                                
exit status 0xc0000142
```

If you see the `0xc0000142` error code try starting `CMD.exe` and force allocating a pty (`-t`):

```sh
ssh -t -J rssh windows-system CMD.exe
```

This should start an interactive shell.

## Server started with `--insecure` still has `Failed to handshake`

If the client binary was generated with the `link` command this client has the server public key fingerprint baked in by default. If you lose your server private key, the clients will no longer be able to connect.
You can also generate clients with `link --fingerprint <fingerprint here>` to specify a fingerprint, there isnt currently a way to disable this as per version 1.0.13.

## Foreground vs Background

By default, clients will run in the background then the parent process will exit, the child process will be given the parent processes `stdout`/`stderr` so you will be able to see output.   
If you need to debug your client, use the `--foreground` flag.
