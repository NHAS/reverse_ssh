#!/bin/bash
set -e

if [ ! -d "/data" ]; then
    echo "Please mount /data"
    exit 1
fi

if [ -z "$EXTERNAL_ADDRESS" ]; then
    echo "Please specify EXTERNAL_ADDRESS"
    exit 1
fi

touch /data/authorized_keys /data/authorized_controllee_keys

# Allow user to seed the authorized_keys file
if [ ! -z "$SEED_AUTHORIZED_KEYS" ]; then
    if [ -s /data/authorized_keys ]; then
        echo "authorized_keys is not empty, ignoring SEED_AUTHORIZED_KEYS\n"
    else
        echo "Seeding authorized_keys...\n"
        echo $SEED_AUTHORIZED_KEYS > /data/authorized_keys
    fi
fi

cd /app/bin
exec ./server --datadir /data --enable-client-downloads --tls --external_address $EXTERNAL_ADDRESS :2222