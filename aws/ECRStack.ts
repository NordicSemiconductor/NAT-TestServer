import * as CloudFormation from '@aws-cdk/core'
import * as ECR from '@aws-cdk/aws-ecr'

export enum Outputs {
	cdEcrRepositoryArn = 'cdEcrRepositoryArn',
}

export class ECRStack extends CloudFormation.Stack {
	public constructor(parent: CloudFormation.App, id: string) {
		super(parent, id)

		const ecr = new ECR.Repository(this, 'Repository')

		new CloudFormation.CfnOutput(this, 'cdEcrRepositoryName', {
			value: ecr.repositoryName,
			exportName: `${this.stackName}:cdEcrRepositoryName`,
		})

		new CloudFormation.CfnOutput(this, Outputs.cdEcrRepositoryArn, {
			value: ecr.repositoryArn,
			exportName: `${this.stackName}:${Outputs.cdEcrRepositoryArn}`,
		})

		new CloudFormation.CfnOutput(this, 'cdEcrRepositoryUri', {
			value: ecr.repositoryUri,
			exportName: `${this.stackName}:cdEcrRepositoryUri`,
		})
	}
}
