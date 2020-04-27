import * as CloudFormation from '@aws-cdk/core'
import * as IAM from '@aws-cdk/aws-iam'
import * as ECR from '@aws-cdk/aws-ecr'

export class CD extends CloudFormation.Resource {
	public readonly ecr: ECR.IRepository
	public readonly accessKey: IAM.CfnAccessKey

	public constructor(parent: CloudFormation.Stack, id: string) {
		super(parent, id)

		this.ecr = new ECR.Repository(this, 'Repository')

		const user = new IAM.User(this, 'user', {
			userName: `${parent.stackName}-cd`,
		})

		user.addToPolicy(
			new IAM.PolicyStatement({
				resources: [this.ecr.repositoryArn, `${this.ecr.repositoryArn}/*`],
				actions: ['ecr:*'],
			}),
		)

		this.accessKey = new IAM.CfnAccessKey(this, 'cdAccessKey', {
			userName: user.userName,
			status: 'Active',
		})
	}
}
