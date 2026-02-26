# Kubernetes Deployment

## How It Works

Each node gets a unique integer ID via `JOB_COMPLETION_INDEX` ([Indexed Jobs](https://kubernetes.io/docs/tasks/job/indexed-parallel-processing-static/)) or from its hostname (e.g., `testplan-2` -> index `2`). Nodes discover peers through `testplan-{index}` DNS names provided by a headless Service.

Test data (genesis, accounts, transactions) must be baked into the image before deploying to k8s. The `run` command inside each container reads `/data/config.json` to know how many validators/fullnodes exist, then starts the node and sends transactions.

## Step 1: Configure benchmark options

Edit `testground/benchmark-options.json` (see [README.md](README.md#configure-benchmark) for all fields):

```json
{
  "outdir": "/data",
  "validators": 3,
  "fullnodes": 0,
  "num_accounts": 10000,
  "num_txs": 5,
  "batch_size": 100,
  "tx_type": "simple-transfer",
  "validator_generate_load": true,
  "num_idle": 20,
  "config_patch": { "mempool": { "size": 100000 } },
  "app_patch": { "evm": { "block-stm-workers": 8 } },
  "genesis_patch": {},
  "node_overrides": {}
}
```

The `validators` + `fullnodes` count must match the Job `completions` / StatefulSet `replicas`.

## Step 2: Build image with embedded data

```bash
cd testground
docker build -t ghcr.io/<org>/cronos-testground:latest -f Dockerfile .. --build-arg EMBED_DATA=true
docker push ghcr.io/<org>/cronos-testground:latest
```

Or patch an existing image:

```bash
# Generate data from options
docker run --rm -v /tmp/data:/data cronos-testground:latest \
  stateless-testcase generic-gen "$(jq '.outdir = "/data/out"' benchmark-options.json)"

# Rebuild with data
echo 'FROM cronos-testground:latest
ADD ./out /data' | docker build -t ghcr.io/<org>/cronos-testground:latest -f - /tmp/data

docker push ghcr.io/<org>/cronos-testground:latest
```

## Step 3: Deploy to Kubernetes

### Option A: Indexed Job + Headless Service

Best for one-shot benchmark runs. The Job runs to completion and pods exit automatically.

```yaml
# cronos-benchmark.yaml
apiVersion: v1
kind: Service
metadata:
  name: testplan
spec:
  clusterIP: None
  selector:
    job-name: cronos-bench
  ports:
    - name: p2p
      port: 26656
    - name: echo
      port: 26659
---
apiVersion: batch/v1
kind: Job
metadata:
  name: cronos-bench
spec:
  completions: 3          # must match validators + fullnodes in options
  parallelism: 3
  completionMode: Indexed
  template:
    metadata:
      labels:
        job-name: cronos-bench
    spec:
      subdomain: testplan
      restartPolicy: Never
      containers:
        - name: node
          image: ghcr.io/<org>/cronos-testground:latest
          command: ["stateless-testcase", "run"]
          ports:
            - containerPort: 26656
            - containerPort: 26659
          volumeMounts:
            - name: outputs
              mountPath: /outputs
      volumes:
        - name: outputs
          emptyDir: {}
```

```bash
# Deploy
kubectl apply -f cronos-benchmark.yaml

# Watch logs
kubectl logs -f -l job-name=cronos-bench --all-containers

# Re-run (delete old job first)
kubectl delete job cronos-bench
kubectl apply -f cronos-benchmark.yaml
```

### Option B: StatefulSet

Best for repeated runs or when you need stable pod identity. Pods get hostnames `testplan-0`, `testplan-1`, etc.

```yaml
apiVersion: v1
kind: Service
metadata:
  name: testplan
spec:
  clusterIP: None
  selector:
    app: cronos-bench
  ports:
    - name: p2p
      port: 26656
    - name: echo
      port: 26659
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: testplan
spec:
  serviceName: testplan
  replicas: 3             # must match validators + fullnodes in options
  selector:
    matchLabels:
      app: cronos-bench
  template:
    metadata:
      labels:
        app: cronos-bench
    spec:
      containers:
        - name: node
          image: ghcr.io/<org>/cronos-testground:latest
          command: ["stateless-testcase", "run"]
          ports:
            - containerPort: 26656
            - containerPort: 26659
          volumeMounts:
            - name: outputs
              mountPath: /outputs
      volumes:
        - name: outputs
          emptyDir: {}
```

## Step 4: Collect results

Each node writes `{group}_{index}.tar.bz2` (e.g., `validators_0.tar.bz2`) to `/outputs` and a raw archive to `/data.tar.bz2`.

```bash
# Copy from a specific pod
kubectl cp cronos-bench-0:/outputs/validators_0.tar.bz2 ./validators_0.tar.bz2

# Or copy all outputs from all pods
for i in 0 1 2; do
  kubectl cp cronos-bench-${i}:/outputs/ ./results/node-${i}/
done
```

For persistent collection, replace `emptyDir` with a PVC or NFS volume:

```yaml
volumes:
  - name: outputs
    persistentVolumeClaim:
      claimName: benchmark-outputs
```

## Multi-Region Deployment (Real Network Latency)

Deploy validators across geographic regions to benchmark under real-world network conditions.

### Approach A: Single cluster with nodes in multiple zones/regions

Most cloud providers support clusters with node pools in different zones or regions. This is the simplest setup because k8s networking and DNS work automatically.

#### GKE example

```bash
# Create cluster with node pools in different regions
gcloud container clusters create cronos-bench \
  --region us-central1 --num-nodes 0

gcloud container node-pools create pool-us \
  --cluster cronos-bench --region us-central1 \
  --node-locations us-central1-a --num-nodes 1

gcloud container node-pools create pool-eu \
  --cluster cronos-bench --region us-central1 \
  --node-locations europe-west1-b --num-nodes 1

gcloud container node-pools create pool-asia \
  --cluster cronos-bench --region us-central1 \
  --node-locations asia-east1-a --num-nodes 1
```

#### EKS example

```bash
eksctl create cluster --name cronos-bench --region us-east-1

# Add node groups in different AZs (cross-region requires Wavelength or Outposts)
eksctl create nodegroup --cluster cronos-bench --name ng-us-east-1a \
  --node-zones us-east-1a --nodes 1
eksctl create nodegroup --cluster cronos-bench --name ng-us-west-2a \
  --node-zones us-west-2a --nodes 1
```

#### Label nodes by region

```bash
kubectl label nodes <node-name> bench-region=us
kubectl label nodes <node-name> bench-region=eu
kubectl label nodes <node-name> bench-region=asia
```

#### Deploy with pod-to-region pinning

Use `node_overrides` in `benchmark-options.json` to give each validator different settings, and topology constraints to pin each pod to a specific region.

```yaml
# cronos-bench-multiregion.yaml
apiVersion: v1
kind: Service
metadata:
  name: testplan
spec:
  clusterIP: None
  selector:
    job-name: cronos-bench
  ports:
    - name: p2p
      port: 26656
    - name: echo
      port: 26659
---
apiVersion: batch/v1
kind: Job
metadata:
  name: cronos-bench
spec:
  completions: 3
  parallelism: 3
  completionMode: Indexed
  template:
    metadata:
      labels:
        job-name: cronos-bench
    spec:
      subdomain: testplan
      restartPolicy: Never
      # Spread pods across regions
      topologySpreadConstraints:
        - maxSkew: 1
          topologyKey: bench-region
          whenUnsatisfiable: DoNotSchedule
          labelSelector:
            matchLabels:
              job-name: cronos-bench
      containers:
        - name: node
          image: ghcr.io/<org>/cronos-testground:latest
          command: ["stateless-testcase", "run"]
          ports:
            - containerPort: 26656
            - containerPort: 26659
          volumeMounts:
            - name: outputs
              mountPath: /outputs
      volumes:
        - name: outputs
          emptyDir: {}
```

To pin specific validators to specific regions, use separate Job manifests or a StatefulSet with per-ordinal overrides:

```yaml
# Pin validator 0 to US, validator 1 to EU, validator 2 to Asia
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: testplan
spec:
  serviceName: testplan
  replicas: 3
  selector:
    matchLabels:
      app: cronos-bench
  template:
    metadata:
      labels:
        app: cronos-bench
    spec:
      topologySpreadConstraints:
        - maxSkew: 1
          topologyKey: bench-region
          whenUnsatisfiable: DoNotSchedule
          labelSelector:
            matchLabels:
              app: cronos-bench
      containers:
        - name: node
          image: ghcr.io/<org>/cronos-testground:latest
          command: ["stateless-testcase", "run"]
          ports:
            - containerPort: 26656
            - containerPort: 26659
```

### Approach B: Multiple clusters with Submariner

For true multi-cloud (e.g., AWS + GCP + Azure), use [Submariner](https://submariner.io/) to connect clusters and provide cross-cluster DNS.

```bash
# Install submariner broker on cluster-1
subctl deploy-broker --kubeconfig cluster1.kubeconfig

# Join each cluster
subctl join broker-info.subm --kubeconfig cluster1.kubeconfig --clusterid us
subctl join broker-info.subm --kubeconfig cluster2.kubeconfig --clusterid eu
subctl join broker-info.subm --kubeconfig cluster3.kubeconfig --clusterid asia

# Export the headless service for cross-cluster DNS
subctl export service testplan --namespace default
```

With Submariner, pods in all clusters can resolve `testplan-{index}.testplan.default.svc.clusterset.local`. Update the `hostname_template` at gen time to match:

```bash
stateless-testcase gen /data/out \
  --hostname-template "testplan-{index}.testplan.default.svc.clusterset.local" \
  --validators 3 ...
```

Then deploy a single pod per cluster using the appropriate `JOB_COMPLETION_INDEX` or hostname.

### Verifying latency between nodes

After deployment, check actual inter-node latency:

```bash
# From validator 0's pod
kubectl exec -it testplan-0 -- ping -c 5 testplan-1
kubectl exec -it testplan-0 -- ping -c 5 testplan-2

# Or measure round-trip with CometBFT's P2P
kubectl logs testplan-0 | grep "block.*timeout\|latency"
```

Compare `block_stats.log` results between single-region and multi-region runs to see the latency impact on TPS and block times.

### Example: 3 validators across US / EU / Asia

```text
benchmark-options.json:
  validators: 3          <- must match Job completions or StatefulSet replicas

Deployment:
  testplan-0  ->  us-central1-a    (US)
  testplan-1  ->  europe-west1-b   (EU)
  testplan-2  ->  asia-east1-a     (Asia)

Expected latency:
  US <-> EU:    ~80-120ms
  US <-> Asia:  ~150-200ms
  EU <-> Asia:  ~120-180ms
```

The benchmark will report `median_blocktime`, `slowest_blocktime`, and TPS. Compare these against a colocated run to measure the impact of geographic distribution on consensus performance.

## Changing node count

When changing `validators` or `fullnodes` in `benchmark-options.json`:

1. Rebuild and push the image (Step 2)
2. Update `completions`/`parallelism` (Job) or `replicas` (StatefulSet) to match
3. Redeploy
