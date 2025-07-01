# SOCI on Amazon Elastic Kubernetes Service (EKS)

## Overview

The [SOCI on Kubernetes](./kubernetes.md) documentation explains how to configure SOCI to work with Kubernetes; however, translating that documentation into a working setup can be challenging. This doc bridges that gap by offering a copy-pasteable walkthrough for setting up a new Amazon EKS cluster that launches containers with SOCI.

This guide will create a launch template to configure SOCI on EC2 instances, an EKS cluster using the defaults from [eksctl](https://eksctl.io), and a managed node group that uses the launch template to create nodes. For the sake of simplicity, we will confirm that everything works by launching a pre-indexed tensorflow image (public.ecr.aws/soci-workshop-examples/tensorflow_gpu:latest) which sleeps forever.

In this guide, we chose to use the [CRI Credentials](./registry-authentication.md#kubernetes-cri-credentials) mechanism for getting private registry credentials to the SOCI snapshotter. Please consult [the documentation](./registry-authentication.md) to confirm that this will work for your use case if you plan to use this setup beyond this example.

> **Note**
>
> SOCI on Kubernetes has a few rough edges that you should consider before
> using it for important workloads. See the Kubernetes documentation [Limitations](./kubernetes.md#limitations).  
> We welcome your feedback and suggestions for improving the Kubernetes experience.

## Prerequisites

This guide requires the following tools. Please follow the links to find installation instructions.

1. **[eksctl](https://eksctl.io/installation/)** - used for creating an EKS cluster and managed nodegroup
1. **[kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl)** - used for creating a deployment on the EKS cluster
1. **[aws CLI](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html) with credentials configured** - used to create launch template for use in the EKS managed nodegroup

## Setup
### Step 1: Configuration

First, we will set some configuration variables. You can update these to match your preferred settings with the following conditions:

`ARCH` can be x86_64 or arm64  
`INSTANCE_TYPE` should match the architecture (e.g. `t4g.large` for `arm64`)  
`AMI_ID` should be an AL2023-based [EKS optimized AMI](https://docs.aws.amazon.com/eks/latest/userguide/eks-optimized-ami.html). This setup relies on [`nodeadm`](https://awslabs.github.io/amazon-eks-ami/nodeadm/) to configure containerd and the Kublet which is not available in AL2-based AMIs.  

```
AWS_REGION=us-west-2
CLUSTER_NAME=soci
KUBERNETES_VERSION=1.30
ARCH=x86_64
INSTANCE_TYPE=t3.large
AMI_ID=$(aws ssm get-parameter --name /aws/service/eks/optimized-ami/${KUBERNETES_VERSION}/amazon-linux-2023/${ARCH}/standard/recommended/image_id  --region $AWS_REGION --query "Parameter.Value" --output text)
```

### Step 2: Create an EKS cluster 

Next we will create a cluster using eksctl. This step will create a cluster and the necessary resources for EKS to function, but importantly will not create any nodegroups. We will create nodegroups later using a custom launch template that installs and configures the SOCI snapshotter.

```
eksctl create cluster \
  --without-nodegroup \
  --name $CLUSTER_NAME \
  --version $KUBERNETES_VERSION \
  --region $AWS_REGION
```

### Step 3: Create Node configuration file

This config will allow our node to join the cluster with updated containerd/kubelet config to support SOCI. 

First, we'll use the AWS CLI to get some properties from EKS. These will be necessary for our nodes to join the cluster.

```
CLUSTER_ENDPOINT=$(aws eks describe-cluster \
  --name $CLUSTER_NAME \
  --region $AWS_REGION \
  --output text \
  --query 'cluster.endpoint')
CLUSTER_CERTIFICATE_AUTHORITY=$(aws eks describe-cluster \
  --name $CLUSTER_NAME --region $AWS_REGION \
  --output text \
  --query 'cluster.certificateAuthority.data')
CLUSTER_CIDR=$(aws eks describe-cluster \
  --name $CLUSTER_NAME \
  --region $AWS_REGION \
  --output text \
  --query 'cluster.kubernetesNetworkConfig.serviceIpv4Cidr')
```

In order to avoid issues later, let's verify that each variable is set:

```
echo "$CLUSTER_ENDPOINT"
echo "$CLUSTER_CERTIFICATE_AUTHORITY"
echo "$CLUSTER_CIDR"
```

If everything worked, you should see an output like:
```
https://example.com
Y2VydGlmaWNhdGVBdXRob3JpdHk=
10.100.0.0/16
```

If any are empty, please re-run the AWS CLI commands. Issues here will prevent nodes from joining the cluster and result in rollback of the managed node group.


Next we will write this as a EKS optimized AMI nodeadm config. This configuration also tells containerd and kubelet to use SOCI.

```
cat <<EOF > node_config.yaml
apiVersion: node.eks.aws/v1alpha1
kind: NodeConfig
spec:
  cluster:
    name: $CLUSTER_NAME
    apiServerEndpoint: $CLUSTER_ENDPOINT
    certificateAuthority: $CLUSTER_CERTIFICATE_AUTHORITY
    cidr: $CLUSTER_CIDR
  kubelet:
    config:
      imageServiceEndpoint: unix:///run/soci-snapshotter-grpc/soci-snapshotter-grpc.sock
  containerd: 
    config: |
      [proxy_plugins.soci]
        type = "snapshot"
        address = "/run/soci-snapshotter-grpc/soci-snapshotter-grpc.sock"
        [proxy_plugins.soci.exports]
          root = "/var/lib/soci-snapshotter-grpc"
      [plugins."io.containerd.grpc.v1.cri".containerd]
        snapshotter = "soci"
        # This line is required for containerd to send information about how to lazily load the image to the snapshotter
        disable_snapshot_annotations = false
EOF
```


### Step 4: Create SOCI install script

This will create an script that will be run on instance boot to install and configure the SOCI snapshotter.

```
cat <<'EOF_SCRIPT' >install_soci.sh
#!/bin/bash
# Set environment variables
ARCH=$(uname -m | sed s/aarch64/arm64/ | sed s/x86_64/amd64/)
version="0.11.1"
ARCHIVE=soci-snapshotter-$version-linux-$ARCH.tar.gz

pushd /tmp
# Download, verify, and install the soci-snapshotter
curl --silent --location --fail --output $ARCHIVE https://github.com/awslabs/soci-snapshotter/releases/download/v$version/$ARCHIVE
curl --silent --location --fail --output $ARCHIVE.sha256sum https://github.com/awslabs/soci-snapshotter/releases/download/v$version/$ARCHIVE.sha256sum
sha256sum ./$ARCHIVE.sha256sum
tar xzvf ./$ARCHIVE -C /usr/local/bin soci-snapshotter-grpc
rm ./$ARCHIVE
rm ./$ARCHIVE.sha256sum

# Configure the SOCI snapshotter for CRI credentials
mkdir -p /etc/soci-snapshotter-grpc
cat <<EOF >/etc/soci-snapshotter-grpc/config.toml
[cri_keychain]
# This tells the soci-snapshotter to act as a proxy ImageService
# and to cache credentials from requests to pull images.
enable_keychain = true
# This tells the soci-snapshotter where containerd's ImageService is located.
image_service_path = "/run/containerd/containerd.sock"
EOF

# Start the soci-snapshotter
curl --silent --location --fail --output /etc/systemd/system/soci-snapshotter.service https://raw.githubusercontent.com/awslabs/soci-snapshotter/v$version/soci-snapshotter.service
systemctl daemon-reload
systemctl enable --now soci-snapshotter

popd
EOF_SCRIPT
```


### Step 5: Create EC2 Userdata

This step will combine the node configuration and SOCI installation script from the previous steps into a MIME multipart archive that works as EC2 user data. 


```
cat <<EOF > userdata.txt
MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="BOUNDARY"

--BOUNDARY
Content-Type: application/node.eks.aws

---
$(cat node_config.yaml)

--BOUNDARY
Content-Type: text/x-shellscript; charset="us-ascii"

$(cat install_soci.sh)

--BOUNDARY--
EOF
```

### Step 6: Create a node launch template

Next, we will create a launch template that uses our chosen AMI, instance type, and userdata. This will be used to launch new nodes for our EKS cluster.

```
cat <<EOF > launch_template_data.json
{
  "ImageId": "${AMI_ID}",
  "InstanceType": "${INSTANCE_TYPE}",
  "UserData": "$(cat userdata.txt | base64 --wrap=0)"
}
EOF

aws ec2 create-launch-template \
  --launch-template-name soci-eks-node \
  --launch-template-data file://launch_template_data.json \
  --region $AWS_REGION
```

After creating the launch template, we can retrieve it's ID into a variable.

```
LAUNCH_TEMPLATE_ID=$(aws ec2 describe-launch-templates \
  --launch-template-name soci-eks-node \
  --region $AWS_REGION \
  --query "LaunchTemplates[0].LaunchTemplateId" \
  --output text)
```

### Step 7: Create an EKS managed nodegroup

As the final setup for our cluster, we will use the launch template to create a managed nodegroup. After this step, we will have nodes in our cluster configured to launch containers with the SOCI snapshotter.

```
eksctl create nodegroup -f - <<EOF
---
apiVersion: eksctl.io/v1alpha5
kind: ClusterConfig

metadata:
  name: "$CLUSTER_NAME"
  region: "$AWS_REGION"

managedNodeGroups:
  - name: "${CLUSTER_NAME}-ng-1"
    launchTemplate:
      id: "$LAUNCH_TEMPLATE_ID"
EOF
```

## Launching a Pod
### Step 1: Configure kubectl

Here, we update `kubeconfig` so that `kubectl` can communicate with our EKS cluster

```
aws eks update-kubeconfig --region $AWS_REGION --name "$CLUSTER_NAME"
```

### Step 2: Create a deployment

And finally we can create a deployment using SOCI. 

For this demo, we will use the a pre-indexed tensorflow-gpu image (~3GB) that will sleep forever. This represents the best case scenario for SOCI where approximately none of the image is needed to start the container. 

```
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: soci-sample-deployment
  labels:
    app: soci
spec:
  replicas: 1
  selector:
    matchLabels:
      app: soci
  template:
    metadata:
      labels:
        app: soci
    spec:
      containers:
      - name: soci-container
        image: public.ecr.aws/soci-workshop-examples/tensorflow_gpu:latest
        command: ["sleep"]
        args: ["inf"]
EOF
```

## Verification

Containerd snapshotters are not directly visible to Kubernetes so they do not appear in any UI. As an approximation, we can use the shortened image pull time to infer that SOCI was used. To be sure, we can inspect the node to verify that the SOCI filesystems were created. 

For this, we will look up the node where the pod was scheduled, map that to an EC2 instance, and use AWS Systems Manager (SSM) to look up the SOCI filesystems with `findmnt`

```
NODE_NAME=$(kubectl get pods --selector app=soci --output jsonpath="{.items[0].spec.nodeName}")
INSTANCE_ID=$(aws ec2 describe-instances \
  --region $AWS_REGION \
  --filter "Name=private-dns-name,Values=$NODE_NAME" \
  --query "Reservations[0].Instances[0].InstanceId" \
  --output text)
SSM_COMMAND_ID=$(aws ssm send-command \
    --instance-ids $INSTANCE_ID \
    --document-name "AWS-RunShellScript" \
    --comment "Get SOCI mounts" \
    --parameters commands='findmnt --source soci' \
    --query "Command.CommandId"\
    --output text \
    --region $AWS_REGION)
aws ssm list-command-invocations \
    --command-id $SSM_COMMAND_ID \
    --region $AWS_REGION \
    --details \
    --query "CommandInvocations[*].CommandPlugins[*].Output" \
    --output text
```

If everything worked, we should expect an output like:
```
TARGET                                                     SOURCE FSTYPE         OPTIONS
/var/lib/soci-snapshotter-grpc/snapshotter/snapshots/27/fs soci   fuse.rawBridge rw,nosuid,nodev,relatime,user_id=0,group_id=0,allow_other,max_read=131072
/var/lib/soci-snapshotter-grpc/snapshotter/snapshots/28/fs soci   fuse.rawBridge rw,nosuid,nodev,relatime,user_id=0,group_id=0,allow_other,max_read=131072
/var/lib/soci-snapshotter-grpc/snapshotter/snapshots/29/fs soci   fuse.rawBridge rw,nosuid,nodev,relatime,user_id=0,group_id=0,allow_other,max_read=131072
/var/lib/soci-snapshotter-grpc/snapshotter/snapshots/30/fs soci   fuse.rawBridge rw,nosuid,nodev,relatime,user_id=0,group_id=0,allow_other,max_read=131072
/var/lib/soci-snapshotter-grpc/snapshotter/snapshots/31/fs soci   fuse.rawBridge rw,nosuid,nodev,relatime,user_id=0,group_id=0,allow_other,max_read=131072
/var/lib/soci-snapshotter-grpc/snapshotter/snapshots/32/fs soci   fuse.rawBridge rw,nosuid,nodev,relatime,user_id=0,group_id=0,allow_other,max_read=131072
/var/lib/soci-snapshotter-grpc/snapshotter/snapshots/33/fs soci   fuse.rawBridge rw,nosuid,nodev,relatime,user_id=0,group_id=0,allow_other,max_read=131072
/var/lib/soci-snapshotter-grpc/snapshotter/snapshots/34/fs soci   fuse.rawBridge rw,nosuid,nodev,relatime,user_id=0,group_id=0,allow_other,max_read=131072
/var/lib/soci-snapshotter-grpc/snapshotter/snapshots/35/fs soci   fuse.rawBridge rw,nosuid,nodev,relatime,user_id=0,group_id=0,allow_other,max_read=131072
```

##  Clean up (Optional)

Run these clean up commands to delete the files, Amazon EKS cluster, and launch template created by this guide.

```
rm launch_template_data.json userdata.txt install_soci.sh node_config.yaml
eksctl delete cluster --name $CLUSTER_NAME --disable-nodegroup-eviction --region $AWS_REGION
aws ec2 delete-launch-template --launch-template-id $LAUNCH_TEMPLATE_ID --region $AWS_REGION
```

## Next Steps

This guide showed you how to set up SOCI on an Amazon EKS cluster. Check out the following documentation to get a better understanding of the components of this setup and to learn how to index your own container images.

1. [SOCI on Kubernetes Documentation](./kubernetes.md) - general SOCI on Kubernetes information
1. [Registry Authentication](./registry-authentication.md) - trade-offs for each authentication mechanism
1. [Getting Started](./getting-started.md) - instructions for indexing your own images
