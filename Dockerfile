# gcc 12.2.0; libc6-dbg 2.36. Valgrind 3.27.1, built from a fork of Philip Guo's
# OPT-tracing patch (originally against valgrind 3.11.0 / gcc 4.8.4) ported forward
# to compile and run cleanly against a modern glibc/gcc/DWARF5 toolchain.
FROM debian:12-slim AS build


# Install gcc, g++, valgrind, and gdb
# valgrind needs libc6-dbg and bash (not sh/ash from alpine)
# autotools-dev containes aclocal, required by autogen.sh inside valgrind
# automake is required by autogen.sh
RUN apt-get update && apt-get install -y \
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

COPY ./parser/valgrind-3.27.1 /tmp/parser/valgrind-3.27.1

# Builds custom version of valgrind (by Philip Guo, ported to 3.27.1)
# Note: this is a pristine, never-built source tree (unlike the old vendored
# 3.11.0 tree), so there's no Makefile yet for `make clean` to act on.
RUN chmod +x /tmp/parser/valgrind-3.27.1/autogen.sh && \
    cd /tmp/parser/valgrind-3.27.1 && \
    ./autogen.sh && ./configure --prefix=`pwd`/inst  && make && make install && \
    cd /tmp/parser && mv /tmp/parser/valgrind-3.27.1/inst inst && \
    find /tmp/parser/valgrind-3.27.1/ -mindepth 1 -maxdepth 1 -type d -exec rm -rf {} + && \
    mv inst /tmp/parser/valgrind-3.27.1/inst && cd /tmp/parser/valgrind-3.27.1 && \
    rm -f Makefile* README* conf* NEWS.old


FROM debian:12-slim

RUN apt-get update && apt-get install -y \
    gcc \
    g++ \
    libc6-dbg \
    python3 \
    bash \
    && rm -rf /var/lib/apt/lists/*

COPY --from=build /tmp/parser/valgrind-3.27.1/ /tmp/parser/valgrind-3.27.1/

COPY ./parser/vg_to_opt_trace.py /tmp/parser
COPY ./parser/wsgi_backend.py /tmp/parser

RUN mkdir "/tmp/user_code"

# Set the working directory
WORKDIR /tmp/user_code

# Default command
CMD ["/bin/sh"]
