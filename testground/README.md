# Guide

[Testground documentation](https://docs.testground.ai/)

## Getting started

### Prerequisites

- docker
- go 1.22, or higher

### Install Testground

```bash
$ git clone https://github.com/testground/testground.git
$ cd testground
# compile Testground and all related dependencies
$ make install
```

It'll install the `testground` binary in your `$GOPATH/bin` directory, and build several docker images.

### Running Testground

```bash
$ TESTGROUND_HOME=$PWD/data testground daemon
```

Keep the daemon process running during the test.

### Running Test Plan

Import the test plan before the first run:

```bash
$ TESTGROUND_HOME=$PWD/data testground plan import --from /path/to/cronos/testground/benchmark
```

Run the benchmark test plan in local docker environment:

```bash
$ testground run composition -f /path/to/cronos/testground/benchmark/compositions/local.toml --wait
```

### macOS

If you use `colima` as docker runtime on macOS, create the symbolic link `/var/run/docker.sock`:

```bash
$ sudo ln -s $HOME/.colima/docker.sock /var/run/docker.sock
```

And mount the related directories into the virtual machine:

```toml
mounts:
  - location: /var/folders
    writable: false
  - location: <TESTGROUND_HOME>
    writable: true
```

