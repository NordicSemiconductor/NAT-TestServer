import * as CloudFormation from '@aws-cdk/core'
import * as IAM from '@aws-cdk/aws-iam'
import * as S3 from '@aws-cdk/aws-s3'
import * as ECR from '@aws-cdk/aws-ecr'
import * as CloudWatchLogs from '@aws-cdk/aws-logs'
import * as Events from '@aws-cdk/aws-events'
import * as EventTargets from '@aws-cdk/aws-events-targets'
import * as Lambda from '@aws-cdk/aws-lambda'
import { CD } from './CD'
import { LayeredLambdas } from '@bifravst/package-layered-lambdas'
import { Lambdas } from './prepare-resources'

export class ServerStack extends CloudFormation.Stack {
	public constructor(
		parent: CloudFormation.App,
		id: string,
		args: {
			ecrRepositoryArn: string
			sourceCodeBucketName: string
			baseLayerZipFileName: string
			lambdas: LayeredLambdas<Lambdas>
			updateDNSRoleArn: string
			assumeRoleExternalID: string
			hostedZoneId: string
			recordName: string
		},
	) {
		super(parent, id)

		const sourceCodeBucket = S3.Bucket.fromBucketAttributes(
			this,
			'SourceCodeBucket',
			{
				bucketName: args.sourceCodeBucketName,
			},
		)

		const baseLayer = new Lambda.LayerVersion(this, `${id}-layer`, {
			code: Lambda.Code.bucket(sourceCodeBucket, args.baseLayerZipFileName),
			compatibleRuntimes: [Lambda.Runtime.NODEJS_12_X],
		})

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

		// Concatenate the log files

		const concatenateLogFiles = new Lambda.Function(
			this,
			'concatenateLogFiles',
			{
				layers: [baseLayer],
				handler: 'index.handler',
				runtime: Lambda.Runtime.NODEJS_12_X,
				timeout: CloudFormation.Duration.seconds(900),
				memorySize: 1792,
				code: Lambda.Code.bucket(
					sourceCodeBucket,
					args.lambdas.lambdaZipFileNames.concatenateLogFiles,
				),
				description:
					'Runs every hour and concatenates the raw device messages so it is more performant for Athena to query them.',
				initialPolicy: [
					new IAM.PolicyStatement({
						resources: ['*'],
						actions: [
							'logs:CreateLogGroup',
							'logs:CreateLogStream',
							'logs:PutLogEvents',
						],
					}),
					new IAM.PolicyStatement({
						resources: [bucket.bucketArn, `${bucket.bucketArn}/*`],
						actions: [
							's3:ListBucket',
							's3:GetObject',
							's3:PutObject',
							's3:DeleteObject',
						],
					}),
				],
				environment: {
					BUCKET_NAME: bucket.bucketName,
				},
				reservedConcurrentExecutions: 1,
			},
		)

		new CloudWatchLogs.LogGroup(this, `concatenateLogFilesLogGroup`, {
			removalPolicy: CloudFormation.RemovalPolicy.DESTROY,
			logGroupName: `/aws/lambda/${concatenateLogFiles.functionName}`,
			retention: CloudWatchLogs.RetentionDays.ONE_WEEK,
		})

		const rule = new Events.Rule(this, 'invokeConcatenateMessagesRule', {
			schedule: Events.Schedule.expression('rate(1 hour)'),
			description: 'Invoke the lambda which concatenates the log messages',
			enabled: true,
			targets: [new EventTargets.LambdaFunction(concatenateLogFiles)],
		})

		concatenateLogFiles.addPermission('InvokeByEvents', {
			principal: new IAM.ServicePrincipal('events.amazonaws.com'),
			sourceArn: rule.ruleArn,
		})

		// Continuous deployment

		const cd = new CD(this, 'CD', {
			...args,
			bucket,
			userAccessKey: accessKey,
			ecr: ECR.Repository.fromRepositoryArn(this, 'ecr', args.ecrRepositoryArn),
			sourceCodeBucket,
			baseLayer,
		})

		new CloudFormation.CfnOutput(this, 'fargateServiceArn', {
			value: cd.fargateService.serviceArn,
			exportName: `${this.stackName}:fargateServiceArn`,
		})

		new CloudFormation.CfnOutput(this, 'clusterArn', {
			value: cd.cluster.clusterArn,
			exportName: `${this.stackName}:clusterArn`,
		})
	}
}
