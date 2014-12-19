#!/bin/bash

OPTION="$1"

if test -z "$OPTION"; then
    OPTION="all"
fi

if [ "$OPTION" = "clean" ]; then
    rm -rfv build
else
    if test ! -d build; then
        mkdir -v build
        cd build
        CFLAGS="-ggdb" ../configure
    else
        cd build
    fi

    if [ "$OPTION" = "install" ]; then
        # relink to make sure
        rm -v .libs/libuuid.a
        ar cru .libs/libuuid.a .libs/uuid.o .libs/uuid_md5.o .libs/uuid_sha1.o .libs/uuid_prng.o .libs/uuid_mac.o .libs/uuid_time.o .libs/uuid_ui64.o .libs/uuid_ui128.o .libs/uuid_str.o
        ranlib .libs/libuuid.a
        cp -uv .libs/libuuid.a $2
    else
        make $OPTION
    fi
fi

cd ..
