package vpc

import (
	ec2Ext "github.com/pulumi/pulumi-awsx/sdk/go/awsx/ec2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type VpcComponentArgs struct {
	Name      string      `pulumi:"name"`
	CidrBlock string      `pulumi:"cidrBlock"`
	Subnets   SubnetsArgs `pulumi:"subnets"`
}

type SubnetsArgs struct {
	PublicSubnets   SubnetArgs `pulumi:"publicSubnets"`
	PrivateSubnets  SubnetArgs `pulumi:"privateSubnets"`
	IsolatedSubnets SubnetArgs `pulumi:"isolatedSubnets"`
}

type SubnetArgs struct {
	CidrMask int `pulumi:"cidrMask"`
}

type VpcComponent struct {
	pulumi.ResourceState

	vpcID             pulumi.StringOutput      `pulumi:"vpcID"`
	publicSubnetIDs   pulumi.StringArrayOutput `pulumi:"publicSubnetIDs"`
	privateSubnetIDs  pulumi.StringArrayOutput `pulumi:"privateSubnetIDs"`
	isolatedSubnetIDs pulumi.StringArrayOutput `pulumi:"isolatedSubnetIDs"`
	cidrBlock         pulumi.StringOutput      `pulumi:"cidrBlock"`
}

func NewVpcComponent(ctx *pulumi.Context, vpcComponentArgs *VpcComponentArgs, opts ...pulumi.ResourceOption) (*VpcComponent, error) {
	vpcComponent := &VpcComponent{}
	err := ctx.RegisterComponentResource("pulumi-eks:pkg/vpc:vpc", vpcComponentArgs.Name, vpcComponent, opts...)
	if err != nil {
		return nil, err
	}

	network, err := ec2Ext.NewVpc(ctx, vpcComponentArgs.Name, &ec2Ext.VpcArgs{
		CidrBlock:                 pulumi.StringRef(vpcComponentArgs.CidrBlock),
		NumberOfAvailabilityZones: pulumi.IntRef(3),
		SubnetSpecs: []ec2Ext.SubnetSpecArgs{
			{
				CidrMask: pulumi.IntRef(vpcComponentArgs.Subnets.PublicSubnets.CidrMask),
				Name:     pulumi.StringRef("public"),
				Tags: pulumi.ToStringMap(map[string]string{
					"network-tag": "public",
				}),
				Type: ec2Ext.SubnetTypePublic,
			},
			{
				CidrMask: pulumi.IntRef(vpcComponentArgs.Subnets.PrivateSubnets.CidrMask),
				Name:     pulumi.StringRef("private"),
				Tags: pulumi.ToStringMap(map[string]string{
					"network-tag": "private",
				}),
				Type: ec2Ext.SubnetTypePrivate,
			},
			{
				CidrMask: pulumi.IntRef(vpcComponentArgs.Subnets.IsolatedSubnets.CidrMask),
				Name:     pulumi.StringRef("isolated"),
				Tags: pulumi.ToStringMap(map[string]string{
					"network-tag": "isolated",
				}),
				Type: ec2Ext.SubnetTypeIsolated,
			},
		},
	}, pulumi.Parent(vpcComponent))
	vpcComponent.isolatedSubnetIDs = network.IsolatedSubnetIds
	vpcComponent.publicSubnetIDs = network.PublicSubnetIds
	vpcComponent.privateSubnetIDs = network.PrivateSubnetIds
	vpcComponent.cidrBlock = network.Vpc.CidrBlock()
	vpcComponent.vpcID = network.VpcId
	ctx.RegisterResourceOutputs(vpcComponent, pulumi.Map{
		"PublicSubnetIDs":   vpcComponent.publicSubnetIDs,
		"PrivateSubnetIDs":  vpcComponent.privateSubnetIDs,
		"IsolatedSubnetIDs": vpcComponent.isolatedSubnetIDs,
	})
	return vpcComponent, err
}

func (vpc *VpcComponent) ID() pulumi.StringOutput {
	return vpc.vpcID
}

func (vpc *VpcComponent) GetPublicSubnetIDs() pulumi.StringArrayOutput {
	return vpc.publicSubnetIDs
}

func (vpc *VpcComponent) GetIsolatedSubnetIDs() pulumi.StringArrayOutput {
	return vpc.isolatedSubnetIDs
}

func (vpc *VpcComponent) GetPrivateSubnetIDs() pulumi.StringArrayOutput {
	return vpc.privateSubnetIDs
}

func (vpc *VpcComponent) GetCidrBlock() pulumi.StringOutput {
	return vpc.cidrBlock
}
