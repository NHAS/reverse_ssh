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

## Todo

- ~~Make the selection UI a bit better~~
- ~~Handle IO errors better in all parties~~
- Remove remote connection host from list when a disconnection is registered
- Make the client cleanly fork itself from whatever parent it started from, giving it persistence (might be hard in go)
- ~~direct-tcp port forwarding to select hosts~~
