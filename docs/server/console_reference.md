# Server Console Commands

To use the server console you must connect to your rssh server instance directly. 
Some commands may not be avaiable if you are not connected as an administrator (privilege 5).

+--------------+--------------------------------------------------------------------------------+
| Function     | Purpose                                                                        |
+--------------+--------------------------------------------------------------------------------+
| [access](#access)       | Temporarily share/unhide client connection.                                    |
+--------------+--------------------------------------------------------------------------------+
| [autocomplete](#autocomplete) | Generate bash/zsh autocompletion, or match clients and return list of ids      |
+--------------+--------------------------------------------------------------------------------+
| clear        | Clear server console                                                           |
+--------------+--------------------------------------------------------------------------------+
| [connect](#connect)      | Start shell on remote controllable host.                                       |
+--------------+--------------------------------------------------------------------------------+
| [exec](#exec)         | Execute a command on one or more rssh client                                   |
+--------------+--------------------------------------------------------------------------------+
| exit         | Close server console                                                           |
+--------------+--------------------------------------------------------------------------------+
| help         | Get help for commands, or display all commands                                 |
+--------------+--------------------------------------------------------------------------------+
| [kill](#kill)         | Stop the execute of the rssh client.                                           |
+--------------+--------------------------------------------------------------------------------+
| [link](#link)         | Generate client binary and return link to it                                   |
+--------------+--------------------------------------------------------------------------------+
| [listen](#listen)       | Change, add or stop rssh server port. Open the server port on a client (proxy) |
+--------------+--------------------------------------------------------------------------------+
| [log](#log)          | Collect log output from client                                                 |
+--------------+--------------------------------------------------------------------------------+
| ls           | List connected controllable hosts.                                             |
+--------------+--------------------------------------------------------------------------------+
| map          | Display a topology map of connected clients and their pivot chains             |
+--------------+--------------------------------------------------------------------------------+
| priv         | Privilege shows the current user privilege level.                              |
+--------------+--------------------------------------------------------------------------------+
| version      | Give server build version                                                      |
+--------------+--------------------------------------------------------------------------------+
| [watch](#watch)        | Watches controllable client connections                                        |
+--------------+--------------------------------------------------------------------------------+
| [webhook](#webhook)      | Add or remove webhooks                                                         |
+--------------+--------------------------------------------------------------------------------+
| who          | List users connected to the RSSH server                                        |
+--------------+--------------------------------------------------------------------------------+


## access

Temporarily changes the ownership of an RSSH client connection, only lasts until restart of rssh server.

```sh
access [OPTIONS] -p <FILTER>
Filter uses glob matching against all attributes of a target (id, public key hash, hostname, ip)
        --all   Set the ownership to anyone on the server
        --current       Set the ownership to only the current user
        --owners        Set the ownership of the client, comma seperated user list
        --pattern       Clients to act on
        -a      Set the ownership to anyone on the server
        -c      Set the ownership to only the current user
        -o      Set the ownership of the client, comma seperated user list
        -p      Clients to act on
        -y      Auto confirm prompt
```

### Details

The RSSH server supports very basic user privileges, where users found in the `data-directory`/`keys` (specified by `--datadir`) folder e.g `data-directory/keys/jim` will be assigned as a "user" only able to see clients that are public (found in the authorized_controllee_keys file without an `owners` tag, or an empty `owners` tag) or specifically assigned to them, e.g `owners="jim"`. 

This can be changed at run time via an user sharing access to a client they own with the `access` command, or a server administrator.   

Defaultly, any public key found in the `authorized_keys` file will be marked as an administrator to retain backwards compatibility.

### Authorized Keys Ownership Format

```sh
owner="username" ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIBE1xzpHj1ohkDnQYF3UTzjgzGJ4uncbNg+nrfzuHB/r 
```

## autocomplete

Generate a `bashrc` or `zshrc` compatiable RSSH completions. So you can use tab completion on your RSSH client IDs even when not using the server console.
```sh
autocomplete
        --clients       Return a list of client ids
        --shell-completion      Generate bash completion to put in .bashrc/.zshrc with optional server name (will use rssh as server name if not set)
```

### Installation

Use `>>` to append them to your `bashrc` or `zshrc`. 
You will need to add the server address as the `--shel-completion` argument.

```sh
ssh your.rssh.server.internal -p 3232 autocomplete --shell-completion your.rssh.server.internal:3232 >> .bashrc
```

### Usage

```sh
# Will give you an option based on what clients are connected
ssh -J your.rssh.server.internal:3232 <TAB>
```

### Example of completions

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


## connect

Start shell on remote controllable host.

```sh
    connect [--shell ARG] <remote_id>
        --shell Set the shell (or program) to start on connection, this also can take an http, https or rssh url that is then downloaded to disk and executed
```

### Example
```sh
# Use executable on disk
connect --shell /bin/some/binary fe6c52029e37185e4c7d512edd67a6c7694e2995

# Downloads executable file
connect --shell https://your.site/binary fe6c52029e37185e4c7d512edd67a6c7694e2995

connect --shell http://your.site/binary fe6c52029e37185e4c7d512edd67a6c7694e2995

# You must place binary in the `downloads` folder of the RSSH server
connect --shell rssh://binary fe6c52029e37185e4c7d512edd67a6c7694e2995

# These can also also be done via the jumphost syntax
#  
# e.g
# ssh -J your.rssh.server:3232 <rssh_client_id> https://your.host/program
#
```

### Supported shell Schemes

`http/https`: Pure web downloading

`rssh`: Download via the rssh server

The rssh server will serve content over SSH from the `downloads` directory in the executables working directory.

Both of these methods will opportunistically use [memfd](https://man7.org/linux/man-pages/man2/memfd_create.2.html) on linux which does not write to disk.

## exec

Execute a command against an RSSH client. This can be used to run commands against multiple RSSH clients all at the same time. 

```sh
Filter uses glob matching against all attributes of a target (hostname, ip, id), allowing you to run a command against multiple machines
        --raw   Do not label output blocks with the client they came from
        -q      Quiet, no output (will also remove confirmation prompt)
        -y      No confirmation prompt
```

### Examples

```sh
# Match all clients
exec -y * ls -l 
```

## kill
Stop the execution of a remote RSSH client.

```sh
kill <remote_id>
kill <glob pattern>
Stop the execute of the rssh client.
        -y      Do not prompt for confirmation before killing clients
```

## link
Link is by far one of the most complex commands that the RSSH server provides. 
It gives users the ability to generate new RSSH clients and serve them on an HTTP, HTTPS socket.

It supports cross compilation, and the use of UPX (to make binaries smaller) and garble (to make binaries harder for scrape for build details).

The docker container is recommended when using this command as its mildly annoying to get the various tools in the path correctly.


### Arguments 

```sh
--fingerprint   Set RSSH server fingerprint will default to server public key
--garble        Use garble to obfuscate the binary (requires garble to be installed)
--goarch        Set the target build architecture (default runtime GOARCH)
--goarm Set the go arm variable (not set by default)
--goos  Set the target build operating system (default runtime GOOS)
--http  Use http polling as the underlying transport
--https Use https polling as the underlying transport
--log-level     Set default output logging levels, [INFO,WARNING,ERROR,FATAL,DISABLED]
--lzma  Use lzma compression for smaller binary at the cost of overhead at execution (requires upx flag to be set)
--name  Set the link download url/filename (default random characters)
--no-lib-c      Compile client without glibc
--ntlm-proxy-creds      Set NTLM proxy credentials in format DOMAIN\USER:PASS
--owners        Set owners of client, if unset client is public all users. E.g --owners jsmith,ldavidson
--proxy Set connect proxy address to bake it
--raw-download  Download over raw TCP, outputs bash downloader rather than http
--shared-object Generate shared object file
--single-session        Only allow one RSSH client with this public key to connect at a time
--sni   When TLS is in use, set a custom SNI for the client to connect with
--stdio Use stdin and stdout as transport, will disable logging, destination after stdio:// is ignored
--tls   Use TLS as the underlying transport
--upx   Use upx to compress the final binary (requires upx to be installed)
--use-host-header       Use HTTP Host header as callback address when generating download template (add .sh to your download urls and find out)
--use-kerberos  Instruct client to try and use kerberos ticket when using a proxy
--version-string        Set the SSH version string the client uses, will always be prefixed with SSH-
--working-directory     Set download/working directory for automatic script (i.e doing curl https://<url>.sh)
--ws    Use plain http websockets as the underlying transport
--wss   Use TLS websockets as the underlying transport
-C      Comment to add as the public key (acts as the name)
-l      List currently active download links
-o      Set owners of client, if unset client is public all users. E.g --owners jsmith,ldavidson
-r      Remove download link
-s      Set homeserver address, defaults to server --external_address if set, or server listen address if not
```

### Examples

```sh
# Served under the test name
# E.g http://rssh.server/test
link --goos linux --goarch arm64 --name test


# Build a binary for windows which requires the mingw cross compiler
link --goos windows --name test2

# Build a shared library
link --shared-object
```


## listen

Starts or stops listening server ports.

This command allows you to change the servers listening port, or forward the servers control port onto an rssh client.

What this means is you can effectively forward the server port that accepts RSSH clients onto a currently running RSSH client.

This is a convenience method so you dont have to write SSH syntax.

```sh
--auto  Automatically turn on server control port on clients that match criteria, (use --off --auto to disable and --l --auto to view)
--client        Open server port on client/s takes a pattern, e.g -c *, --client your.hostname.here
--off   Turn off port, e.g --off :8080 127.0.0.1:4444
--on    Turn on port, e.g --on :8080 127.0.0.1:4444
--server        Change the server listeners
-c      Open server port on client/s takes a pattern, e.g -c *, --client your.hostname.here
-l      List all enabled addresses
-s      Change the server listeners
```

### Example

```sh

# Open a server control port on the a0... client listening on all interfaces on port 2222 
listen --client a0baa1631fe7cfbbfae34eb7a66d46c00d2a161e --on :2222
# Select a specific interface
listen --client a0baa1631fe7cfbbfae34eb7a66d46c00d2a161e --on 192.16.0.23:2223

# Open a new port on the server 4444 on all interfaces
listen --server --on :4444
```


## log
This server command enables the collect RSSH log output from clients. You can also enable logging to a file, or enable console output. 

```sh
--log-level     Set client log level, default for generated clients is currently: "INFO"
--to-console    directs output to the server console (or current connection), stop with any keypress
--to-file       direct output to file, takes a path as an argument
-c      client to collect logging from
```

### Examples
```sh
# Direct log lines from the RSSH client to the server console 
log -c a0baa1631fe7cfbbfae34eb7a66d46c00d2a161e --log-level INFO --to-console

# Log out to the file in temp
log -c a0baa1631fe7cfbbfae34eb7a66d46c00d2a161e --log-level INFO --to-file /tmp/log_rssh.txt
```

## watch
Streams connection state events of clients.

```sh
--json  Print out watch log as json
-a      Lists all previous connection events
-l      List previous n number of connection events, e.g watch -l 10 shows last 10 connections
```

### Example output

After running `watch` you will get lines that look like this, which denote clients joining and leaving.
```sh
catcher$ watch

Watching clients...
2026/11/03 15:49:23 <- nhas.computer.local (127.0.0.1:65142 fa7c0e53f1644e52a02f880130d9329eefb4b14f) SSH-v2.8.1-23-g35f2725-darwin_arm64 connected
2026/11/03 15:49:32 -> nhas.computer.local (127.0.0.1:65142 fa7c0e53f1644e52a02f880130d9329eefb4b14f) SSH-v2.8.1-23-g35f2725-darwin_arm64 disconnected
2026/11/03 15:49:33 <- nhas.computer.local (127.0.0.1:65145 5352792f15a24481a3890b79c4c555ae91a20f97) SSH-v2.8.1-23-g35f2725-darwin_arm64 connected
2026/11/03 15:49:36 -> nhas.computer.local (127.0.0.1:65145 5352792f15a24481a3890b79c4c555ae91a20f97) SSH-v2.8.1-23-g35f2725-darwin_arm64 disconnected
```

You can also specify `json` which will emit the lines as json objects in the following format:
```json
Watching clients...
{"Status":"connected","ID":"abc92a9a661f3e5a4adba95e286ade52493f7227","IP":"127.0.0.1:65172","HostName":"nhas.computer.local","Version":"SSH-v2.8.1-23-g35f2725-darwin_arm64","Timestamp":"2026-11-03T15:55:16.063253+13:00"}
```

This also does not emit colors.


## webhook
Set the RSSH server to send an HTTP webhook which shows the joining and leaving of RSSH clients.

```sh
--insecure      Disable TLS certificate checking
--off   Turns off existing webhook url
--on    Turns on webhook/s, must supply output as url
-l      Lists active webhooks
```


### Examples

```bash
catcher$ webhook --on http://localhost:8080/test_url
```

Then disconnect, or connect a client, this will when issue a `POST` request with the following format.

```bash
$ nc -l -p 8080
POST /test_url HTTP/1.1
Host: localhost:8080
User-Agent: Go-http-client/1.1
Content-Length: 165
Content-Type: application/json
Accept-Encoding: gzip

{"Status":"connected","ID":"ae92b6535a30566cbae122ebb2a5e754dd58f0ca","IP":"[::1]:52608","HostName":"user.computer","Timestamp":"2022-06-12T12:23:40.626775318+12:00"}
```


As an additional note, please use the `/slack` endpoint if connecting this to discord.