import * as CloudFormation from '@aws-cdk/core'
import * as IAM from '@aws-cdk/aws-iam'
import * as S3 from '@aws-cdk/aws-s3'
import { CD } from './CD'

export class NATTestResourcesStack extends CloudFormation.Stack {
	public constructor(parent: CloudFormation.App, id: string) {
		super(parent, id)

		const bucket = new S3.Bucket(this, 'bucket', {
			removalPolicy: CloudFormation.RemovalPolicy.RETAIN,
		})

		const user = new IAM.User(this, 'user', {
			userName: 'nat-test-server-logs',
		})

		user.addToPolicy(
			new IAM.PolicyStatement({
				resources: [bucket.bucketArn, `${bucket.bucketArn}/*`],
				actions: ['s3:PutObject', 's3:GetObject', 's3:ListBucket'],
			}),
		)

		const accessKey = new IAM.CfnAccessKey(this, 'userAccessKey', {
			userName: user.userName,
			status: 'Active',
		})

		new CloudFormation.CfnOutput(this, 'bucketName', {
			value: bucket.bucketName,
			exportName: `${this.stackName}:bucketName`,
		})

		new CloudFormation.CfnOutput(this, 'userAccessKeyId', {
			value: accessKey.ref,
			exportName: `${this.stackName}:userAccessKeyId`,
		})

		new CloudFormation.CfnOutput(this, 'userSecretAccessKey', {
			value: accessKey.attrSecretAccessKey,
			exportName: `${this.stackName}:userSecretAccessKey`,
		})

		// Continuous deployment

		const cd = new CD(this, 'CD', {
			bucket,
			userAccessKey: accessKey,
		})

		new CloudFormation.CfnOutput(this, 'cdAccessKeyId', {
			value: cd.accessKey.ref,
			exportName: `${this.stackName}:cdAccessKeyId`,
		})

		new CloudFormation.CfnOutput(this, 'cdSecretAccessKey', {
			value: cd.accessKey.getAtt('SecretAccessKey').toString(),
			exportName: `${this.stackName}:cdSecretAccessKey`,
		})

		new CloudFormation.CfnOutput(this, 'cdEcrRepositoryName', {
			value: cd.ecr.repositoryName,
			exportName: `${this.stackName}:cdEcrRepositoryName`,
		})

		new CloudFormation.CfnOutput(this, 'cdEcrRepositoryUri', {
			value: cd.ecr.repositoryUri,
			exportName: `${this.stackName}:cdEcrRepositoryUri`,
		})

		new CloudFormation.CfnOutput(this, 'fargateArn', {
			value: cd.fargate.serviceArn,
			exportName: `${this.stackName}:fargateArn`,
		})

		new CloudFormation.CfnOutput(this, 'clusterArn', {
			value: cd.cluster.clusterArn,
			exportName: `${this.stackName}:clusterArn`,
		})
	}
}
