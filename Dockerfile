# Alpine doesn't have glibc
#FROM alpine:3.20.3

# Is there a more light OS?
# ubuntu:14.04
# gcc 4.8.4
# image 14.04.1 were using manifest Schema v1 (new dockers does not accept it)

# gcc 5.4.0
#FROM ubuntu:16.04

# Too new gcc version?
# gcc (Debian 14.2.0-8) 14.2.0
#FROM debian:trixie-20241111-slim

# gcc (Debian 8.3.0-6) 8.3.0
#FROM debian:buster-20231009

# gcc 10.2.1
FROM debian:11.11-slim AS builder

# Install gcc, g++, valgrind, and gdb
# valgrind needs libc6-dbg and bash (not sh/ash from alpine)
# autotools-dev containes aclocal, required by autogen.sh inside valgrind
# automake is required by autogen.sh
RUN apt-get update && apt-get install -y \
    gcc \
    g++ \
    make \
    libc6-dbg \
    python3 \
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
    make clean && ./autogen.sh && ./configure --prefix=/tmp/parser/valgrind && make && make install

FROM debian:11.11-slim

RUN apt-get update && apt-get install -y \
    gcc \
    g++ \
    libc6-dbg \
    python3 \
    bash \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /tmp/parser/valgrind /tmp/parser/valgrind

COPY ./parser/vg_to_opt_trace.py /tmp/parser
COPY ./parser/wsgi_backend.py /tmp/parser

RUN mkdir "/tmp/user_code"

# Set the working directory
WORKDIR /tmp/user_code

# Default command
CMD ["/bin/sh"]

# TODO: remove the build files after to reduce image size
