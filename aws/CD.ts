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
			cidr: '10.3.0.0/21',
			maxAzs: 2,
			subnetConfiguration: [
				{
					subnetType: EC2.SubnetType.PUBLIC,
					name: 'Ingress',
					cidrMask: 24,
				},
			],
		})

		const cluster = new ECS.Cluster(this, 'Cluster', {
			vpc: vpc,
		})

		const runNatTestServerTask = new ECS.FargateTaskDefinition(
			this,
			'RunNatTestServer',
			{
				memoryLimitMiB: 512,
				cpu: 256,
			},
		)

		const container = runNatTestServerTask.addContainer(
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
			},
		)
		container.addPortMappings({
			containerPort: 22,
		})
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
		securityGroup.addIngressRule(EC2.Peer.anyIpv4(), EC2.Port.allTcp())
		securityGroup.addIngressRule(EC2.Peer.anyIpv6(), EC2.Port.allTcp())
		securityGroup.addEgressRule(EC2.Peer.anyIpv4(), EC2.Port.allTcp())
		securityGroup.addEgressRule(EC2.Peer.anyIpv6(), EC2.Port.allTcp())

		new ECS.FargateService(this, 'NatTestServerFargateService', {
			cluster,
			taskDefinition: runNatTestServerTask,
			securityGroup,
			desiredCount: 1,
			minHealthyPercent: 0,
			maxHealthyPercent: 100,
			assignPublicIp: true,
		})
	}
}
