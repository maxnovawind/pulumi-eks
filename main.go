package main

import (
	"github.com/maxnovawind/pulumi-eks/pkg/cluster"
	"github.com/maxnovawind/pulumi-eks/pkg/rds"
	"github.com/maxnovawind/pulumi-eks/pkg/vpc"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		var (
			confDatabase rds.RdsComponentArgs
			confCluster  cluster.ClusterComponentArgs
			confNetwork  vpc.VpcComponentArgs
		)
		// GetConfig
		cfg := config.New(ctx, "")
		cfg.RequireObject("cluster", &confCluster)
		err := cfg.TryObject("network", &confNetwork)
		if err != nil {
			return err
		}
		cfg.RequireObject("database", &confDatabase)
		awscfg := config.New(ctx, "aws")
		region := awscfg.Require("region")

		network, err := vpc.NewVpcComponent(ctx, &confNetwork)
		if err != nil {
			return err
		}
		confCluster.Network = network
		cluster, err := cluster.NewClusterComponent(ctx, string(confCluster.Name), confCluster)
		if err != nil {
			return err
		}

		if confDatabase.Enable {
			confDatabase.VpcNetwork = network
			confDatabase.Region = pulumi.String(region)
			database, err := rds.NewRdsComponent(ctx, string(confDatabase.Name), &confDatabase)
			if err != nil {
				return err
			}

			ctx.Export("Database", database.RdsEndpoints)
		}
		ctx.Export("cluster", cluster.Kubeconfig)
		return err
	})
}
