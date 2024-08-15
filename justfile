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

deploy_elastic:
  just set_cluster_context london
  helm upgrade --install camunda-london oci://registry-1.docker.io/bitnamicharts/elasticsearch \
    -n camunda-london \
    -f ./aws/dual-region/kubernetes/elastic-values.yml \
    --set extraConfig.cluster.routing.allocation.awareness.attributes=region \
    --set extraConfig.node.attr.region=london
  kubectl apply -f ./aws/dual-region/kubernetes/elastic-metrics-headless.yml
  just set_cluster_context frankfurt
  helm upgrade --install camunda-frankfurt oci://registry-1.docker.io/bitnamicharts/elasticsearch \
    -n camunda-frankfurt \
    -f ./aws/dual-region/kubernetes/elastic-values.yml \
    --set extraConfig.cluster.routing.allocation.awareness.attributes=region \
    --set extraConfig.node.attr.region=frankfurt
  kubectl apply -f ./aws/dual-region/kubernetes/elastic-metrics-headless.yml
  just set_cluster_context paris
  helm upgrade --install camunda-paris oci://registry-1.docker.io/bitnamicharts/elasticsearch \
    -n camunda-paris \
    -f ./aws/dual-region/kubernetes/elastic-values.yml \
    --set extraConfig.cluster.routing.allocation.awareness.attributes=region \
    --set extraConfig.node.attr.region=paris
  kubectl apply -f ./aws/dual-region/kubernetes/elastic-metrics-headless.yml

remove_elastic:
  just set_cluster_context london
  helm uninstall camunda-london -n camunda-london
  kubectl delete pvc -n camunda-london --all
  just set_cluster_context frankfurt
  helm uninstall camunda-frankfurt -n camunda-frankfurt
  kubectl delete pvc -n camunda-frankfurt --all
  just set_cluster_context paris
  helm uninstall camunda-paris -n camunda-paris
  kubectl delete pvc -n camunda-paris --all

deploy_monitoring:
  just set_cluster_context paris
  helm upgrade --install prom prometheus-community/prometheus -f ./aws/dual-region/kubernetes/prometheus-values.yml -n monitoring --create-namespace
  helm upgrade --install graf grafana/grafana -n monitoring --set persistence.enabled=true

get_grafana_admin:
  #!/bin/sh
  admin=$(kubectl get secret graf-grafana -n monitoring -o jsonpath='{.data.admin-user}' | base64 --decode)
  password=$(kubectl get secret graf-grafana -n monitoring -o jsonpath='{.data.admin-password}' | base64 --decode)
  echo "$admin:$password"

get_elastic_lbs:
  #!/bin/sh
  just set_cluster_context london
  london_lb=$(kubectl get service camunda-london-elasticsearch -o jsonpath='{.status.loadBalancer.ingress[0].hostname}')
  just set_cluster_context frankfurt
  frankfurt_lb=$(kubectl get service camunda-frankfurt-elasticsearch -o jsonpath='{.status.loadBalancer.ingress[0].hostname}')
  just set_cluster_context paris
  paris_lb=$(kubectl get service camunda-paris-elasticsearch -o jsonpath='{.status.loadBalancer.ingress[0].hostname}')
  echo "London: $london_lb"
  echo "Frankfurt: $frankfurt_lb"
  echo "Paris: $paris_lb"

create_kubeconfig region_alias region:
  #!/bin/sh
  TOKEN=$(kubectl get secret temp-admin-secret -n default -o jsonpath="{.data.token}" | base64 --decode)
  CLUSTER_NAME=$(kubectl config view --minify -o jsonpath='{.clusters[0].name}')
  SERVER_URL=$(kubectl config view --minify -o jsonpath='{.clusters[0].cluster.server}')
  CERTIFICATE=$(aws eks describe-cluster --region={{region}} --name={{cluster_prefix}}-{{region_alias}} --output text --query 'cluster.{certificateAuthorityData: certificateAuthority.data}')
  cat <<EOF > kubeconfig-{{region_alias}}.yaml
  apiVersion: v1
  kind: Config
  clusters:
  - cluster:
      certificate-authority-data: $CERTIFICATE
      server: $SERVER_URL
    name: $CLUSTER_NAME
  contexts:
  - context:
      cluster: $CLUSTER_NAME
      user: temp-admin
    name: {{region_alias}}
  current-context: {{region_alias}}
  users:
  - name: temp-admin
    user:
      token: $TOKEN
  EOF

create_temp_admin:
  #!/bin/sh
  just set_cluster_context paris
  kubectl apply -f ./aws/dual-region/kubernetes/temp-admin.yml
  just create_kubeconfig paris {{paris}}
  just set_cluster_context london
  kubectl apply -f ./aws/dual-region/kubernetes/temp-admin.yml
  just create_kubeconfig london {{london}}
  just set_cluster_context frankfurt
  kubectl apply -f ./aws/dual-region/kubernetes/temp-admin.yml
  just create_kubeconfig frankfurt {{frankfurt}}

  KUBECONFIG=./kubeconfig-paris.yaml:./kubeconfig-london.yaml:./kubeconfig-frankfurt.yaml
  kubectl config view --merge --flatten > temp-admin-kubeconfig.yaml

  rm -f ./kubeconfig-paris.yaml ./kubeconfig-london.yaml ./kubeconfig-frankfurt.yaml
