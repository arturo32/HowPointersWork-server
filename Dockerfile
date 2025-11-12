# Alpine doesn't have glibc
#FROM alpine:3.20.3

# Is there a more light OS?
# ubuntu:14.04
# gcc 4.8.4
# image 14.04.1 were using manifest Schema v1 (new dockers does not accept it)

# gcc 5.4.0 (libc-dbg: 2.23-0ubuntu11.3). Works with some warning from valgrind
#FROM ubuntu:16.04

# gcc 10.2.1; lic6-dbg 2.36-9+dev12u13
#FROM debian:12.12-slim AS builder

# Is yet to be known how big can the libc-dbg version can be before valgrind don't work anymore.
# The libc-dbg:2.23-0ubuntu11.3. Works with small warnings. with version 2.31, valgrind says that there is
# a conditional jump with unitialized variables.


# gcc 6.3.0; libc6-dbg 2.24-11+deb9u4
FROM debian:9.13-slim AS build


# Install gcc, g++, valgrind, and gdb
# valgrind needs libc6-dbg and bash (not sh/ash from alpine)
# autotools-dev containes aclocal, required by autogen.sh inside valgrind
# automake is required by autogen.sh
RUN sed -i 's/deb.debian.org/archive.debian.org/g' /etc/apt/sources.list && \
    sed -i 's|security.debian.org|archive.debian.org|g' /etc/apt/sources.list && \
    sed -i '/stretch-updates/d' /etc/apt/sources.list && \
    apt-get update && apt-get install -y \
    gcc \
    g++ \
    make \
    libc6-dbg \
    autotools-dev \
    automake \
    bash \
    && rm -rf /var/lib/apt/lists/*

# Debug
#RUN gcc --version

COPY ./parser/valgrind-3.11.0 /tmp/parser/valgrind-3.11.0

# Builds custom version of valgrind (by Philip Guo)
RUN chmod +x /tmp/parser/valgrind-3.11.0/autogen.sh && \
    cd /tmp/parser/valgrind-3.11.0 && \
    make clean && ./autogen.sh && ./configure --prefix=`pwd`/inst  && make && make install && \
    cd /tmp/parser && mv /tmp/parser/valgrind-3.11.0/inst inst && \
    find /tmp/parser/valgrind-3.11.0/ -mindepth 1 -maxdepth 1 -type d -exec rm -rf {} + && \
    mv inst /tmp/parser/valgrind-3.11.0/inst && cd /tmp/parser/valgrind-3.11.0 && \
    rm -f Makefile* README* conf* NEWS.old


FROM debian:9.13-slim

RUN sed -i 's/deb.debian.org/archive.debian.org/g' /etc/apt/sources.list && \
    sed -i 's|security.debian.org|archive.debian.org|g' /etc/apt/sources.list && \
    sed -i '/stretch-updates/d' /etc/apt/sources.list && \
    apt-get update && apt-get install -y \
    gcc \
    g++ \
    libc6-dbg \
    python3 \
    bash \
    && rm -rf /var/lib/apt/lists/*

COPY --from=build /tmp/parser/valgrind-3.11.0/ /tmp/parser/valgrind-3.11.0/

COPY ./parser/vg_to_opt_trace.py /tmp/parser
COPY ./parser/wsgi_backend.py /tmp/parser

RUN mkdir "/tmp/user_code"

# Set the working directory
WORKDIR /tmp/user_code

# Default command
CMD ["/bin/sh"]
