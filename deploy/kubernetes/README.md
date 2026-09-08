# Kubernetes operator notes

These manifests deploy each Sennet process role. They do not provision Kafka, PostgreSQL, ClickHouse Keeper, ClickHouse replicas, an ingress controller, TLS, or a production object-store archive. Replica counts remain one and no availability claim is made.

Create the production secret before applying the distributed roles:

```text
kubectl -n sennet create secret generic sennet-data-plane \
  --from-literal=postgres-url='postgres://...' \
  --from-literal=kafka-brokers='broker-0:9092,broker-1:9092,broker-2:9092' \
  --from-literal=clickhouse-url='https://clickhouse.example' \
  --from-literal=clickhouse-user='sennet' \
  --from-literal=clickhouse-password='...' \
  --from-file=firebase-service-account-json=service-account.json \
  --from-literal=operator-token='...'
```

Set the immutable image digest in `kustomization.yaml`, apply the ClickHouse cluster DDL and Kafka production topic configuration first, then run `kubectl apply -k deploy/kubernetes`. Keep the `all-in-one` deployment at zero replicas in a distributed installation. It exists only as an opt-in local evaluation role; create `sennet-local-evaluation` and scale it to one only in a disposable cluster.

Gateway/query requests carry short-lived Firebase login ID tokens. There are no Sennet API keys. Do not place the operator token or Firebase service-account material in browsers. Use a secrets operator or encrypted secret workflow instead of checking rendered Secret objects into source control.

Scale gateway and query/control only after measuring their saturation and dependency behavior. A storage-consumer replica count above the partition count is idle capacity; changing its consumer group starts an independent read. Verify lag, duplicate collapse, poison handling, archive durability, graceful termination, and dependency failure behavior before promotion.
