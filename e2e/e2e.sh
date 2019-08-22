#!/bin/bash -euo pipefail

# This script performs the following actions:
#
# 1. Create an EKS cluster with eksctl, based on the cluster.yaml file
# 2. Deploy the AWS ALB Ingress controller
# 3. Deploy the kube-readiness app
# 4. Perform a rolling update of the kube-readiness app
# 5. Start a k6 load-test

if ! [ -x "$(command -v eksctl)" ]; then
  echo 'Error: eksctl is not installed.' >&2
  exit 1
fi

if ! [ -x "$(command -v k6)" ]; then
  echo 'Error: k6 is not installed.' >&2
  exit 1
fi

if ! [ -x "$(command -v kubectl)" ]; then
  echo 'Error: kubectl is not installed.' >&2
  exit 1
fi

if ! [ -x "$(command -v aws)" ]; then
  echo 'Error: aws is not installed.' >&2
  exit 1
fi

# Global variables

NAMESPACE=${NAMESPACE:-default}
AWS_PROFILE=${AWS_PROFILE:-default}
CLUSTER_NAME=${CLUSTER_NAME:-kube-readiness}
KUBECONFIG="./kube-config-$CLUSTER_NAME"
APP_LB_DNS=""
TG_ARN=""

# Always clean up the kube-readiness server deployment and remove the kubeconfig file

function finish {
  kubectl -n $NAMESPACE delete --ignore-not-found=true -f ./app/k8s/deployment.yaml > /dev/null

  rm -f $KUBECONFIG
}
trap finish EXIT

# 1. Create the EKS cluster if it doesn't exist

if ! eksctl -p $AWS_PROFILE get cluster $CLUSTER_NAME > /dev/null; then
  sed "s/__CLUSTER_NAME__/$CLUSTER_NAME/g" cluster.yaml | eksctl -p $AWS_PROFILE create cluster --kubeconfig $KUBECONFIG -f -
fi

# 2. Install AWS ALB Ingress controller

kubectl apply -f ./alb-ingress-controller > /dev/null

# 3a. Deploy the kube-readiness ingress

kubectl apply -n $NAMESPACE -f app/k8s/ingress.yaml > /dev/null

echo -n "Waiting for loadbalancer DNS location to become available"
while [[ -z $APP_LB_DNS ]]; do
  APP_LB_DNS=$(aws --region=eu-west-1 elbv2 describe-load-balancers --query "LoadBalancers[?contains(LoadBalancerName,'$NAMESPACE') && contains(LoadBalancerName 'kuberead')].DNSName" --output text)
  printf '.'
  sleep 2
done

# Get ingress loadbalancer ARN
LB_ARN=$(aws --region=eu-west-1 elbv2 describe-load-balancers --query "LoadBalancers[?DNSName==\`$APP_LB_DNS\`].LoadBalancerArn" --output text)

# 3b. Deploy the kube readiness app

# Check for previous deployment of kube-readiness app and clean it up
if [[ ! -z $(kubectl get deployment --ignore-not-found kube-readiness) ]]; then

  # Make sure the kube-readiness deployment has been deleted
  kubectl -n $NAMESPACE delete --ignore-not-found=true -f app/k8s/deployment.yaml > /dev/null

  # Get ingress target group ARN
  echo
  echo -n "Waiting for targetgroup"
  while [[ -z $TG_ARN ]]; do
    TG_ARN=$(aws --region=eu-west-1 elbv2 describe-target-groups --query "TargetGroups[?contains(LoadBalancerArns, '$LB_ARN')].TargetGroupArn" --output text)
    printf '.'
    sleep 2
  done

  # Wait for an empty targetgroup
  echo
  echo -n "Waiting for all loadbalancer targets to deregister"
  until [[ -z $(aws --region=eu-west-1 elbv2 describe-target-health --target-group-arn $TG_ARN --output text --query "TargetHealthDescriptions") ]]; do
      printf '.'
      sleep 2
  done

fi

# Deploy kube-readiness app

kubectl -n $NAMESPACE apply -f app/k8s > /dev/null

# Wait for successful loadbalancer response
echo
echo -n "Waiting for kube-readiness app to respond"
until $(curl --output /dev/null --silent --head --fail http://$APP_LB_DNS); do
    printf '.'
    sleep 2
done

# 4. Perform rolling upgrade of the kube-readiness appr (1 -> 2)
echo
echo "Executing rolling upgrade"
sed "s/:1/:2/g" app/k8s/deployment.yaml | kubectl apply -n $NAMESPACE -f - > /dev/null

# 5. Start the k6 load test
echo "Starting load test"
APP_LB_DNS=$APP_LB_DNS k6 run k6.js
