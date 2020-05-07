import * as CloudFormation from '@aws-cdk/core'
import * as IAM from '@aws-cdk/aws-iam'
import * as ECR from '@aws-cdk/aws-ecr'
import * as EC2 from '@aws-cdk/aws-ec2'
import * as ECS from '@aws-cdk/aws-ecs'
import * as S3 from '@aws-cdk/aws-s3'
import * as Logs from '@aws-cdk/aws-logs'
import * as EventTargets from '@aws-cdk/aws-events-targets'
import * as Events from '@aws-cdk/aws-events'
import * as Lambda from '@aws-cdk/aws-lambda'
import { Lambdas } from './prepare-resources'
import { LayeredLambdas } from '@bifravst/package-layered-lambdas'
import * as CloudWatchLogs from '@aws-cdk/aws-logs'
export class CD extends CloudFormation.Resource {
	public readonly fargateService: ECS.IFargateService
	public readonly cluster: ECS.ICluster

	public constructor(
		parent: CloudFormation.Stack,
		id: string,
		{
			bucket,
			userAccessKey,
			ecr,
			sourceCodeBucket,
			baseLayer,
			lambdas,
			updateDNSRoleArn,
			assumeRoleExternalID,
			hostedZoneId,
			recordName,
		}: {
			bucket: S3.IBucket
			userAccessKey: IAM.CfnAccessKey
			ecr: ECR.IRepository
			sourceCodeBucket: S3.IBucket
			baseLayer: Lambda.ILayerVersion
			lambdas: LayeredLambdas<Lambdas>
			updateDNSRoleArn: string
			assumeRoleExternalID: string
			hostedZoneId: string
			recordName: string
		},
	) {
		super(parent, id)

		const vpc = new EC2.Vpc(this, 'VPC', {
			cidr: '10.3.0.0/16',
			maxAzs: 2,
			subnetConfiguration: [
				{
					subnetType: EC2.SubnetType.PUBLIC,
					name: 'PublicSubnetOne',
					cidrMask: 25,
				},
				{
					subnetType: EC2.SubnetType.PUBLIC,
					name: 'PublicSubnetTwo',
					cidrMask: 25,
				},
			],
		})

		this.cluster = new ECS.Cluster(this, 'Cluster', {
			vpc: vpc,
		})

		const runNatTestServerTaskDefinition = new ECS.FargateTaskDefinition(
			this,
			'RunNatTestServer',
			{
				memoryLimitMiB: 512,
				cpu: 256,
			},
		)

		const container = runNatTestServerTaskDefinition.addContainer(
			'NatTestServerContainer',
			{
				image: ECS.ContainerImage.fromEcrRepository(ecr),
				environment: {
					AWS_ACCESS_KEY_ID: userAccessKey.ref,
					AWS_SECRET_ACCESS_KEY: userAccessKey.attrSecretAccessKey,
					AWS_REGION: parent.region,
					AWS_BUCKET: bucket.bucketName,
					LOG_PREFIX: 'raw',
				},
				cpu: 256,
				memoryLimitMiB: 512,
				memoryReservationMiB: 512,
				logging: new ECS.AwsLogDriver({
					streamPrefix: 'NatTestServer',
					logRetention: Logs.RetentionDays.ONE_WEEK,
				}),
				essential: true,
				readonlyRootFilesystem: true,
			},
		)
		container.addPortMappings({
			containerPort: 3051,
		})
		container.addPortMappings({
			containerPort: 3050,
			protocol: ECS.Protocol.UDP,
		})

		const securityGroup = new EC2.SecurityGroup(this, 'natTestServer', {
			vpc,
			allowAllOutbound: true,
			description: 'Security group for NAT Test Server',
		})
		securityGroup.addEgressRule(EC2.Peer.anyIpv4(), EC2.Port.allTcp())
		securityGroup.addIngressRule(EC2.Peer.anyIpv4(), EC2.Port.tcp(3051))
		securityGroup.addIngressRule(EC2.Peer.anyIpv4(), EC2.Port.udp(3050))

		this.fargateService = new ECS.FargateService(
			this,
			'NatTestServerFargateService',
			{
				cluster: this.cluster,
				taskDefinition: runNatTestServerTaskDefinition,
				securityGroup,
				desiredCount: 1,
				minHealthyPercent: 0,
				maxHealthyPercent: 100,
				assignPublicIp: true,
			},
		)

		// Listen to Cluster Change messages and update the public IP
		const updateDNS = new Lambda.Function(this, 'updateDNS', {
			layers: [baseLayer],
			handler: 'index.handler',
			runtime: Lambda.Runtime.NODEJS_12_X,
			timeout: CloudFormation.Duration.seconds(900),
			memorySize: 1792,
			code: Lambda.Code.bucket(
				sourceCodeBucket,
				lambdas.lambdaZipFileNames.updateDNS,
			),
			description:
				'Updates the DNS record whenever the Fargate instance changes',
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
					resources: [updateDNSRoleArn],
					actions: ['sts:AssumeRole'],
				}),
				new IAM.PolicyStatement({
					resources: ['*'],
					actions: ['ec2:DescribeNetworkInterfaces'],
				}),
			],
			environment: {
				STS_ROLE_ARN: updateDNSRoleArn,
				STS_EXTERNAL_ID: assumeRoleExternalID,
				HOSTED_ZONE_ID: hostedZoneId,
				RECORD_NAME: recordName,
			},
		})

		new CloudWatchLogs.LogGroup(parent, `updateDNSLogGroup`, {
			removalPolicy: CloudFormation.RemovalPolicy.DESTROY,
			logGroupName: `/aws/lambda/${updateDNS.functionName}`,
			retention: CloudWatchLogs.RetentionDays.ONE_WEEK,
		})

		const rule = new Events.Rule(this, 'invokeOnECSTaskStateChange', {
			description:
				'Triggers the update DNS lambda when a ECS Task State Change event occurs',
			enabled: true,
			eventPattern: {
				detailType: ['ECS Task State Change'],
				detail: {
					clusterArn: [this.cluster.clusterArn],
					desiredStatus: ['RUNNING'],
					lastStatus: ['RUNNING'],
				},
			},
			targets: [new EventTargets.LambdaFunction(updateDNS)],
		})

		updateDNS.addPermission('InvokeByEvents', {
			principal: new IAM.ServicePrincipal('events.amazonaws.com'),
			sourceArn: rule.ruleArn,
		})
	}
}
