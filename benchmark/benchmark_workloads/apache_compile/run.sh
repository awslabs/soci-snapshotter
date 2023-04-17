#!/usr/bin/env bash

make
FILE=/httpd
if [ ! -f "$FILE" ]; then
    echo "$FILE does not exist."
else
    echo "$FILE exists"
fi
