package cluster

import (
	"encoding/json"
	"fmt"

	"github.com/maxnovawindunix/pulumi-eks/pkg/iam"
	"github.com/maxnovawindunix/pulumi-eks/pkg/vpc"
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/eks"
	eksv2 "github.com/pulumi/pulumi-eks/sdk/go/eks"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

var (
	managedPolicyArns = []string{
		"arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy",
		"arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy",
		"arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly",
	}
)

type ClusterComponentArgs struct {
	Name     pulumi.String     `pulumi:"name"`
	Network  *vpc.VpcComponent `pulumi:"network"`
	NodeGrps []NodeGrpArgs     `pulumi:"nodeGrps"`
}

type NodeGrpArgs struct {
	Name          pulumi.String              `pulumi:"name"`
	CapacityType  pulumi.String              `pulumi:"capacityType"`
	Scaling       eks.NodeGroupScalingConfig `pulumi:"scaling"`
	Taints        []eks.NodeGroupTaint       `pulumi:"taints"`
	InstanceTypes []string                   `pulumi:"instanceType"`
}

type ClusterComponent struct {
	pulumi.ResourceState

	Kubeconfig pulumi.AnyOutput `pulumi:"kubeconfig"`
}

func defaultAssumeRolePolicy() []byte {
	baseIamJSON, _ := json.Marshal(map[string]interface{}{
		"Version": "2012-10-17",
		"Statement": []map[string]interface{}{
			{
				"Action": "sts:AssumeRole",
				"Effect": "Allow",
				"Sid":    "",
				"Principal": map[string]interface{}{
					"Service": "ec2.amazonaws.com",
				},
			},
		},
	})
	return baseIamJSON
}

func NewClusterComponent(ctx *pulumi.Context, name string, args ClusterComponentArgs, opts ...pulumi.ResourceOption) (*ClusterComponent, error) {
	var clusterComponent ClusterComponent

	err := ctx.RegisterComponentResource("pulumi-eks:pkg/cluster:cluster", name, &clusterComponent, opts...)
	if err != nil {
		return nil, err
	}
	iamListRole := []iam.ApplyRoles{}

	// Create IamListRole for EKS cluster
	for _, nodeArgs := range args.NodeGrps {
		iamListRole = append(iamListRole, iam.NewCustomRole(string(nodeArgs.Name)))
	}

	iamNodeRoles, err := iam.NewIamComponent(ctx, fmt.Sprintf("%s-cluster", name),
		[]iam.BaseOpts{
			iam.WithCustomPolicies(managedPolicyArns),
			iam.WithAssumeRolepolicy(defaultAssumeRolePolicy())}, iamListRole, pulumi.Parent(&clusterComponent))

	if err != nil {
		return nil, err
	}
	cluster, err := eksv2.NewCluster(ctx, name, &eksv2.ClusterArgs{
		VpcId:                        args.Network.ID(),
		PublicSubnetIds:              args.Network.GetPublicSubnetIDs(),
		PrivateSubnetIds:             args.Network.GetPrivateSubnetIDs(),
		NodeAssociatePublicIpAddress: pulumi.BoolRef(false),
		SkipDefaultNodeGroup:         pulumi.BoolRef(true),
		EndpointPrivateAccess:        pulumi.Bool(true),
		EndpointPublicAccess:         pulumi.Bool(true),
		InstanceRoles:                iamNodeRoles.GetAllRoles(),
	}, pulumi.Parent(&clusterComponent))

	if err != nil {
		return nil, err
	}

	for _, nodeArgs := range args.NodeGrps {
		// need to convert to slice of interface
		taints := make([]interface{}, len(nodeArgs.Taints))
		for i, taint := range nodeArgs.Taints {
			taints[i] = taint
		}

		_, err = eksv2.NewManagedNodeGroup(ctx, string(nodeArgs.Name), &eksv2.ManagedNodeGroupArgs{
			Cluster:            cluster,
			CapacityType:       nodeArgs.CapacityType,
			InstanceTypes:      pulumi.ToStringArray(nodeArgs.InstanceTypes),
			ForceUpdateVersion: pulumi.Bool(true),
			NodeRoleArn:        iamNodeRoles.GetRole(string(nodeArgs.Name)).Arn,
			ScalingConfig: pulumi.All(nodeArgs.Scaling.DesiredSize, nodeArgs.Scaling.MinSize, nodeArgs.Scaling.MaxSize).ApplyT(func(args []interface{}) (*eks.NodeGroupScalingConfig, error) {
				return &eks.NodeGroupScalingConfig{
					DesiredSize: args[0].(int),
					MinSize:     args[1].(int),
					MaxSize:     args[2].(int),
				}, nil
			}).(eks.NodeGroupScalingConfigPtrInput),
			Taints: pulumi.All(taints...).ApplyT(func(taints []interface{}) eks.NodeGroupTaintArray {
				var nodeTaints eks.NodeGroupTaintArray
				if len(taints) == 0 {
					return nil
				}
				for _, taint := range taints {
					nodeTaints = append(nodeTaints, eks.NodeGroupTaintArgs{
						Effect: pulumi.String(taint.(eks.NodeGroupTaint).Effect),
						Key:    pulumi.String(taint.(eks.NodeGroupTaint).Key),
						Value:  pulumi.StringPtr(*taint.(eks.NodeGroupTaint).Value),
					})
				}
				return nodeTaints
			}).(eks.NodeGroupTaintArrayInput),
			SubnetIds: args.Network.GetPrivateSubnetIDs(),
		}, pulumi.Parent(&clusterComponent))
		if err != nil {
			return nil, err
		}
	}
	clusterComponent.Kubeconfig = cluster.Kubeconfig

	ctx.RegisterResourceOutputs(&clusterComponent, pulumi.Map{
		"kubeconfig": clusterComponent.Kubeconfig,
	})

	return &clusterComponent, nil
}
