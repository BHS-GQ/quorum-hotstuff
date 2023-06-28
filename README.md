# Basic HotStuff in GoQuorum

A fork of GoQuorum 22.7.4 containing an implementation of the [Basic HotStuff](https://arxiv.org/pdf/1803.05069.pdf) (BHS) consensus protocol.

## Usage


### Executable

To build the `geth` executable for BHS in GoQuorum, run the ff:

```bash
make geth
```

This should generate a binary executable `/build/bin/geth`.

### Docker Image

To use BHS in GQ as a Docker container, run the ff:

```bash
docker build . -t <docker-image-name>
```

Run toy, local GoQuorum networks using `samples/simple`. Update `samples/simple/config/goquorum/Dockerfile` to point to `<docker-image-name>`.

```Dockerfile
FROM --platform=linux/amd64 <docker-image-name>
```

Inside `samples/simple`, run the network using:

```bash
./run.sh
```

and destroy it using

```bash
./remove.sh
```

Thanks to Consensys' [Quorum quickstart](https://github.com/ConsenSys/quorum-dev-quickstart) for providing a great template.

### Emulated Network

For more complex use-cases, please refer to our [emnet]() (emulated network) repository.

## BHS Implementation

For more information about our BHS implementation, refer to the [documentation](consensus/README.md). 
