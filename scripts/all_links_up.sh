#!/bin/bash

sudo true

if [[ $? -ne 0 ]]; then
    echo "Please run this script with sudo permissions."
    exit 1
fi

get-interfaces() {
    ip -o link show | awk '{print substr($2, 1, length($2)-1)}'
}

mapfile -t INTERFACES < <(get-interfaces)

for IFACE in ${INTERFACES[@]}; do
    echo "ip link set $IFACE up"
    sudo ip link set $IFACE up
done
