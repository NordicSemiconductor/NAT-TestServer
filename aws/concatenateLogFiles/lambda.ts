import { S3 } from 'aws-sdk'
import { identifyIssuer } from 'e118-iin-list'
import { collectFiles } from './collectFiles'
import { concatenateFiles } from './concatenateFiles'
import { concatenateMessages } from './concatenateRawMessages'
import { isNone } from 'fp-ts/lib/Option'

const s3 = new S3()
const Bucket = process.env.BUCKET_NAME || ''

const collectFilesInBucket = collectFiles({ s3, Bucket })
const passConcatenate = concatenateFiles({ s3, Bucket })
const enrichWithSIMVendorConcatenate = concatenateFiles({
	s3,
	Bucket,
	transform: (body: string) => {
		try {
			const message = JSON.parse(body)
			const iccid = message.Message?.iccid
			if (!iccid) {
				console.error({
					enrichWithSIMVendorConcatenate: {
						message: `Message did not contain ICCID!`,
						body,
					},
				})
				return body
			}
			const issuer = identifyIssuer(iccid)
			if (isNone(issuer)) {
				console.error({
					enrichWithSIMVendorConcatenate: {
						message: `Could not identify issuer for ICCID!`,
						iccid,
					},
				})
				return body
			}
			return JSON.stringify({
				...message,
				simIssuer: issuer.value,
			})
		} catch {
			console.error({
				enrichWithSIMVendorConcatenate: {
					message: `Messages is not JSON!`,
					body,
				},
			})
			return body
		}
	},
})

/**
 * Runs every hour and concatenates the raw log files so it is more performant for Athena to query them.
 * It also parses the ICCID and determines the SIM vendor.
 */
export const handler = async () => {
	await concatenateMessages({
		collectFilesInBucket,
		concat: {
			raw: enrichWithSIMVendorConcatenate,
			days: passConcatenate,
			months: passConcatenate,
		},
	})
}
