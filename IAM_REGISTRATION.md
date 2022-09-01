[comment]: # ( Copyright Contributors to the Open Cluster Management project )
# AWS IAM Registration

## Hub Initialization

As per existing - no changes

## Spoke Join Procedure

Two options will be provided:

1. Self managed IAM - when the user prefers to use IaC e.g. terraform to manage IAM role and policy creation
2. Clusteradm managed IAM - `clusteradm` will take care of creating necessary IAM roles and policies

We'll consider option 2 in the example:

```shell
% clusteradm join \
  --hub-token ${HUB_TOKEN} \
  --hub-apiserver https://1234567890ABCDEF1234567890ABCDEF.gr7.us-west-2.eks.amazonaws.com \
  --cluster-name worker-0 \
  --registration-type aws-iam \
  --aws-create-iam-role true \
  --aws-eks-cluster worker-0 \
  --aws-hub-account-id 2222222222 \
  --aws-tags application=ocm-poc
  
getting worker account id from sts identity
EKS worker cluster worker-0 exists!
OIDC is configured for EKS cluster, with issuer 1234567890ABCDEF1234567890ABCDEF
OIDC Provider oidc.eks.us-west-2.amazonaws.com/id/1234567890ABCDEF1234567890ABCDEF exists!
Policy arn:aws:iam::1111111111:policy/ocm.worker.worker-0 does not exist - will create!
Role arn:aws:iam::1111111111:role/ocm.worker.worker-0 does not exist - will create!
Finished validating AWS environment - OK!

Resources will be created with the following tags:
application: ocm-poc
costcenter: 12345
open-cluster-management.io/cluster: worker-0
open-cluster-management.io/managed: true

Creating IAM policy ocm.worker.worker-0 to allow the worker to assume a role in the hub's account
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": "sts:AssumeRole",
      "Resource": "arn:aws:iam::2222222222:role/ocm.remote-worker.worker-0"
    }
  ]
}
Created policy arn:aws:iam::1111111111:policy/ocm.worker.worker-0
Creating IAM role ocm.worker.worker-0 with attached policy...
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:aws:iam::1111111111:oidc-provider/oidc.eks.us-west-2.amazonaws.com/id/1234567890ABCDEF1234567890ABCDEF"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "oidc.eks.us-west-2.amazonaws.com/id/1234567890ABCDEF1234567890ABCDEF:sub": "system:serviceaccount:open-cluster-management-agent:klusterlet-registration-sa",
          "oidc.eks.us-west-2.amazonaws.com/id/1234567890ABCDEF1234567890ABCDEF:aud": "sts.amazonaws.com"
        }
      }
    }
  ]
}
Created role arn:aws:iam::1111111111:role/ocm.worker.worker-0
Attached policy role arn:aws:iam::1111111111:policy/ocm.worker.worker-0 to role arn:aws:iam::1111111111:role/ocm.worker.worker-0
Please log onto the hub cluster and run the following command:

    /Users/dgorst/go/bin/clusteradm accept --clusters worker-0

```
- TODO clean up output from join command
- TODO add additional required options to the join command

Note the `--aws-tags` flag is optional - however it's common for businesses to enforce certain policies that these are included.

### Explanation

#### Created IAM resources

An IAM role `ocm.worker.worker-0` and attached trust and permission policies are created that will be assumed by the registration agent pod's service account using IRSA (IAM Roles For Service Accounts).

This trust policy will ensure IAM trusts the OIDC provider of the worker cluster to attest to the identity of the registration agent's service account:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:aws:iam::1111111111:oidc-provider/oidc.eks.us-west-2.amazonaws.com/id/1234567890ABCDEF1234567890ABCDEF"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "oidc.eks.us-west-2.amazonaws.com/id/1234567890ABCDEF1234567890ABCDEF:sub": "system:serviceaccount:open-cluster-management-agent:klusterlet-registration-sa",
          "oidc.eks.us-west-2.amazonaws.com/id/1234567890ABCDEF1234567890ABCDEF:aud": "sts.amazonaws.com"
        }
      }
    }
  ]
}
```

The permission policy will allow assuming a corresponding role in the hub's account once it is created (when we accept on the hub):

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": "sts:AssumeRole",
      "Resource": "arn:aws:iam::2222222222:role/ocm.remote-worker.worker-0"
    }
  ]
}
```

Note the service account features the IRSA annotation `eks.amazonaws.com/role-arn` mapping it to the created role:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::1111111111:role/ocm.worker.worker-0
  name: klusterlet-registration-sa
```


#### ManagedCluster Resource

The ManagedCluster created by the registration agent has an additional annotation identifying the remote worker's role arn:

```yaml
apiVersion: cluster.open-cluster-management.io/v1
kind: ManagedCluster
metadata:
  annotations:
    open-cluster-management.io/aws-iam-worker-role: arn:aws:iam::1111111111:role/ocm.worker.worker-0
```

This will be used by the `clusteradm accept` command to set up necessary policies that will allow this role to assume a corresponding role in the hub cluster account.

#### Registration Agent

The registration agent has AWS credentials (injected by irsa) for the `arn:aws:iam::1111111111:role/ocm.worker.worker-0` role and will use these to attempt to assume the `arn:aws:iam::2222222222:role/ocm.remote-worker.worker-0` role in the hub's account.
At this point, that role does not exist so will error and continue retrying. 

Once the hub accepts this cluster, it will successfully assume the remote role and can generate a kubeconfig containing an IAM token. The kubeconfig is written to a Secret as per the existing CSR registration method. 
This token is periodically refreshed as per its expiration. 

**Note that a CSR is not created when IAM has been chosen as the registration method.** 

## Accept Procedure

Accept is largely similar to when using CSR, however a couple of additional options must be provided:

```shell
% clusteradm accept --clusters worker-0 \
  --aws-hub-eks-cluster hub \
  --aws-tags application=ocm-poc \
  --aws-tags costcenter=12345
  
Creating IAM policy ocm.remote-worker.worker-0 to allow the worker to assume a role in the hub's account
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "eks:DescribeIdentityProviderConfig",
        "eks:AccessKubernetesApi",
        "eks:DescribeCluster"
      ],
      "Resource": [
        "arn:aws:eks:*:2222222222:identityproviderconfig/*/*/*/*",
        "arn:aws:eks:*:2222222222:cluster/*"
      ]
    }
  ]
}
Creating IAM role ocm.remote-worker.worker-0 with attached policy...
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "AWS": "arn:aws:iam::1111111111:root"
      },
      "Action": "sts:AssumeRole",
      "Condition": {}
    }
  ]
}
Created role arn:aws:iam::2222222222:role/ocm.remote-worker.worker-0
Attached policy role arn:aws:iam::2222222222:policy/ocm.remote-worker.worker-0 to role arn:aws:iam::464976251208:role/ocm.remote-worker.worker-0
updating aws-auth to map role arn:aws:iam::2222222222:role/ocm.remote-worker.worker-0 to group system:open-cluster-management:worker-0
set hubAcceptsClient to true for managed cluster worker-0

 Your managed cluster worker-0 has joined the Hub successfully. Visit https://open-cluster-management.io/scenarios or https://github.com/open-cluster-management-io/OCM/tree/main/solutions for next steps.

```

As with join, the `--aws-tags` flag is optional.

### IAM Resources

A role `arn:aws:iam::2222222222:role/ocm.remote-worker.worker-0` is created specifically for the remote worker cluster, in the hub account.

The trust policy allows the worker account to assume this role in the hub account:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "AWS": "arn:aws:iam::1111111111:root"
      },
      "Action": "sts:AssumeRole",
      "Condition": {}
    }
  ]
}
```

The permission policy allows the worker cluster to describe the hub cluster (required to gather certain information to generate a kubeconfig), and to access the kube api of the hub cluster:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "eks:DescribeIdentityProviderConfig",
        "eks:AccessKubernetesApi",
        "eks:DescribeCluster"
      ],
      "Resource": [
        "arn:aws:eks:*:2222222222:identityproviderconfig/*/*/*/*",
        "arn:aws:eks:*:2222222222:cluster/*"
      ]
    }
  ]
}
```

### Mapping the role to a Kubernetes group

`clusteradm accept` updates the `kube-system/aws-auth` configmap to map the `ocm.remote-worker.worker-0` role to the group:

```shell
% kubectl get cm aws-auth -n kube-system -o yaml                                    
apiVersion: v1
kind: ConfigMap
data:
  mapRoles: |
    - rolearn: arn:aws:iam::2222222222:role/ocm.remote-worker.worker-0
      username: ""
      groups:
      - system:open-cluster-management:worker-0
...
```

From this point, the role can be used to obtain credentials that map to the worker's group.

### Returning data to the worker to assume the remote role

The registration agent on the worker cluster requires some additional data in order to assume the remote role. This is added to the ManagedCluster resource as a few annotations:

```yaml
apiVersion: cluster.open-cluster-management.io/v1
kind: ManagedCluster
metadata:
  annotations:
    open-cluster-management.io/aws-iam-hub-account: "2222222222"
    open-cluster-management.io/aws-iam-hub-eks-cluster: hub
    open-cluster-management.io/aws-iam-hub-region: us-west-2
    open-cluster-management.io/aws-iam-hub-role: arn:aws:iam::2222222222:role/ocm.remote-worker.worker-0
    open-cluster-management.io/aws-iam-worker-role: arn:aws:iam::1111111111:role/ocm.worker.worker-0
```

The registration agent reads this using its bootstrap token, and continues to generate its credentials 