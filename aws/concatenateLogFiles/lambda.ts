import { S3 } from 'aws-sdk'
import { collectFiles } from './collectFiles'
import { concatenateFiles } from './concatenateFiles'
import { concatenateMessages } from './concatenateRawMessages'
import { enrichWithSimIssuer } from '../enrichSimVendor/enrich'

const s3 = new S3()
const Bucket = process.env.BUCKET_NAME || ''

const collectFilesInBucket = collectFiles({ s3, Bucket })
const passConcatenate = concatenateFiles({ s3, Bucket })
const enrichWithSIMVendorConcatenate = concatenateFiles({
	s3,
	Bucket,
	transform: enrichWithSimIssuer,
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
