## lightDAG

![build status](https://img.shields.io/github/actions/workflow/status/asonnino/hotstuff/rust.yml?style=flat-square&logo=GitHub&logoColor=white&link=https%3A%2F%2Fgithub.com%2Fasonnino%2Fhotstuff%2Factions)
[![golang](https://img.shields.io/badge/golang-1.21.1-blue?style=flat-square&logo=golang)](https://www.rust-lang.org)
[![python](https://img.shields.io/badge/python-3.9-blue?style=flat-square&logo=python&logoColor=white)](https://www.python.org/downloads/release/python-390/)
[![license](https://img.shields.io/badge/license-Apache-blue.svg?style=flat-square)](LICENSE)

This repo provides a minimal implementation of the lightDAG consensus protocol. The codebase has been designed to be small, efficient, and easy to benchmark and modify. It has not been designed to run in production but uses real cryptography (kyber), networking(native), and storage (nutsdb).

Say something about lightDAG...

## Quick Start

lightDAG is written in Golang, but all benchmarking scripts are written in Python and run with Fabric. To deploy and benchmark a testbed of 4 nodes on your local machine, clone the repo and install the python dependencies:

```shell
git clone https://github.com/ac-dcz/lightDAG
cd lightDAG/benchmark
pip install -r requirements.txt
```

You also need to install tmux (which runs all nodes and clients in the background).
Finally, run a local benchmark using fabric:

```shell
fab local
```

This command may take a long time the first time you run it (compiling golang code in release mode may be slow) and you can customize a number of benchmark parameters in fabfile.py. When the benchmark terminates, it displays a summary of the execution similarly to the one below.

```

-----------------------------------------
 SUMMARY:
-----------------------------------------
 + CONFIG:
 Protocol: lightDAG 
 DDOS attack: False 
 Committee size: 4 nodes
 Input rate: 3,000 tx/s
 Transaction size: 250 B
 Batch size: 800 tx/Batch Faults: 0 nodes
 Execution time: 10 s

 + RESULTS:
 Consensus TPS: 10,225 tx/s
 Consensus latency: 196 ms

 End-to-end TPS: 10,169 tx/s
 End-to-end latency: 230 ms
-----------------------------------------

```

## Next Steps
The wiki documents the codebase, explains its architecture and how to read benchmarks' results, and provides a step-by-step tutorial to run benchmarks on Alibaba cloud across multiple data centers (WAN).