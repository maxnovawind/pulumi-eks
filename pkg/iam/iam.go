package iam

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/iam"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type IamComponent struct {
	pulumi.ResourceState

	// propagate pulumi context
	context *pulumi.Context

	// opts
	managedPolicyArns []string
	assumeRolePolicy  []byte

	// output of iamComponent
	mapNodeRole map[string]*iam.Role
}

type BaseOpts func(*IamComponent)

type ApplyRoles func(*IamComponent) error

func NewIamComponent(ctx *pulumi.Context, name string, iamOpts []BaseOpts, applyRoles []ApplyRoles, opts ...pulumi.ResourceOption) (*IamComponent, error) {
	resource := &IamComponent{context: ctx, mapNodeRole: make(map[string]*iam.Role)}
	err := ctx.RegisterComponentResource("pulumi-eks:pkg/iam:iam", name, resource, opts...)
	if err != nil {
		return nil, err
	}

	// Apply opts
	for _, opt := range iamOpts {
		opt(resource)
	}

	// Apply roles
	for _, applyRole := range applyRoles {
		err := applyRole(resource)
		if err != nil {
			return nil, err
		}
	}
	return resource, err
}

func NewCustomRole(customRole string) func(*IamComponent) error {
	return func(iamComp *IamComponent) error {
		json0 := string(iamComp.assumeRolePolicy)
		nodeRole, err := iam.NewRole(iamComp.context, fmt.Sprintf("%s-role", customRole), &iam.RoleArgs{
			AssumeRolePolicy:  pulumi.String(json0),
			ManagedPolicyArns: pulumi.ToStringArray(iamComp.managedPolicyArns),
		})
		iamComp.mapNodeRole[customRole] = nodeRole
		return err
	}
}
func WithCustomPolicies(customPolicies []string) func(*IamComponent) {
	return func(iamComp *IamComponent) {
		iamComp.managedPolicyArns = append(iamComp.managedPolicyArns, customPolicies...)
	}
}

func WithAssumeRolepolicy(dataAssumeRolePolicy []byte) func(*IamComponent) {
	return func(iamComp *IamComponent) {
		iamComp.assumeRolePolicy = dataAssumeRolePolicy
	}
}

func (iamComp *IamComponent) GetRole(name string) *iam.Role {
	return iamComp.mapNodeRole[name]
}

func (iamComp *IamComponent) GetAllRoles() iam.RoleArray {
	var result iam.RoleArray
	for _, role := range iamComp.mapNodeRole {
		result = append(result, role)
	}
	return result
}
