#!/bin/bash

cd "$(dirname $0)/.."

if [ ! -d "build" ]; then
    mkdir build
fi

cd src
go install
go build -o ../build/ConnectivityStats .
BUILDEXIT=$?
cd ..

if [[ $BUILDEXIT == "0" ]]; then
    echo "Build success!"
else 
    echo "Build failed."
    exit $BUILDEXIT
fi
