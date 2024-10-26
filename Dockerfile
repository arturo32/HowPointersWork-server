# Start from the Alpine base image
FROM alpine:3.20.3

# Install gcc, g++, valgrind, and gdb
RUN apk add --no-cache \
    gcc \
    g++ \
    valgrind \
    gdb \
    musl-dev \
    build-base

# Set the working directory
WORKDIR /app

# Default command
CMD ["/bin/sh"]