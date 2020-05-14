import { CloudFormation, S3 } from 'aws-sdk'
import { stackOutput } from '@bifravst/cloudformation-helpers'
import { stackName } from '../stackName'
import { enrichWithSimIssuer } from './enrich'

const cf = new CloudFormation()
const s3 = new S3()

export const enrich = async () =>
	stackOutput(cf)<{ bucketName: string }>(stackName()).then(
		async ({ bucketName }) =>
			Promise.all([
				s3
					.listObjects({ Bucket: bucketName, Prefix: 'hours' })
					.promise()
					.then(({ Contents }) => Contents),
				s3
					.listObjects({ Bucket: bucketName, Prefix: 'days' })
					.promise()
					.then(({ Contents }) => Contents),
				s3
					.listObjects({ Bucket: bucketName, Prefix: 'months' })
					.promise()
					.then(({ Contents }) => Contents),
			])
				.then((items) =>
					(items.flat() as { Key: string }[]).map(({ Key }) => Key),
				)
				.then(async (items) =>
					items.reduce(
						async (p, item) =>
							p.then(async () =>
								s3
									.getObject({
										Bucket: bucketName,
										Key: item,
									})
									.promise()
									.then(async ({ Body }) => {
										if (!Body) return
										const messages = Body.toString().split('\n')
										await s3
											.putObject({
												Bucket: bucketName,
												Key: item,
												Body: messages
													.map((m) => {
														const p = JSON.parse(m)
														if (p.simIssuer) return m
														console.log(`Enriching ${m}...`)
														return enrichWithSimIssuer(m)
													})
													.join('\n'),
											})
											.promise()
									}),
							),
						Promise.resolve(),
					),
				),
	)
