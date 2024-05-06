# Merkle Proof Service

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/Galactica-corp/merkle-proof-service)](https://github.com/Galactica-corp/merkle-proof-service/releases)

The Merkle Proof Service is an advanced utility engineered to facilitate the indexing of Merkle trees derived from Ethereum Solidity contracts. It further provides an API, enabling the retrieval of Merkle proofs, inclusive of root and path details. The service is equipped with two distinct API interfaces: gRPC and gRPC-gateway, the latter being an implementation of OpenAPI v2.


## Building the Executable

The process of building the executable for the Merkle Proof Service involves utilizing the Makefile provided at the root of the project. The procedure is as follows:

1. Clone the repository to your local machine and navigate to the root directory of the project:
    ```bash
    git clone https://github.com/Galactica-corp/merkle-proof-service.git
    cd merkle-proof-service
    ```

2. Execute the following command to build the binary:
    ```bash
    make build
    ```
   This operation will result in the creation of a binary named `galacticad-merkle` within the `build` directory.

3. Alternatively, the binary can be installed by executing the following command:
    ```bash
    make install
    ```

The `galacticad-merkle` binary can build with a variety of database backends by passing the `DB_BACKEND` variable to the `make build` command. For instance, to build `galacticad-merkle` with the `pebbledb` backend, the following command can be executed:

```bash
make build DB_BACKEND=pebbledb
```

The database backends available for selection include: `goleveldb`, `cleveldb`, `memdb`, `boltdb`, `rocksdb`, `badgerdb`, and `pebbledb`.

Each database backend possesses unique performance characteristics. For instance, `pebbledb` is recommended for production use due to its superior performance in reading data to generate the Merkle proof.

For testing purposes, the `memdb` backend is recommended. This backend stores data in memory, resulting in the fastest data write and read speeds.

## Running the Service

To run the service, you can use the built binary. Here's an example of how to start the indexer with a specified EVM RPC endpoint:

```bash
./build/galacticad-merkle indexer start --evm-rpc ws://localhost:8546
```

Here are the available flags for the `start` command:

- `--evm-rpc string`: EVM RPC endpoint (default "ws://localhost:8546")
- `--grpc-gateway.address string`: gRPC gateway server address (default "localhost:8480")
- `--grpc.address string`: gRPC server address (default "localhost:50651")

And the global flags:

- `--home string`: home directory to store data (default is $HOME/.galacticad-merkle)
- `--config string`: config file (default is $HOME/.galacticad-merkle/merkle.yaml)
- `--db-backend string`: database backend, available options: `goleveldb` (default), `cleveldb`, `memdb`, `boltdb`, `rocksdb`, `badgerdb`, `pebbledb`
- `--log-level string`: log level, available options: debug, info, error, none. Default is info


## API to Retrieve Merkle Proofs

The Merkle Proof Service provides two distinct API interfaces: `gRPC` and `gRPC-gateway`. The `gRPC-gateway` interface is an implementation of `OpenAPI v2`, which allows for the generation of client libraries in various programming languages.

### gRPC-gateway

By default, the `gRPC-gateway` interface is available at `http://localhost:8480`. The OpenAPI v2 specification can be accessed at `http://localhost:8480/swagger.json` and documentation can be viewed at `http://localhost:8480/docs`.

You can change the address of the `gRPC-gateway` address by several ways:

- Pass the `--grpc-gateway.address` flag to the `start` command
- Set the `GRPC_GATEWAY_ADDRESS` environment variable
- Change the address in the configuration YAML file:
   ```yaml
   grpc_gateway:
     address: localhost:8480
   ```

The service provides the following endpoints:

- `GET /v1/galactica/merkle/proof/{registry}/{leaf}`: Retrieves the Merkle proof for a given contract address and leaf index.
- `GET /v1/galactica/merkle/empty_index/{registry}`: Retrieves the random empty index for a given contract address.

### gRPC

The `gRPC` interface is available at `http://localhost:50651`. You cain find the gRPC query server definition in the following file: [proto/galactica/merkle/query.proto](proto/galactica/merkle/query.proto).

The service provides the following methods:

- `Proof`: Retrieves the Merkle proof for a given contract address and leaf index. 
- `GetEmptyIndex`: Retrieves the random empty index for a given contract address.

You can change the address of the `gRPC` server by several ways:

- Pass the `--grpc.address` flag to the `start` command
- Set the `GRPC_ADDRESS` environment variable
- Change the address in the configuration YAML file:
   ```yaml
   grpc:
     address: localhost:50651
   ```

## Configuration

The Merkle Proof Service can be configured using a YAML file. By default, the configuration file is located at `$HOME/.galacticad-merkle/merkle.yaml`. The configuration file can be customized to specify the database backend, log level, and other parameters.

Here is an example of the configuration file:

```yaml
log_level: info
evm_rpc: wss://evm-rpc-ws-reticulum.galactica.com
db_backend: pebbledb
grpc:
  address: localhost:50651
grpc_gateway:
  address: localhost:8480
jobs:
  - address: 0x<contract_address>
    contract: ZkCertificateRegistry
    start_block: <start_block>
```

### Ethereum contract configuration

The `jobs` section of the configuration file is used to specify the Ethereum contract addresses and the corresponding Solidity contract names. The `start_block` parameter is used to specify the block number from which the indexer should start indexing the contract events. 

You can add multiple jobs to the configuration file to index multiple contracts. 

## License

The Merkle Proof Service is licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for the full license text.
