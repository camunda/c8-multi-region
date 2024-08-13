# Define regions
paris := "eu-west-3"
london := "eu-west-2"
frankfurt := "eu-central-1"
cluster_prefix := "lars-saas-test"

# AWS CLI profile (optional, set to "" if not using profiles)
aws_profile := "--profile infex"

# Function to get kubeconfig for a specific cluster
get_kubeconfig region region_alias:
  aws eks --region {{region}} {{aws_profile}} update-kubeconfig --name {{cluster_prefix}}-{{region_alias}} --alias {{region_alias}} --kubeconfig kubeconfig.yaml

kubeconfig_all:
  just get_kubeconfig {{paris}} paris
  just get_kubeconfig {{london}} london
  just get_kubeconfig {{frankfurt}} frankfurt

set_cluster_context region_alias:
  kubectl config use-context {{region_alias}}

create_namespace region_alias namespace:
  just set_cluster_context {{region_alias}}
  kubectl create namespace {{namespace}}

create_all_namespaces:
  just create_namespace paris camunda-paris
  just create_namespace london camunda-london
  just create_namespace frankfurt camunda-frankfurt

deploy_nginx region namespace:
  just set_cluster_context {{region}}
  kubectl apply -f ./aws/dual-region/kubernetes/nginx.yml -n {{namespace}}

create_debug_pod region namespace:
  just set_cluster_context {{region}}
  kubectl run alpine -n {{namespace}} --image alpine --command sleep -- 1d

deploy_all_nginx:
  just deploy_nginx paris camunda-paris
  just deploy_nginx london camunda-london
  just deploy_nginx frankfurt camunda-frankfurt

reach_all_nginx:
  #!/bin/sh
  just set_cluster_context london
  kubectl exec -it sample-nginx -n camunda-london -- /bin/sh -c "curl sample-nginx-peer.camunda-paris.svc.cluster.local"
  if [ $? -ne 0 ]; then
    echo "Failed to reach sample-nginx-peer.camunda-paris.svc.cluster.local from camunda-london"
  fi
  kubectl exec -it sample-nginx -n camunda-london -- /bin/sh -c "curl sample-nginx-peer.camunda-frankfurt.svc.cluster.local"
  if [ $? -ne 0 ]; then
    echo "Failed to reach sample-nginx-peer.camunda-frankfurt.svc.cluster.local from camunda-london"
  fi
  just set_cluster_context paris
  kubectl exec -it sample-nginx -n camunda-paris -- /bin/sh -c "curl sample-nginx-peer.camunda-london.svc.cluster.local"
  if [ $? -ne 0 ]; then
    echo "Failed to reach sample-nginx-peer.camunda-london.svc.cluster.local from camunda-paris"
  fi
  kubectl exec -it sample-nginx -n camunda-paris -- /bin/sh -c "curl sample-nginx-peer.camunda-frankfurt.svc.cluster.local"
  if [ $? -ne 0 ]; then
    echo "Failed to reach sample-nginx-peer.camunda-frankfurt.svc.cluster.local from camunda-paris"
  fi
  just set_cluster_context frankfurt
  kubectl exec -it sample-nginx -n camunda-frankfurt -- /bin/sh -c "curl sample-nginx-peer.camunda-paris.svc.cluster.local"
  if [ $? -ne 0 ]; then
    echo "Failed to reach sample-nginx-peer.camunda-paris.svc.cluster.local from camunda-frankfurt"
  fi
  kubectl exec -it sample-nginx -n camunda-frankfurt -- /bin/sh -c "curl sample-nginx-peer.camunda-london.svc.cluster.local"
  if [ $? -ne 0 ]; then
    echo "Failed to reach sample-nginx-peer.camunda-london.svc.cluster.local from camunda-frankfurt"
  fi

create_all_debug_pods:
  just create_debug_pod paris camunda-paris
  just create_debug_pod london camunda-london
  just create_debug_pod frankfurt camunda-frankfurt

ping_pod_via_ip source_region_alias source_name source_namespace target_region_alias target_name target_namespace count:
  #!/bin/sh
  echo "Pinging {{target_name}} in {{target_region_alias}} from {{source_name}} in {{source_region_alias}}"
  just set_cluster_context {{target_region_alias}}
  pod_ip=$(kubectl get pods {{target_name}} -o jsonpath="{.status.podIP}" -n {{target_namespace}})
  just set_cluster_context {{source_region_alias}}
  avg_ms=$(kubectl exec -it {{source_name}} -n {{source_namespace}} -- /bin/sh -c "ping -c {{count}} $pod_ip" | awk -F '/' 'END {print $5}')
  echo "The average round trip latency for count {{count}} is: $avg_ms"

ping_pod_via_service source_region_alias source_name source_namespace target_region_alias target_name target_namespace count:
  #!/bin/sh
  echo "Pinging {{target_name}} in {{target_region_alias}} from {{source_name}} in {{source_region_alias}}"
  just set_cluster_context {{source_region_alias}}
  avg_ms=$(kubectl exec -it {{source_name}} -n {{source_namespace}} -- /bin/sh -c "ping -c {{count}} {{target_name}}.{{target_namespace}}.svc.cluster.local" | awk -F '/' 'END {print $5}')
  echo "The average round trip latency for count {{count}} is: $avg_ms"

ping_all_pods_via_ip count:
  just ping_pod_via_ip london alpine camunda-london paris alpine camunda-paris {{count}}
  just ping_pod_via_ip london alpine camunda-london frankfurt alpine camunda-frankfurt {{count}}
  just ping_pod_via_ip frankfurt alpine camunda-frankfurt paris alpine camunda-paris {{count}}

ping_all_pods_via_service count:
  just ping_pod_via_service london alpine camunda-london paris sample-nginx-peer camunda-paris {{count}}
  just ping_pod_via_service london alpine camunda-london frankfurt sample-nginx-peer camunda-frankfurt {{count}}
  just ping_pod_via_service frankfurt alpine camunda-frankfurt paris sample-nginx-peer camunda-paris {{count}}

generate_core_dns_entry:
  #!/bin/sh
  export REGION_0={{london}}
  export REGION_1={{paris}}
  export REGION_2={{frankfurt}}
  export CLUSTER_0=london
  export CLUSTER_1=paris
  export CLUSTER_2=frankfurt
  ./aws/dual-region/scripts/generate_core_dns_entry.sh

dns_stitching:
  just set_cluster_context london
  kubectl apply -f ./aws/dual-region/kubernetes/internal-dns-lb.yml
  just set_cluster_context frankfurt
  kubectl apply -f ./aws/dual-region/kubernetes/internal-dns-lb.yml
  just set_cluster_context paris
  kubectl apply -f ./aws/dual-region/kubernetes/internal-dns-lb.yml
  just generate_core_dns_entry
