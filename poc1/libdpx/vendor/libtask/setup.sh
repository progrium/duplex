#!/bin/bash

OPTION="$1"

if test -z "$OPTION"; then
    OPTION="all"
fi

if [ "$OPTION" = "install" ]; then
    cp -uv *.a $2
else
    make $OPTION
fi
