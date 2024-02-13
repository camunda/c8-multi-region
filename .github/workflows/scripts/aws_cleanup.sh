#!/bin/bash

echo "Deleting additional resources..."
# KMS keys can't be deleted due to resource policies, requires manual intervention

echo "Deleting IAM Roles"
# Detach permissions and profile instances and delete IAM roles
role_arns=$(aws iam list-roles --query "Roles[?contains(RoleName, 'nightly')].RoleName" --output text)

read -r -a role_arns_array <<< "$role_arns"

for role_arn in "${role_arns_array[@]}"
do
    echo "Removing instance profiles and policies of role: $role_arn"
    attached_policy_arns=$(aws iam list-attached-role-policies --role-name "$role_arn" --query 'AttachedPolicies[].PolicyArn' --output text)
    read -r -a attached_policy_arns_array <<< "$attached_policy_arns"

    for policy_arn in "${attached_policy_arns_array[@]}"
    do
        echo "Removing attached policy: $policy_arn"
        aws iam detach-role-policy --role-name "$role_arn" --policy-arn "$policy_arn"
    done

    policy_arns=$(aws iam list-role-policies --role-name "$role_arn" --query 'PolicyNames' --output text)
    read -r -a policy_arns_array <<< "$policy_arns"

    for policy_arn in "${policy_arns_array[@]}"
    do
        echo "Deleting policy: $policy_arn"
        aws iam delete-role-policy --role-name "$role_arn" --policy-name "$policy_arn"
    done

    instance_profile_arns=$(aws iam list-instance-profiles-for-role --role-name "$role_arn" --query 'InstanceProfiles[].InstanceProfileName' --output text)
    read -r -a instance_profile_arns_array <<< "$instance_profile_arns"

    for instance_profile_arn in "${instance_profile_arns_array[@]}"
    do
        echo "Removing instance profile: $instance_profile_arn"
        aws iam remove-role-from-instance-profile --instance-profile-name "$instance_profile_arn" --role-name "$role_arn"
    done

    echo "Deleting role: $role_arn"
    aws iam delete-role --role-name "$role_arn"

done

echo "Deleting IAM Policies"
# Delete Policies
iam_policies=$(aws iam list-policies --query "Policies[?contains(PolicyName, 'nightly')].Arn" --output text)

read -r -a iam_policies_array <<< "$iam_policies"

for iam_policy in "${iam_policies_array[@]}"
do
    echo "Deleting policy: $iam_policy"
    aws iam delete-policy --policy-arn "$iam_policy"
done

echo "Deleting OIDC Providers"
# Delete OIDC Provider
oidc_providers=$(aws iam list-open-id-connect-providers --query "OpenIDConnectProviderList[?contains(Arn, 'eu-west-2') || contains(Arn, 'eu-west-3')].Arn" --output text)

read -r -a oidc_providers_array <<< "$oidc_providers"

for oidc_provider in "${oidc_providers_array[@]}"
do
    echo "Deleting OIDC Provider: $oidc_provider"
    aws iam delete-open-id-connect-provider --open-id-connect-provider-arn "$oidc_provider"
done

echo "Deleting VPC Peering Connections"
# Delete VPC Peering Connection
peering_connection_ids=$(aws ec2 describe-vpc-peering-connections --region eu-west-2 --query "VpcPeeringConnections[?Status.Code == 'active' && Tags[?contains(Value, 'nightly')]]".VpcPeeringConnectionId --output text)

read -r -a peering_connection_ids_array <<< "$peering_connection_ids"

for peering_connection_id in "${peering_connection_ids_array[@]}"
do
    echo "Deleting VPC Peering Connection: $peering_connection_id"
    aws ec2 delete-vpc-peering-connection --region eu-west-2 --vpc-peering-connection-id "$peering_connection_id"
done
