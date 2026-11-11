
# RSSH Server Configuration

- [RSSH Server Configuration](#rssh-server-configuration)
  - [Container](#container)
  - [Config file](#config-file)
  - [CLI / Environment Variables](#cli--environment-variables)
    - [Data](#data)
      - [`--datadir`](#--datadir)
    - [Authorisation](#authorisation)
      - [`--insecure`](#--insecure)
      - [`--openproxy`](#--openproxy)
    - [Network](#network)
      - [`--tls`](#--tls)
      - [`--tlscert`](#--tlscert)
      - [`--tlskey`](#--tlskey)
      - [`--webserver` (Depreciated)](#--webserver-depreciated)
      - [`--enable-client-downloads`](#--enable-client-downloads)
      - [`--external_address`](#--external_address)
      - [`--timeout`](#--timeout)
    - [Utility](#utility)
      - [`--fingerprint`](#--fingerprint)
      - [`--log-level` / `RSSH_LOG_LEVEL`](#--log-level--rssh_log_level)
      - [`--console-label` / `RSSH_CONSOLE_LABEL`](#--console-label--rssh_console_label)
      - [`--config`](#--config)


## Container

The container deployment requires that `/data` is mounted so that the server can store client keys, download links and other information. 

Additionally, as container deployments often sit behind a TCP proxy `EXTERNAL_ADDRESS` is required when starting up the RSSH server. 


In the cases that a config file is being used, you must put the config file in `/data/config.toml` 

Otherwise the server will be started with the following arguments:
- `--datadir /data`
- `--enable-client-downloads`
- `--tls` 
- `--external_address $EXTERNAL_ADDRESS`
- Internal container listen address: `:2222`

## Config file 

The description of these settings are identical to the cli/environment variables of the same name.

```toml
# Security Settings
insecure = false
open_proxy = false

# TLS Settings
tls = true
tls_cert = "server.crt"
tls_key = "server.key"

# Network Settings
listen_address = ":2222"
external_address = "your.server.com:2222"
timeout = 50

# Features
enable_client_downloads = true

# Paths
data_dir = "./data"

# Logging
log_level = "info"
console_label = "catcher"
```

## CLI / Environment Variables

### Data

#### `--datadir`

The RSSH server data directory path is where the `keys`, `downloads`, `authorized_keys` and `authorized_controllee_keys` are populated. 

This is the directory where all the settings for webhooks, watch logs and other data is stored. 

It currently defaults to working directory.

Under the *docker* container it defaults to `/data`

### Authorisation

#### `--insecure`
Ignore the data directory `authorized_controllee_keys` file and allow any RSSH client to connect.  
If an RSSH client has been generated with the `link` command this will not enable access as the client checks the server public key. 

#### `--openproxy`
This option is for advanced users. 

The `ssh` tool natively can do dynamic remote forwards `-R` which effectively allows access to the network of that machine. 

So you can do the following on a remote system:

```sh
ssh -R 1080 -N -f user@remote-server
```

This will open `1080` on the *RSSH server* side, then you are able to connect to that socket and send traffic to the *connecting* machines network. 

As this is able to open a port on the RSSH server, it is restricted to opening the port only on the loopback interface (`127.0.0.1`).

It is also not enabled by default, or in the docker container.

### Network

#### `--tls`
Enable TLS on RSSH server socket (ssh/http over TLS). 
Without the `--tlscert` or `--tlskey` options this will automatically generate a server cert with the following attributes:

| Attribute | Value |
| -- | -- |
| Country| `US` |
| Organization | `Cloudflare Inc` |
| Validity| `30 days` |
| Is CA | `true` |
| Common Name | `--external_address` | 

#### `--tlscert`
TLS certificate path. This option will only try and load the certificate on connection. 

#### `--tlskey`
TLS key path. This option will only try and load the certificate on connection. 

#### `--webserver` (Depreciated)
Please see the [`--enable-client-downloads`](#--enable-client-downloads) option.


#### `--enable-client-downloads`

This option enables the build and serve portion of the RSSH server. 
This is used with the `link` command. 

It starts up an HTTP, HTTPS and TCP server on the current RSSH listening port. 

#### `--external_address`
This option is used to bake into the RSSH clients when built with the `link` command. 

It is only useful when the externally reachable address of the RSSH server is different from the listening address. 

Example:

```sh
# Listening address is on the 192.168.20.1 interface on 3232 port
# But the RSSH clients will have rssh.domain baked in
./server --external_address rssh.domain 192.168.20.1:3232
```

If no port is specified in the external address then various parts of the RSSH server will automatically determine one. 

For the RSSH server shell generator (appending `.sh` to RSSH server links) it will default to port `80`

Otherwise please reference this table.

| Scheme | Description| Default Port |
| -- | -- | --- | 
| `n/a` / `empty`| No scheme specified | 22 | 
|`unknown`| If a unsupported scheme is provided| 22 |
| `tls` | A HTTP TLS  connection| 443 | 
| `ws` | A websockets server connection | 80  |
| `wss` | A secure websockets connection | 443  |
| `stdio` | A connection via STDIN and STDOUT |  N/A  |

#### `--timeout`
Set RSSH client timeout (when a client is considered disconnected) defaults to *5 seconds*, if set to *0 timeout is disabled*.

This sets timeouts on both the server and RSSH client. 
The RSSH client uses SSH based timeouts by sending pings at half the interval of the provided timeout. 

The server uses SSH based timeouts same as the client, but also sets TCP timeouts at the operating system/socket level. 

There is also some added complexity around some of the more advanced RSSH transports, such as HTTP polling mode where these timeouts may only happen via SSH.


### Utility

#### `--fingerprint`
Print a hex encoded SHA256 server public key fingerprint and exit. This will also generate a public private keypair for the RSSH server if none is present

E.g

```sh
./server --fingerprint
41c6aa0012431e67144055c17d1289f90dc378a2b837fbd9efc78b814117d9cd
```

#### `--log-level` / `RSSH_LOG_LEVEL`
Change logging server output levels (will also set default log level for generated clients).

|  Level |  Purpose |
|---|---|
|  INFO |  Print all log messages, including joining/leaving and connection information | 
|  WARNING | Only print things that need attention | 
|   ERROR|  Print things that have caused an unrecoverable state in a connection of server session | 
|FATAL| Only output when something has caused the server or client to crash|
|DISABLED| Print nothing and be happy about it|

#### `--console-label` / `RSSH_CONSOLE_LABEL`
Change server console label.  (Default: catcher)

E.g 

```sh
./server --console-label toaster 127.0.0.1:3232
```

Connect with SSH 
```sh
ssh localhost -p 3232
toaster$ ls
```

#### `--config`
Instead of configuring the RSSH server via cli arguments you can also set these within a `TOML` file. 

Optional TOML config file path

```sh
./server --config example.conf
```