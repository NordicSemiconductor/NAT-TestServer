import { ServerApp } from './ServerApp'
import { stackName } from './stackName'
import { CloudFormation } from 'aws-sdk'
import { Outputs } from './ECRStack'
import { prepareResources } from './prepare-resources'

const cf = new CloudFormation()

const main = async () => {
	const { Stacks } = await cf
		.describeStacks({
			StackName: stackName('ecr'),
		})
		.promise()

	const ecrRepoArnOutput = Stacks?.[0].Outputs?.find(
		({ OutputKey }) => OutputKey === Outputs.cdEcrRepositoryArn,
	)

	if (!ecrRepoArnOutput) {
		throw new Error(`ECR not found.`)
	}

	const res = await prepareResources()

	new ServerApp(stackName(), {
		...res,
		ecrRepositoryArn: ecrRepoArnOutput.OutputValue as string,
		updateDNSRoleArn: process.env.STS_ROLE_ARN || '',
		assumeRoleExternalID: process.env.STS_EXTERNAL_ID || '',
		hostedZoneId: process.env.HOSTED_ZONE_ID || '',
		recordName: process.env.RECORD_NAME || '',
	}).synth()
}

main().catch((err) => {
	console.error(err)
	process.exit(1)
})
