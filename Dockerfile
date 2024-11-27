# Alpine doesn't have glibc
#FROM alpine:3.20.3

# Is there a more light OS?
# gcc 4.8.4
FROM ubuntu:14.04.1

# Too new gcc version?
# gcc (Debian 14.2.0-8) 14.2.0
#FROM debian:trixie-20241111-slim

# gcc (Debian 8.3.0-6) 8.3.0
#FROM debian:buster-20231009


RUN apt-get update

# Install gcc, g++, valgrind, and gdb
# valgrind needs libc6-dbg and bash (not sh/ash from alpine)
# autotools-dev containes aclocal, required by autogen.sh inside valgrind
# automake is required by autogen.sh
RUN apt-get install -y \
    gcc \
    g++ \
    make \
    libc6-dbg \
    python3 \
    autotools-dev \
    automake \
    bash

RUN gcc --version

COPY ./parser/valgrind-3.11.0 /tmp/parser/valgrind-3.11.0

# The line below was not necessary until I tried to build this image inside a Ubuntu with WSL
RUN chmod +x /tmp/parser/valgrind-3.11.0/autogen.sh

# Builds custom version of valgrind (by Philip Guo)
RUN cd /tmp/parser/valgrind-3.11.0 \
    make clean && ./autogen.sh && ./configure --prefix=`pwd`/inst && make && make install

COPY ./parser/vg_to_opt_trace.py /tmp/parser
COPY ./parser/wsgi_backend.py /tmp/parser

RUN mkdir "/tmp/user_code"

# Set the working directory
WORKDIR /tmp/user_code

# Default command
CMD ["/bin/sh"]

# TODO: remove the build files after to reduce image size
