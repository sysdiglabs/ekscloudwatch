# EKS audit integration example

The following instructions show how to deploy a simple application that reads EKS Kubernetes audit logs and forwards them to the Sysdig Secure agent.
The steps below show an example configuration implemented with the AWS console, but the same can be done with scripts, API calls or Infrastructure-as-Code configurations.

These instructions have been tested with eks.5 on Kubernetes v1.14.

## EKS setup: enable CloudWatch audit logs

Your EKS cluster needs be configured to forward audit logs to CloudWatch, which is disabled by default.

1. Open the EKS dashboard from the AWS console
1. Select your cluster > _Logging_ > _Update_ and enable _Audit_

![Audit Enabled](readme_img/audit_logs.png)

## EKS setup: configure the VPC endpoint

Your VPC needs an endpoint for the service `com.amazonaws.<your-region>.logs`, accessible from all the EKS security groups.

1. Open the VPC dashboard from the AWS console
1. Select _Endpoints_ > _Create Endpoints_
1. Select _Find service by name_, enter `com.amazonaws.<your-region>.logs` and click "Verify".
1. Under VPC select your cluster's VPC
1. Select all security groups

## EKS setup: configure EC2 instance profiles and roles

The EC2 instances that make up your EKS cluster must have the necessary permission to read CW logs. Usually they all use the same IAM Role, so that is the one to configure.

1. Open the EC2 dashboard from the AWS console
1. Select the AWS EC2 instances that are configured as cluster nodes
1. Select the associated IAM Role, which should be the same for all nodes
1. Find the policy `CloudWatchReadOnlyAccess` and attach it

![Permissions](readme_img/attach_permissions.png)

## Deploy the client and its configmap

We can now deploy the log forwarder itself along with its configmap.

```
$ kubectl --namespace sysdig-agent apply -f ./ekscloudwatch-config.yaml
configmap/ekscloudwatch-config created
$ kubectl --namespace sysdig-agent apply -f ./deployment.yaml
deployment.apps/eks-cloudwatch created
```

To check if the forwarder is configured and working correctly you can check the logs for the pod that you just deployed in the `sysdig-agent` namespace. 

You should see k8s audit related events in the Sysdig Secure dashboard.
