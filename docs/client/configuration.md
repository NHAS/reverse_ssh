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
    - [`--use-kerberos`](#--use-kerberos)



## CLI

### `-d` / `--destination`
RSSH Server address.

### `--destination-file`
Read server connect back address as file

### `--foreground`
Causes the client to run without forking to background

### `--fingerprint`
Server public key SHA256 hex fingerprint for auth

### `--fingerprint-file`
Read server public key SHA256 hex fingerprint from file path

### `--proxy`
Location of HTTP connect proxy to use

### `--ntlm-proxy-creds`
NTLM proxy credentials in format `DOMAIN\\USER:PASS`

### `--process_name`
Process name shown in tasklist/process list

### `--sni`
When using TLS set the clients requested SNI to this value

### `--log-level`
Change logging output levels, [INFO,WARNING,ERROR,FATAL,DISABLED]

### `--version-string`
SSH version string to use, i.e SSH-VERSION, defaults to internal.Version-runtime.GOOS_runtime.GOARCH

### `--private-key-path`
Optional path to unencrypted SSH key to use for connecting

### `--connect-timeout`
Duration to wait for initial connection seconds, default 180, set to 0 to wait indefinitely

### `--use-kerberos`
Use kerberos authentication on proxy server (if proxy server specified)