
# RSSH Server CLI Arguments

- [RSSH Server CLI Arguments](#rssh-server-cli-arguments)
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
    - [`--log-level`](#--log-level)
    - [`--console-label`](#--console-label)
    - [`--config`](#--config)



## Data

### `--datadir`
Directory to search for keys, config files, and to store compile cache (defaults to working directory)

## Authorisation

### `--insecure`
Ignore authorized_controllee_keys file and allow any RSSH client to connect
### `--openproxy`
Allow any ssh client to do a dynamic remote forward (-R) and effectively allowing anyone to open a port on localhost on the server

## Network

### `--tls`
Enable TLS on socket (ssh/http over TLS)

### `--tlscert`
TLS certificate path

### `--tlskey`
TLS key path

### `--webserver` (Depreciated)
Enable webserver on the `listen_address` port

### `--enable-client-downloads`
Enable webserver and raw TCP to download clients

### `--external_address`
If the external IP and port of the RSSH server is different from the listening address, set that here

### `--timeout`
Set rssh client timeout (when a client is considered disconnected) defaults, in seconds, defaults to *5 seconds*, if set to *0 timeout is disabled*.


## Utility

### `--fingerprint`
Print fingerprint and exit. (Will generate server key if none exists)

### `--log-level`
Change logging server output levels (will also set default log level for generated clients).

|  Level |  Purpose |
|---|---|
|  INFO |  Print all log messages, including joining/leaving and connection information | 
|  WARNING | Only print things that need attention | 
|   ERROR|  Print things that have caused an unrecoverable state in a connection of server session | 
|FATAL| Only output when something has caused the server or client to crash|
|DISABLED| Print nothing and be happy about it|

### `--console-label`
Change console label.  (Default: catcher)

### `--config`
Optional TOML config file path
