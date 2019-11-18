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

Testing is performed against a Oracle instance.  To make the thisngs easier I'm using 
```
docker run -d -p 49162:1521 -e ORACLE_ALLOW_REMOTE=true epiclabs/docker-oracle-xe-11g
```

Also you can run the docker-compose.yml to start an instance with DB included
```
cd test
docker-compose build
docker-compose up
```

## Configuring
The server looks for a configuration file that is passed in via the --config argument. That file should be in YAML format and follows the specification laid down by the main Grafeas project. 

For example:
```
  oracle:
    # Database host
    host: "localhost:49162"
    # Database name
    dbname: "xe"
    # Database username
    user: "system"
    # Database password
    password: "oracle"
    paginationkey: "JGMZNCmpDjpN2Jz10wMcF9kXc1vM8QC1nuxHB2gjIgY="
```

The complete config.yaml will looks like:
```
grafeas:
  api:
    # Endpoint address
    address: "0.0.0.0:8080"
    # PKI configuration (optional)
    cafile: 
    keyfile: 
    certfile: 
    # CORS configuration (optional)
    cors_allowed_origins:
      # - "http://example.net"
  storage_type: oracle
  oracle:
    # Database host
    host: "localhost:49162"
    # Database name
    dbname: "xe"
    # Database username
    user: "system"
    # Database password
    password: "oracle"
    paginationkey: "JGMZNCmpDjpN2Jz10wMcF9kXc1vM8QC1nuxHB2gjIgY="
```

The configuration file is specified by way of the --config argument
```
--config /path/to/config.yaml

```


## Contributing

Pull requests welcome.

### Special thanks

Thanks to [Jhon-tipper](https://github.com/john-tipper/grafeas-dynamodb) and his awesome implementation of [DynamoDB](https://github.com/john-tipper/grafeas-dynamodb) and the [Grafeas Team](https://github.com/grafeas/grafeas)

## License

Grafeas-oracle is under the Apache 2.0 license. See the [LICENSE](LICENSE) file for details.
