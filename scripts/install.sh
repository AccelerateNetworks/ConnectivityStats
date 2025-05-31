#!/bin/bash

fail() {
    echo ""
    echo "Install failed."
    exit $1
    quit $1
    return $0
}

get-services() {
    sudo systemctl list-units | grep -E -o 'connectivitystats@[a-z0-9]+\.service'
}

cd "$(dirname $0)"

if [[ "$(dirname $0)" == */scripts ]]; then
    cd ..
fi

sudo true

if [[ $? -ne "0" ]]; then
    echo "Please run this script with sudo permissions."
    exit 1
fi

mapfile -t RESTARTSERVICES < <(get-services)

if [[ -e "/usr/sbin/ConnectivityStats" ]]; then
    echo "Previous install found, removing."
    echo ""
    ./scripts/uninstall.sh skip-services
    echo ""
fi

mapfile -t SERVICES < <(get-services)

for SERVICE in ${SERVICES[@]}; do
    echo $SERVICE
    sudo systemctl stop $SERVICE
done

echo "Building..."

./scripts/build.sh
EXITCODE=$?
if [[ "$EXITCODE" -ne "0" ]]; then
    fail $EXITCODE
fi

mapfile -t SERVICES < <(get-services)

for SERVICE in ${SERVICES[@]}; do
    echo "Waiting for $SERVICE to stop..."
done

if (( ${#SERVICES[@]} )); then
    echo ""
fi

STILL_RUNNING=1
while $([ "$STILL_RUNNING" -ge "0" ] && [ "$STILL_RUNNING" -lt "10" ]); do
    mapfile -t SERVICES < <(get-services)
    if (( ${#SERVICES[@]} )); then
        sleep 1
        ((STILL_RUNNING++))
    else
        STILL_RUNNING=-1
    fi
done

mapfile -t SERVICES < <(get-services)

for SERVICE in ${SERVICES[@]}; do
    sudo systemctl kill $SERVICE
    echo "Killing $SERVICE..."
done

if (( ${#SERVICES[@]} )); then
    echo ""
fi


echo ""
echo "Installing..."

sudo cp -f build/ConnectivityStats /usr/sbin
EXITCODE=$?
if [[ "$EXITCODE" -ne "0" ]]; then
    fail $EXITCODE
fi

# if [[ -z $DISPLAY ]]; then
#     DISPLAYVAR=""
#     sudo cp -f ./connectivitystats@.service /usr/lib/systemd/system/connectivitystats@.service

# else
#     DISPLAYVAR="Environment=\"DISPLAY=$DISPLAY\""
#     cat ./connectivitystats@.service | sed "s/%DISPLAYVAR%/$DISPLAYVAR/g" > /tmp/connectivitystats@.service
#     sudo mv -f /tmp/connectivitystats@.service /usr/lib/systemd/system/connectivitystats@.service
# fi

sudo cp -f ./connectivitystats@.service /usr/lib/systemd/system/connectivitystats@.service


EXITCODE=$?
if [[ "$EXITCODE" -ne "0" ]]; then
    fail $EXITCODE
fi

sudo systemctl daemon-reload
sudo systemctl reset-failed

LOGDIR="/var/log/connectivitystats" 

if [[ ! -d "$LOGDIR" ]]; then
    sudo mkdir $LOGDIR
    echo "Created directory: $LOGDIR"
fi

# Network garbage. See: https://github.com/prometheus-community/pro-bing?tab=readme-ov-file#supported-operating-systems
sudo sysctl -w net.ipv4.ping_group_range="0 2147483647" > /dev/null
sudo setcap cap_net_raw=+ep /usr/sbin/ConnectivityStats > /dev/null

if (( ${#RESTARTSERVICES[@]} > 0 )); then
    echo ""

    for SERVICE in ${RESTARTSERVICES[@]}; do
        sudo service $SERVICE restart
        echo "Restarted service $SERVICE"
    done

    echo ""
    echo "You have reinstalled ConnectivityStats!"
    echo ""
else
    echo ""
    echo "You have installed ConnectivityStats!"
    echo ""
    echo "To begin taking measurements, run:" 
    echo "$ sudo service connectivitystats@all start"
    echo ""
    echo "Or, to measure a spesific interface, run:" 
    echo "$ sudo service connectivitystats@<interface> start"
    echo ""
    echo "To finish your measurements, run:"
    echo "$ sudo service connectivitystats@* stop"
    echo ""
    echo "Recorded data can be found in: /var/log/connectivitystats/<interface>.csv"
    echo ""
    echo "Please be aware that your internet connection will temporarily be disconnected while running measurements."
    echo ""

fi

exit 0