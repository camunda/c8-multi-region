#!/bin/bash

kubectl --context "$CLUSTER_0" patch storageclass gp2 \
  -p '{"metadata":{"annotations":{"storageclass.kubernetes.io/is-default-class":"false"}}}'
kubectl --context "$CLUSTER_1" patch storageclass gp2 \
  -p '{"metadata":{"annotations":{"storageclass.kubernetes.io/is-default-class":"false"}}}'

kubectl --context "$CLUSTER_0" apply -f "../kubernetes/storage-class.yml"
kubectl --context "$CLUSTER_1" apply -f "../kubernetes/storage-class.yml"
