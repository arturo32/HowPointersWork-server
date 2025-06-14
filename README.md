# How Pointers Work - Server

Inspired by this [article on Tork engine](https://dev.to/acoh3n/lets-build-a-code-execution-engine-4kgi) and this [fork of Philip Guo's Python Tutor](https://github.com/meghaagr13/CTutor). 

<a href="https://github.com/arturo32/HowPointersWork">Click here to go to the frontend repository</a>.

And <a href="https://github.com/runabol/tork"> click here to go to the Tork repository</a>.

## Running

### With docker

This project is composed of two images: One that will run the Go program and other that will be run by Tork.

So we need to first build the image that Tork will use (the one that will compile and run the custom valgrind):

```bash
sudo docker build -f Dockerfile -t gcc-compiler .
```

Then, we build the main image:
```bash
sudo docker build -f Dockerfile.main -t hpw-server .
```

As ["Docker in docker" is unadvised](https://jpetazzo.github.io/2015/09/03/do-not-use-docker-in-docker-for-ci/), we run the main container using the `-v` flag that will bind the most internal container image docker socket to the external docker. Ps.: `--network=host/--net=host` don't work on Windows.

```bash
sudo docker run -v /var/run/docker.sock:/var/run/docker.sock --network=host -it hpw-server
```

### Without docker

You'll need:

- [Go](https://golang.org/) version 1.19 or better installed.
- Docker

Build the gcc compiler image:

```bash
`docker build -t gcc-compiler .`
```


Start the server with docker:

First, build the image:
```bash
sudo docker build -f Dockerfile.main -t hpw-server .
```

Start the server:

```bash
go run main.go run standalone
```

Execute a code snippet. Example

```bash
curl \
  -s \
  -X POST \
  -H "content-type:application/json" \
  -d '{"language":"c","code":"#include <stdio.h>\n\nint main(){\nint i = 23;\nint *k = &i;\nreturn 0;\n}"}' \
  http://localhost:8000/execute
```

You can try changing the `language` to `c++`.


### How to update Tork in the future
```bash
go list -m -u github.com/runabol/tork #find last version
go get github.com/runabol/tork@v0.1.121 #get last version
go mod tidy #syncronize dependencies
```
