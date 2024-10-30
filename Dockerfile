# Start from the Alpine base image
FROM alpine:3.20.3

RUN apk update

# Install gcc, g++, valgrind, and gdb
RUN apk add --no-cache \
    gcc \
    g++ \
    valgrind \
    musl-dev \
    build-base \
    make \
    linux-headers \
    gdb

#RUN mkdir gdb-build; \
#    cd gdb-build; \
#    wget http://ftp.gnu.org/gnu/gdb/gdb-10.2.tar.xz; \
#    tar -xvf gdb-10.2.tar.xz; \
#    cd gdb-10.2; \
#    ./configure; \
#    make; \
#    sudo make install;

# Set the working directory
WORKDIR /app

# Default command
CMD ["/bin/sh"]
