import * as CloudFormation from '@aws-cdk/core'
import * as IAM from '@aws-cdk/aws-iam'
import * as ECR from '@aws-cdk/aws-ecr'
import * as EC2 from '@aws-cdk/aws-ec2'
import * as ECS from '@aws-cdk/aws-ecs'
import * as S3 from '@aws-cdk/aws-s3'
import * as Logs from '@aws-cdk/aws-logs'
export class CD extends CloudFormation.Resource {
	public readonly ecr: ECR.IRepository
	public readonly accessKey: IAM.CfnAccessKey
	public readonly fargate: ECS.IFargateService
	public readonly cluster: ECS.ICluster

	public constructor(
		parent: CloudFormation.Stack,
		id: string,
		{
			bucket,
			userAccessKey,
		}: { bucket: S3.IBucket; userAccessKey: IAM.CfnAccessKey },
	) {
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
				image: ECS.ContainerImage.fromEcrRepository(this.ecr),
				environment: {
					AWS_ACCESS_KEY_ID: userAccessKey.ref,
					AWS_SECRET_ACCESS_KEY: userAccessKey.attrSecretAccessKey,
					AWS_DEFAULT_REGION: parent.region,
					AWS_BUCKET: bucket.bucketName,
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

		this.fargate = new ECS.FargateService(this, 'NatTestServerFargateService', {
			cluster: this.cluster,
			taskDefinition: runNatTestServerTaskDefinition,
			securityGroup,
			desiredCount: 1,
			minHealthyPercent: 0,
			maxHealthyPercent: 100,
			assignPublicIp: true,
		})
	}
}
