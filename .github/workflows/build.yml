name: Build
on:
  pull_request:
    branches:
      - main

jobs:
  cleanup-runs:
    runs-on: ubuntu-latest
    steps:
      - uses: rokroskar/workflow-run-cleanup-action@master
        env:
          GITHUB_TOKEN: "${{ secrets.GITHUB_TOKEN }}"
    if: "!startsWith(github.ref, 'refs/tags/') && github.ref != 'refs/heads/main'"

  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2.3.4
      - uses: actions/setup-go@v2.1.3
        with:
          go-version: 1.17
      - uses: technote-space/get-diff-action@v5
        id: git_diff
        with:
          SUFFIX_FILTER: |
            .go
            .mod
            .sum
      - run: |
          make build
        if: "env.GIT_DIFF != ''"

  unittest:
    runs-on: ubuntu-latest
    timeout-minutes: 10
    steps:
      - uses: actions/checkout@v2.3.4
      - uses: actions/setup-go@v2.1.3
        with:
          go-version: 1.17
      - uses: technote-space/get-diff-action@v5
        id: git_diff
        with:
          SUFFIX_FILTER: |
            .go
            .mod
            .sum
      - name: test & coverage report creation
        run: make test
        if: "env.GIT_DIFF != ''"
      - name: filter out proto files
        run: |
          excludelist+=" $(find ./ -type f -name '*.pb.go')"
          for filename in ${excludelist}; do
            filename=$(echo $filename | sed 's/^./github.com\/crypto-org-chain\/cronos/g')
            echo "Excluding ${filename} from coverage report..."
            sed -i.bak "/$(echo $filename | sed 's/\//\\\//g')/d" coverage.txt
          done
        if: "env.GIT_DIFF != ''"
      - uses: codecov/codecov-action@v2.0.2
        with:
          token: ${{ secrets.CODECOV_TOKEN }}
          file: ./coverage.txt
          fail_ci_if_error: true
        if: "env.GIT_DIFF != ''"

  gomod2nix:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2.3.4
      - uses: cachix/install-nix-action@v15
      - name: gomod2nix
        run: nix run -f ./nix gomod2nix
      - name: check working directory is clean
        id: changes
        uses: UnicornGlobal/has-changes-action@v1.0.11
      - uses: actions/upload-artifact@v2
        if: steps.changes.outputs.changed == 1
        with:
          name: gomod2nix.toml
          path: ./gomod2nix.toml
      - if: steps.changes.outputs.changed == 1
        run: echo "Working directory is dirty" && exit 1

  contracts:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2.3.4
      - uses: cachix/install-nix-action@v15
      - uses: cachix/cachix-action@v10
        with:
          name: cronos
          extraPullNames: dapp
          signingKey: "${{ secrets.CACHIX_SIGNING_KEY }}"
      - name: test contracts
        run: make test-cronos-contracts
      - name: build contracts
        run: make gen-cronos-contracts
      - name: check working directory is clean
        id: changes
        uses: UnicornGlobal/has-changes-action@v1.0.11
      - uses: actions/upload-artifact@v2
        if: steps.changes.outputs.changed == 1
        with:
          name: contracts_out
          path: ./contracts/out
          if-no-files-found: ignore
      - if: steps.changes.outputs.changed == 1
        run: echo "Working directory is dirty" && exit 1
