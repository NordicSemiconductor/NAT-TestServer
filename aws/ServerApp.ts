import { App } from '@aws-cdk/core'
import { ServerStack } from './ServerStack'
import { LayeredLambdas } from '@bifravst/package-layered-lambdas'
import { Lambdas } from './prepare-resources'

export class ServerApp extends App {
	public constructor(
		stackId: string,
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
		super()
		new ServerStack(this, stackId, args)
	}
}
