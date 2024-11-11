# Start from the Alpine base image
#FROM alpine:3.20.3
FROM ubuntu:14.04.1

COPY ./parser /tmp/parser

RUN apt-get update

# Install gcc, g++, valgrind, and gdb
#valgrind needs libc6-dbg and bash (not sh/ash from alpine)
#autotools-dev containes aclocal, required by autogen.sh inside valgrind
#automake is required by autogen.sh
RUN apt-get install -y \
    gcc \
    g++ \
    make \
    libc6-dbg \
    python3 \
    autotools-dev \
    automake \
    bash

#    valgrind \

#    gdb \
#
RUN cd /tmp/parser/valgrind-3.11.0 \
    make clean && ./autogen.sh && make && make install

#RUN mkdir gdb-build; \
#    cd gdb-build; \
#    wget http://ftp.gnu.org/gnu/gdb/gdb-10.2.tar.xz; \
#    tar -xvf gdb-10.2.tar.xz; \
#    cd gdb-10.2; \
#    ./configure; \
#    make; \
#    sudo make install;

# Set the working directory
WORKDIR /tmp

# Default command
CMD ["/bin/sh"]
