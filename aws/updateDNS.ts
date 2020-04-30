import { Route53, STS, EC2 } from 'aws-sdk'

const sts = new STS()
const ec2 = new EC2()

const STS_ROLE_ARN = process.env.STS_ROLE_ARN || ''
const STS_EXTERNAL_ID = process.env.STS_EXTERNAL_ID || ''
const HOSTED_ZONE_ID = process.env.HOSTED_ZONE_ID || ''
const RECORD_NAME = process.env.RECORD_NAME || ''

const updateRecord = async ({ A }: { A: string }) => {
	const assumedRole = await sts
		.assumeRole({
			RoleArn: STS_ROLE_ARN,
			RoleSessionName: 'UpdateDomainRecord',
			ExternalId: STS_EXTERNAL_ID,
			DurationSeconds: 900,
		})
		.promise()

	if (!assumedRole?.Credentials) throw new Error(`Failed to assume role!`)

	const r53 = new Route53({
		accessKeyId: assumedRole.Credentials.AccessKeyId,
		secretAccessKey: assumedRole.Credentials.SecretAccessKey,
		sessionToken: assumedRole.Credentials.SessionToken,
	})

	await r53
		.changeResourceRecordSets({
			HostedZoneId: HOSTED_ZONE_ID,
			ChangeBatch: {
				Changes: [
					{
						Action: 'UPSERT',
						ResourceRecordSet: {
							Name: RECORD_NAME,
							Type: 'A',
							TTL: 600,
							ResourceRecords: [
								{
									Value: A,
								},
							],
						},
					},
				],
			},
		})
		.promise()

	console.log(`Updated ${RECORD_NAME} to ${A}`)
}

type ECSEvent = {
	version: string
	id: string
	'detail-type': string
	source: 'aws.ecs'
	account: string
	time: string
	region: string
	resources: string[]
	detail: {
		clusterArn: string
		containers: {
			containerArn: string
			lastStatus: string
			name: string
			taskArn: string
			networkInterfaces: {
				attachmentId: string
				privateIpv4Address: string
			}[]
			cpu: string
			memory: string
		}[]
		createdAt: string
		launchType: string
		cpu: string
		memory: string
		desiredStatus: string
		group: string
		lastStatus: string
		overrides: {
			containerOverrides: {
				name: string
			}[]
		}
		attachments: {
			id: string
			type: string
			status: string
			details: {
				name:
					| 'subnetId'
					| 'networkInterfaceId'
					| 'macAddress'
					| 'privateIPv4Address'
				value: string
			}[]
		}[]
		connectivity: string
		connectivityAt: string
		pullStartedAt: string
		startedAt: string
		startedBy: string
		stoppingAt: string
		pullStoppedAt: string
		stoppedReason: string
		stopCode: string
		updatedAt: string
		taskArn: string
		taskDefinitionArn: string
		version: number
		platformVersion: string
	}
}

export const handler = async (event: ECSEvent) => {
	console.log(JSON.stringify(event))
	const networkInterfaceIds = event.detail.attachments
		.map(
			({ details }) =>
				details.find(({ name }) => name === 'networkInterfaceId')?.value,
		)
		.filter((ip) => ip) as string[]

	const { NetworkInterfaces } = await ec2
		.describeNetworkInterfaces({
			NetworkInterfaceIds: networkInterfaceIds,
		})
		.promise()

	const publicIp = NetworkInterfaces?.find(
		({ Association }) => Association?.PublicIp,
	)?.Association?.PublicIp
	if (!publicIp) {
		throw new Error(`No public IP found!`)
	}
	await updateRecord({ A: publicIp })
}
