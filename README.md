# Basic HotStuff in GoQuorum

A fork of GoQuorum 22.7.4 containing an implementation of the [Basic HotStuff](https://arxiv.org/pdf/1803.05069.pdf) (BHS) consensus protocol. Heavily based on the BHS implementation found in PolyNetwork's [Zion](https://github.com/polynetwork/Zion) project.

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

Run a local GoQuorum network using `samples/simple`. Update the Dockerfile (`samples/simple/config/goquorum/Dockerfile`) to use `<docker-image-name>`.

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

A readily-available Docker image can be found [here](https://hub.docker.com/r/gvlim/quorumbhs).

### Emulated Network

For more complex use-cases, refer to our [emnet](https://github.com/BHS-GQ/emnet) (emulated network) repository.

## BHS Implementation

For more information about our BHS implementation, refer to the [documentation](consensus/README.md). 

## Credits

- PolyNetwork's [Zion](https://github.com/polynetwork/Zion) project: the basis of our BHS implementation in a Geth-like framework.
- [ConsenSys Quorum Dev Quickstart](https://github.com/ConsenSys/quorum-dev-quickstart): our basis for creating a Docker container network.
