# Berachain beacond monitoring

## Why this tool?

Berachain mainnet binary has a new BLS signature and most of the previous monitoring tools are not working anymore. Here's our tool to monitor beacond with this new BLS signature.

## Usage

```
pip install -r requirements.txt
python beacond-monitor.py --addresses=6073D.....,,C11... --rpc=http://node1:26657,http://node2:26657
```

`addresses`: CometBFT address
`rpc`: RPC node (you can specify multiple nodes)

## Docker

`ghcr.io/stakelab-zone/berachain-beacond-monitoring:latest`
