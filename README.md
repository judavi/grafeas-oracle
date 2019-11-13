# Grafeas - Oracle

[![Build Status](https://github.com/judavi/grafeas-oracle/workflows/GitHub%20Actions/badge.svg)](https://github.com/judavi/grafeas-oracle/actions)

This project provides a [Grafeas](https://github.com/grafeas/grafeas) implementation that supports using Oracle as a storage mechanism.

## Building

Build using the provided Makefile or via Docker.

```shell
# Either build via make
make build

# or docker
docker build --rm .
```

## Unit tests

Testing is performed against a Oracle instance.  The Makefile offers the ability to start and stop a locally installed Oracle instance running via Java.  This requires that port 8000 be free.


## Contributing

Pull requests welcome.

## License

Grafeas-oracle is under the Apache 2.0 license. See the [LICENSE](LICENSE) file for details.
