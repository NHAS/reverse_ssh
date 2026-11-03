# RSSH Client Configuration

The RSSH client can be configured by either embedding values within the binary at build time, adding CLI arguments, or by providing some environment variables. 


- [RSSH Client Configuration](#rssh-client-configuration)
  - [CLI](#cli)
    - [`-d` / `--destination`](#-d----destination)
    - [`--destination-file`](#--destination-file)
    - [`--foreground`](#--foreground)
    - [`--fingerprint`](#--fingerprint)
    - [`--fingerprint-file`](#--fingerprint-file)
    - [`--proxy`](#--proxy)
    - [`--ntlm-proxy-creds`](#--ntlm-proxy-creds)
    - [`--process_name`](#--process_name)
    - [`--sni`](#--sni)
    - [`--log-level`](#--log-level)
    - [`--version-string`](#--version-string)
    - [`--private-key-path`](#--private-key-path)
    - [`--connect-timeout`](#--connect-timeout)
    - [`--use-kerberos` (Windows only)](#--use-kerberos-windows-only)
  - [Makefile](#makefile)
    - [`IGNORE`](#ignore)
    - [`RSSH_HOMESERVER`](#rssh_homeserver)
    - [`RSSH_FINGERPRINT`](#rssh_fingerprint)
    - [`RSSH_PROXY`](#rssh_proxy)
    - [`CGO_ENABLED`](#cgo_enabled)
  - [`link` command](#link-command)
    - [`--fingerprint`](#--fingerprint-1)
    - [`--http`, `--https`, `--wss`, `--ws`, `--stdio`, `--tls`](#--http---https---wss---ws---stdio---tls)
    - [`--log-level`](#--log-level-1)
    - [`--no-lib-c`](#--no-lib-c)
    - [`--ntlm-proxy-creds`](#--ntlm-proxy-creds-1)
    - [`--proxy`](#--proxy-1)
    - [`--sni`](#--sni-1)
    - [`--use-kerberos`](#--use-kerberos)
    - [`-s`](#-s)



## CLI

### `-d` / `--destination`
RSSH server address.

### `--destination-file`
Read server address from file

### `--foreground`
Causes the client to run without forking to background.

### `--fingerprint`
Print out embedded server public key SHA256 hex fingerprint.

### `--fingerprint-file`
Read server public key SHA256 hex fingerprint from file path

### `--proxy`
Address of `HTTP` connect proxy to use

### `--ntlm-proxy-creds`
Use NTLM credentials to connect to proxy, in format `DOMAIN\\USER:PASS`

### `--process_name`
Causes the RSSH client to try and set its process name which is shown in tasklist/process list

### `--sni`
When using TLS set the clients requested [Server Name Indication](https://en.wikipedia.org/wiki/Server_Name_Indication). Helpful for reverse or forward proxies which require some indication for what the client is intending to connect to. 

### `--log-level`
Change logging output levels, [INFO,WARNING,ERROR,FATAL,DISABLED]

|  Level |  Purpose |
|---|---|
|  INFO |  Print all log messages, including joining/leaving and connection information | 
|  WARNING | Only print things that need attention | 
|   ERROR|  Print things that have caused an unrecoverable state | 
|FATAL| Only output when something has caused the client to crash|
|DISABLED| Print nothing and be happy about it|

### `--version-string`
Set the SSH version string to use, i.e `SSH-VERSION`, defaults to `SSH-internal.Version-runtime.GOOS_runtime.GOARCH`.
e.g

`SSH-v2.8.1-23-g35f2725-darwin_arm64`

### `--private-key-path`
Optional path to unencrypted SSH key to use for connecting

### `--connect-timeout`
Duration to wait for initial connection seconds, default `180` seconds, set to `0` to wait indefinitely

### `--use-kerberos` (Windows only)
Fetch the kerberos ticket from the current session and use it for authentication with the proxy server (if proxy server specified).


## Makefile

When building with the `Makefile` there are multiple options for configuring the client using environment variables.

This is done by baking in strings into the binary with the `-X` golang linker command. 
e.g
```sh
# Set the destination variable in the main.go file
ifdef RSSH_HOMESERVER
	LDFLAGS += -X main.destination=$(RSSH_HOMESERVER)
endif
```

```sh
RSSH_HOMESERVER=rssh.server IGNORE=true make client
```

### `IGNORE`

Cause the RSSH client to ignore all flags supplied by the operating system and just operate off baked in details. 

For example. 

If you are using the RSSH client in a place where it gets a bunch of junk args:
```sh
./client -d jaskjdaj -something blah
```

You can cause it to ignore everything by doing 
```sh
IGNORE=true RSSH_HOME_SERVER=rssh.server make client
```

### `RSSH_HOMESERVER`
The RSSH server address (same as `-d`)

### `RSSH_FINGERPRINT`
Set the server fingerprint. Similiar to the `--fingerprint-file` however this bakes it into the binary itself.

### `RSSH_PROXY`
Set the RSSH proxy to use. This must be a single URL with a scheme a scheme in the allowed list (shown below).
If no port is specified, the client will set a default port. 

| Scheme | Port |
| -- | -- |
| `http` | 80 |
| `https`| 443 |
| `socks`| 1080 |
| `socks5`| 1080 |
| `socks4`| 1080|


### `CGO_ENABLED`
Build the RSSH client without CGO, important for systems that dont use glibc, such as alpine. 

## `link` command

The `link` server console command has many options that are fully detailed in the [server console reference](../server/console_reference.md). 

All options listed here are only to do with configuring the RSSH client details rather than the binary format/packing/etc.

So go read that if you need more information.

All the following arguments are intended to be used after connecting to the server console. 

e.g

```sh
catcher$ link --name fred --no-glib-c
```

### `--fingerprint`
Set the baked in server fingerprint, typically you will not need to do this as the link command will take the current running server public key automatically.

Same as [`RSSH_FINGERPRINT`](#rssh_fingerprint).

### `--http`, `--https`, `--wss`, `--ws`, `--stdio`, `--tls`
These options just set the scheme of the URL that is baked into the binary.


### `--log-level`
Set the log level within the binary. 
Same as [`--log-level`](#--log-level)

### `--no-lib-c`
Build the binary without CGO, see [`CGO_ENABLED`](#cgo_enabled).

### `--ntlm-proxy-creds`

Set static ntlm proxy credentials into the binary. This setting is not compatiable with [`--use-kerberos`](#--use-kerberos) which fetches the current sessions kerberos ticket dynamically.

### `--proxy`

See [`--proxy`](#--proxy)

### `--sni`

See[`--sni`](#--sni)

### `--use-kerberos`

See [`--use-kerberos` (Windows only)](#--use-kerberos-windows-only)

### `-s`

See [`-d` / `--destination`](#-d----destination)