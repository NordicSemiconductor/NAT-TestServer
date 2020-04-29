import { App } from '@aws-cdk/core'
import { ServerStack } from './ServerStack'

export class ServerApp extends App {
	public constructor({
		stackId,
		ecrRepositoryArn,
	}: {
		stackId: string
		ecrRepositoryArn: string
	}) {
		super()
		new ServerStack(this, stackId, { ecrRepositoryArn })
	}
}
