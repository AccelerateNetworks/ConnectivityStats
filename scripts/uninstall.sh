#!/bin/bash

sudo true

if [[ $? -ne 0 ]]; then
    echo "Please run this script with sudo permissions."
    exit 1
fi

get-services() {
    sudo systemctl list-units | grep -E -o 'connectivitystats@[a-z0-9]+\.service'
}


if [[ $1 -ne "skip-services" ]]; then
    mapfile SERVICES < <(get-services)

    for SERVICE in "${SERVICES[@]}"; do
        sudo service $SERVICE stop
        sudo systemctl stop $SERVICE
        sudo systemctl disable $SERVICE
        
        echo "Removed service $SERVICE"
    done

    if (( ${#SERVICES[@]} )); then
        echo ""
    fi
fi

declare -a REMOVE_FILES=(
    "/usr/lib/systemd/system/connectivitystats@.service"
    "/lib/systemd/system/connectivitystats@.service"
    "/etc/systemd/system/connectivitystats@.service"
    "/usr/sbin/ConnectivityStats"
)

for FILE in ${REMOVE_FILES[@]}; do
    sudo rm -f $FILE
    echo "Removed file $FILE"
done

sudo systemctl daemon-reload
sudo systemctl reset-failed

echo ""
echo "ConnectivityStats has been uninstalled."