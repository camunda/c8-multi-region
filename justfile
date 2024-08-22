# Define regions
paris := "eu-west-3"
london := "eu-west-2"
frankfurt := "eu-central-1"
cluster_prefix := "lars-saas-test"

# Helm chart versions
elastic_helm_version := "21.3.8"
grafana_helm_version := "8.4.4"
prometheus_helm_version := "25.26.0"
cluster_autoscaler_helm_version := "9.37.0"
camunda_helm_version := "10.3.2"

# Optional DNS Stuff
INGRESS_HELM_CHART_VERSION := "4.11.2"
EXTERNAL_DNS_HELM_CHART_VERSION := "1.14.5"
CERT_MANAGER_HELM_CHART_VERSION := "1.15.3"

EXTERNAL_DNS_IRSA_ARN := "arn:aws:iam::444804106854:role/lars-saas-test-paris-external-dns-role"
CERT_MANAGER_IRSA_ARN := "arn:aws:iam::444804106854:role/lars-saas-test-paris-cert-manager-role"
mail := "lars.lange@camunda.com"

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
    --version {{elastic_helm_version}} \
    -n camunda-london \
    -f ./aws/dual-region/kubernetes/elastic-values.yml \
    --set extraConfig.cluster.routing.allocation.awareness.attributes=region \
    --set extraConfig.node.attr.region=london
  kubectl apply -f ./aws/dual-region/kubernetes/elastic-metrics-headless.yml
  just set_cluster_context frankfurt
  helm upgrade --install camunda-frankfurt oci://registry-1.docker.io/bitnamicharts/elasticsearch \
    --version {{elastic_helm_version}} \
    -n camunda-frankfurt \
    -f ./aws/dual-region/kubernetes/elastic-values.yml \
    --set extraConfig.cluster.routing.allocation.awareness.attributes=region \
    --set extraConfig.node.attr.region=frankfurt
  kubectl apply -f ./aws/dual-region/kubernetes/elastic-metrics-headless.yml
  just set_cluster_context paris
  helm upgrade --install camunda-paris oci://registry-1.docker.io/bitnamicharts/elasticsearch \
    --version {{elastic_helm_version}} \
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
  helm upgrade --install prom prometheus-community/prometheus \
    --version {{prometheus_helm_version}} \
    -f ./aws/dual-region/kubernetes/prometheus-values.yml \
    -n monitoring \
    --create-namespace
  helm upgrade --install graf grafana/grafana \
    --version {{grafana_helm_version}} \
    -n monitoring \
    --set persistence.enabled=true

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
      user: temp-admin-{{region_alias}}
    name: {{region_alias}}
  current-context: {{region_alias}}
  users:
  - name: temp-admin-{{region_alias}}
    user:
      token: $TOKEN
  EOF

create_temp_admin:
  #!/bin/sh
  just set_cluster_context paris
  kubectl apply -f ./aws/dual-region/kubernetes/temp-admin-paris.yml -n default
  just create_kubeconfig paris {{paris}}
  just set_cluster_context london
  kubectl apply -f ./aws/dual-region/kubernetes/temp-admin-london.yml -n default
  just create_kubeconfig london {{london}}
  just set_cluster_context frankfurt
  kubectl apply -f ./aws/dual-region/kubernetes/temp-admin-frankfurt.yml -n default
  just create_kubeconfig frankfurt {{frankfurt}}

  KUBECONFIG=./kubeconfig-paris.yaml:./kubeconfig-london.yaml:./kubeconfig-frankfurt.yaml
  kubectl config view --merge --flatten > temp-admin-kubeconfig.yaml

  rm -f ./kubeconfig-paris.yaml ./kubeconfig-london.yaml ./kubeconfig-frankfurt.yaml

deploy_cluster_autoscaler:
  #!/bin/sh
  role_arn=$(terraform output -state ./aws/dual-region/terraform/terraform.tfstate -raw eks_autoscaling_role_arn)
  just set_cluster_context london
  helm upgrade --install cluster-autoscaler cluster-autoscaler/cluster-autoscaler \
    --version {{cluster_autoscaler_helm_version}} \
    -n cluster-autoscaler \
    --create-namespace \
    --set autoDiscovery.clusterName={{cluster_prefix}}-london \
    --set awsRegion={{london}} \
    --set rbac.serviceAccount.annotations."eks\.amazonaws\.com/role-arn"=$role_arn
  just set_cluster_context frankfurt
  helm upgrade --install cluster-autoscaler cluster-autoscaler/cluster-autoscaler \
    --version {{cluster_autoscaler_helm_version}} \
    -n cluster-autoscaler \
    --create-namespace \
    --set autoDiscovery.clusterName={{cluster_prefix}}-frankfurt \
    --set awsRegion={{frankfurt}} \
    --set rbac.serviceAccount.annotations."eks\.amazonaws\.com/role-arn"=$role_arn
  just set_cluster_context paris
  helm upgrade --install cluster-autoscaler cluster-autoscaler/cluster-autoscaler \
    --version {{cluster_autoscaler_helm_version}} \
    -n cluster-autoscaler \
    --create-namespace \
    --set autoDiscovery.clusterName={{cluster_prefix}}-paris \
    --set awsRegion={{paris}} \
    --set rbac.serviceAccount.annotations."eks\.amazonaws\.com/role-arn"=$role_arn

deploy_camunda:
  #!/bin/sh
  export REGIONS=3
  export CAMUNDA_NAMESPACE_0=camunda-london
  export CAMUNDA_NAMESPACE_1=camunda-frankfurt
  export CAMUNDA_NAMESPACE_2=camunda-paris
  export HELM_RELEASE_NAME=camunda
  export CLUSTER_SIZE=6

  # we need to escape the comma for helm set
  contact_points=$(./aws/dual-region/scripts/generate_zeebe_helm_values.sh | sed 's/,/\\,/g')

  just set_cluster_context london
  helm upgrade --install camunda camunda/camunda-platform \
    --version {{camunda_helm_version}} \
    -n camunda-london \
    -f ./aws/dual-region/kubernetes/camunda-values.yml \
    --set zeebe.env[3].name=ZEEBE_BROKER_CLUSTER_INITIALCONTACTPOINTS \
    --set zeebe.env[3].value="$contact_points" \
    --set global.multiregion.regionId="0"
  just set_cluster_context frankfurt
  helm upgrade --install camunda camunda/camunda-platform \
    --version {{camunda_helm_version}} \
    -n camunda-frankfurt \
    -f ./aws/dual-region/kubernetes/camunda-values.yml \
    --set zeebe.env[3].name=ZEEBE_BROKER_CLUSTER_INITIALCONTACTPOINTS \
    --set zeebe.env[3].value="$contact_points" \
    --set global.multiregion.regionId="1"
  just set_cluster_context paris
  helm upgrade --install camunda camunda/camunda-platform \
    -n camunda-paris \
    --version {{camunda_helm_version}} \
    -f ./aws/dual-region/kubernetes/camunda-values.yml \
    --set zeebe.env[3].name=ZEEBE_BROKER_CLUSTER_INITIALCONTACTPOINTS \
    --set zeebe.env[3].value="$contact_points" \
    --set-string 'operate.env[0].value=true' \
    --set-string 'operate.env[1].value=true' \
    --set global.multiregion.regionId="2"

remove_camunda:
  just set_cluster_context london
  helm uninstall camunda -n camunda-london
  kubectl delete pvc -l app.kubernetes.io/component=zeebe-broker
  just set_cluster_context frankfurt
  helm uninstall camunda -n camunda-frankfurt
  kubectl delete pvc -l app.kubernetes.io/component=zeebe-broker
  just set_cluster_context paris
  helm uninstall camunda -n camunda-paris
  kubectl delete pvc -l app.kubernetes.io/component=zeebe-broker

deploy_dns_stack:
  #!/bin/sh
  just set_cluster_context paris
  # ingress-nginx
  helm upgrade --install \
  ingress-nginx ingress-nginx \
  --repo https://kubernetes.github.io/ingress-nginx \
  --version {{INGRESS_HELM_CHART_VERSION}} \
  --set 'controller.service.annotations.service\.beta\.kubernetes\.io\/aws-load-balancer-backend-protocol=tcp' \
  --set 'controller.service.annotations.service\.beta\.kubernetes\.io\/aws-load-balancer-cross-zone-load-balancing-enabled=true' \
  --set 'controller.service.annotations.service\.beta\.kubernetes\.io\/aws-load-balancer-type=nlb' \
  --namespace ingress-nginx \
  --create-namespace
  # external-dns
  helm upgrade --install \
    external-dns external-dns \
    --repo https://kubernetes-sigs.github.io/external-dns/ \
    --version {{EXTERNAL_DNS_HELM_CHART_VERSION}} \
    --set "env[0].name=AWS_DEFAULT_REGION" \
    --set "env[0].value={{paris}}" \
    --set txtOwnerId=external-dns-ml-saas \
    --set policy=sync \
    --set "serviceAccount.annotations.eks\.amazonaws\.com\/role-arn={{EXTERNAL_DNS_IRSA_ARN}}" \
    --namespace external-dns \
    --create-namespace
  # cert-manager
  kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v{{CERT_MANAGER_HELM_CHART_VERSION}}/cert-manager.crds.yaml
  helm upgrade --install \
    cert-manager cert-manager \
    --repo https://charts.jetstack.io \
    --version {{CERT_MANAGER_HELM_CHART_VERSION}} \
    --namespace cert-manager \
    --create-namespace \
    --set "serviceAccount.annotations.eks\.amazonaws\.com\/role-arn={{CERT_MANAGER_IRSA_ARN}}" \
    --set securityContext.fsGroup=1001 \
    --set ingressShim.defaultIssuerName=letsencrypt \
    --set ingressShim.defaultIssuerKind=ClusterIssuer \
    --set ingressShim.defaultIssuerGroup=cert-manager.io
  cat << EOF | kubectl apply -f -
  ---
  apiVersion: cert-manager.io/v1
  kind: ClusterIssuer
  metadata:
    name: letsencrypt
  spec:
    acme:
      server: https://acme-v02.api.letsencrypt.org/directory
      email: {{mail}}
      privateKeySecretRef:
        name: letsencrypt-issuer-account-key
      solvers:
        - selector: {}
          dns01:
            route53:
              region: {{paris}}
              # Cert-manager will automatically observe the hosted zones
              # Cert-manager will automatically make use of the IRSA assigned service account
  EOF

remove_region_elastic region_alias delete_pvc:
  #!/bin/sh
  just set_cluster_context {{region_alias}}
  helm uninstall camunda-{{region_alias}} -n camunda-{{region_alias}}
  if [[ {{delete_pvc}} == "true" ]]; then
    kubectl delete pvc -l app.kubernetes.io/name=elasticsearch -n camunda-{{region_alias}}
  fi

remove_region_zeebe region_alias delete_pvc:
  #!/bin/sh
  just set_cluster_context {{region_alias}}
  helm uninstall camunda -n camunda-{{region_alias}}
  if [[ {{delete_pvc}} == "true" ]]; then
    kubectl delete pvc -l app.kubernetes.io/component=zeebe-broker -n camunda-{{region_alias}}
  fi

deploy_region_elastic region_alias:
  just set_cluster_context {{region_alias}}
  helm upgrade --install camunda-{{region_alias}} oci://registry-1.docker.io/bitnamicharts/elasticsearch \
    --version {{elastic_helm_version}} \
    -n camunda-{{region_alias}} \
    -f ./aws/dual-region/kubernetes/elastic-values.yml \
    --set extraConfig.cluster.routing.allocation.awareness.attributes=region \
    --set extraConfig.node.attr.region={{region_alias}}

deploy_benchmark:
  just set_cluster_context paris
  kubectl apply -n camunda-paris -f ./benchmark/starter.yaml
  kubectl apply -n camunda-paris -f ./benchmark/worker.yaml

remove_benchmark:
  just set_cluster_context paris
  kubectl delete -n camunda-paris -f ./benchmark/starter.yaml
  kubectl delete -n camunda-paris -f ./benchmark/worker.yaml

# For camunda just use `just deploy_camunda`
