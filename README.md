# Project Emmy

Project Emmy is a new actor runtime for Dapr developed internally at Microsoft.

This repository currently contains a fork of [Dapr](https://github.com/dapr/dapr) with the code for Project Emmy enabled. Eventually, we plan on replacing the fork with a set of patches.

## How to run Project Emmy

In Kubernetes, Project Emmy can be installed using Helm.

First, create a Kubernetes secret where you store the connection string for Postgres.

```sh
# These values are valid for the Postgres deployed by the Dapr tests
echo -n "connectionString=host=dapr-postgres-postgresql.dapr-tests.svc.cluster.local user=postgres password=example port=5432 connect_timeout=10 database=dapr_test" > postgres
kubectl create secret generic postgres-actors -n dapr-tests --from-file=postgres
```

You can then install Project Emmy using Helm. Make sure to set `globals.actors.serviceName=emmy` and `globals.reminders.serviceName=emmy`:

```sh
# Set version 0.0.0 to use the "edge" builds
VERSION=0.0.0
NAMESPACE=dapr-tests
kubectl create namespace $NAMESPACE || true
helm upgrade \
  --install \
  dapr \
  --namespace=dapr-tests \
  --wait --timeout 5m0s \
  --set global.ha.enabled=false \
  --set global.logAsJson=true \
  --set global.mtls.enabled=true \
  --set global.actors.serviceName=emmy \
  --set global.reminders.serviceName=emmy \
  --set dapr_emmy.logLevel=debug \
  --set dapr_emmy.store.name=postgresql \
  --set dapr_emmy.store.optionsFile.secretName=postgres-actors \
  --set dapr_emmy.store.optionsFile.secretKey=postgres \
  --set dapr_emmy.replicaCount=2 \
  oci://ghcr.io/microsoft/project-emmy/chart/dapr \
  --version $VERSION
```

## Building and running from source

### Clone the code

First, clone the repositories:

```sh
git clone https://github.com/microsoft/project-emmy dapr
```

### Run in standalone mode

To run in standalone mode, you first need to have Postgres running.

Perhaps the quickest way is to run this:

```sh
# In the components-contrib folder
docker-compose -f ./.github/infrastructure/docker-compose-postgresql.yml -p postgresql up -d
```

Next, run the Actors service:

```sh
# This connection string is valid for the Postgres that was started with Docker above
PG_CONNSTRING="postgres://postgres:example@localhost:5432/dapr_test"

# In the dapr folder
go run ./cmd/actors \
  --store-name "pg" \
  --store-opt "connectionString=$PG_CONNSTRING" \
  --log-level debug
```

The Actors service listens on port 51101 by default

Now you can start daprd processes as usual, but instead of passing `--placement-host-address`, pass the address of the actors serviece using `--actors-service-address`. For example:

```
go run \
  -tags allcomponents \
  ./cmd/daprd \
  --app-id myapp \
  --app-port 3000 \
  --dapr-http-port 3603 \
  --dapr-grpc-port 60003 \
  --resources-path ./resources \
  --log-level debug \
  --actors-service-address localhost:51101 
```

> Note: using the Dapr CLI is not supported yet.

### Run in Kubernetes

This involves deploying Dapr to Kubernetes from source in the usual way.

Next, create a Kubernetes secret where you store the connection string for Postgres.

```sh
# These values are valid for the Postgres deployed by the Dapr tests
echo -n "connectionString=host=dapr-postgres-postgresql.dapr-tests.svc.cluster.local user=postgres password=example port=5432 connect_timeout=10 database=dapr_test" > postgres
kubectl create secret generic postgres-actors -n dapr-tests --from-file=postgres
```

When running Helm, make sure to add these options to enable Project Emmy:

```sh
ADDITIONAL_HELM_SET="globals.actors.serviceName=emmy,globals.reminders.serviceName=emmy,dapr_emmy.logLevel=debug,dapr_emmy.store.name=postgresql,dapr_emmy.store.optionsFile.secretName=postgres-actors,dapr_emmy.store.optionsFile.secretKey=postgres" \
  make docker-deploy-k8s
```
