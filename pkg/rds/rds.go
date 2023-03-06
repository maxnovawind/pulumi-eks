package rds

import (
	"github.com/maxnovawindunix/pulumi-eks/pkg/vpc"
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/ec2"
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/rds"
	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type RdsComponentArgs struct {
	Enable        bool          `pulumi:"enable"`
	Port          pulumi.Int    `pulumi:"port"`
	Region        pulumi.String `pulumi:"region"`
	Name          pulumi.String `pulumi:"name"`
	InstanceType  pulumi.String `pulumi:"instanceType"`
	DbName        pulumi.String `pulumi:"dbName"`
	DbUsername    pulumi.String `pulumi:"dbUsername"`
	Engine        pulumi.String `pulumi:"engine"`
	EngineVersion pulumi.String `pulumi:"engineVersion"`
	StorageSize   pulumi.Int    `pulumi:"storageSize"`
	StorageType   pulumi.String `pulumi:"storageType"`
	Iops          pulumi.Int    `pulumi:"iops"`
	VpcNetwork    *vpc.VpcComponent
}

type RdsComponent struct {
	pulumi.ResourceState
	RdsEndpoints pulumi.StringMap
}

type RdsEndpoints struct {
	DSN            pulumi.StringOutput `pulumi:"dsn"`
	ReaderEndpoint pulumi.StringOutput `pulumi:"readerEndpoint"`
}

func NewRdsComponent(ctx *pulumi.Context, name string, args *RdsComponentArgs, opts ...pulumi.ResourceOption) (*RdsComponent, error) {
	var rdsComponent RdsComponent

	err := ctx.RegisterComponentResource("pulumi-eks:pkg/rds:rds", name, &rdsComponent, opts...)
	if err != nil {
		return nil, err
	}

	password, err := random.NewRandomPassword(ctx, "password", &random.RandomPasswordArgs{
		Length:  pulumi.Int(16),
		Special: pulumi.Bool(false),
	}, pulumi.Parent(&rdsComponent))
	if err != nil {
		return nil, err
	}

	rdsSecurityGrp, err := ec2.NewSecurityGroup(ctx, name, &ec2.SecurityGroupArgs{
		Description: pulumi.String("Allow From EKS to RDS"),
		VpcId:       args.VpcNetwork.ID(),
		Ingress: ec2.SecurityGroupIngressArray{
			&ec2.SecurityGroupIngressArgs{
				Description: pulumi.String("RDS from VPC"),
				FromPort:    args.Port,
				ToPort:      args.Port,
				Protocol:    pulumi.String("tcp"),
				CidrBlocks: args.VpcNetwork.GetPrivateSubnetIDs().ApplyT(func(strArray []string) pulumi.StringArray {
					var privateCidrBlocks pulumi.StringArray
					for _, str := range strArray {
						result, _ := ec2.LookupSubnet(ctx, &ec2.LookupSubnetArgs{
							Id: pulumi.StringRef(str),
						})
						privateCidrBlocks = append(privateCidrBlocks, pulumi.String(result.CidrBlock))
					}
					return privateCidrBlocks
				}).(pulumi.StringArrayInput),
			},
		},
		Egress: ec2.SecurityGroupEgressArray{
			&ec2.SecurityGroupEgressArgs{
				FromPort: pulumi.Int(0),
				ToPort:   pulumi.Int(0),
				Protocol: pulumi.String("-1"),
				CidrBlocks: pulumi.StringArray{
					pulumi.String("0.0.0.0/0"),
				},
				Ipv6CidrBlocks: pulumi.StringArray{
					pulumi.String("::/0"),
				},
			},
		},
		Tags: pulumi.StringMap{
			"Name": pulumi.String(name),
		},
	}, pulumi.Parent(&rdsComponent))
	if err != nil {
		return nil, err
	}
	subnetGroup, err := rds.NewSubnetGroup(ctx, "subnet-group-rds", &rds.SubnetGroupArgs{
		Description: pulumi.String("Subnet Group"),
		SubnetIds:   args.VpcNetwork.GetIsolatedSubnetIDs(),
	}, pulumi.Parent(&rdsComponent))
	if err != nil {
		return nil, err
	}
	_default, err := rds.NewCluster(ctx, name, &rds.ClusterArgs{
		ClusterIdentifier:      pulumi.String(name),
		Engine:                 args.Engine,
		EngineVersion:          args.EngineVersion,
		DbClusterInstanceClass: args.InstanceType,
		DbSubnetGroupName:      subnetGroup.ID(),
		AvailabilityZones: pulumi.StringArray{
			pulumi.Sprintf("%sa", args.Region),
			pulumi.Sprintf("%sb", args.Region),
			pulumi.Sprintf("%sc", args.Region),
		},
		AllocatedStorage:  args.StorageSize,
		StorageType:       pulumi.String(args.StorageType),
		Iops:              pulumi.Int(args.Iops),
		DatabaseName:      args.DbName,
		MasterUsername:    args.DbUsername,
		MasterPassword:    password.Result,
		SkipFinalSnapshot: pulumi.Bool(true),
		VpcSecurityGroupIds: pulumi.StringArray{
			rdsSecurityGrp.ID(),
		},
	}, pulumi.Parent(&rdsComponent))
	if err != nil {
		return nil, err
	}

	rdsComponent.RdsEndpoints = pulumi.StringMap{
		"DSN with read/write endpoint": pulumi.Sprintf("%s://%s:%s@%s:%d/%s", args.Engine, _default.MasterUsername, password.Result, _default.Endpoint, _default.Port, _default.DatabaseName),
		"Reader Endpoint":              _default.ReaderEndpoint,
	}
	ctx.RegisterResourceOutputs(&rdsComponent, pulumi.Map{
		"Database": rdsComponent.RdsEndpoints,
	})
	return &rdsComponent, err
}
