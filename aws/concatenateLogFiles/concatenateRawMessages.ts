import * as dateFns from 'date-fns'
import * as path from 'path'

import { CollectFilesFn } from './collectFiles'
import { ConcatenateFilesFn } from './concatenateFiles'

const dateRx = /^raw\/([0-9]{4})\/([0-9]{2})\/([0-9]{2})\/([0-9]{2})/

export const concatenateMessages = async ({
	concat: { raw: concatRaw, days: concatDays, months: concatMonths },
	collectFilesInBucket,
}: {
	concat: {
		raw: ConcatenateFilesFn
		days: ConcatenateFilesFn
		months: ConcatenateFilesFn
	}
	collectFilesInBucket: CollectFilesFn
}) => {
	// Concatenate hours
	await concatRaw({
		files: await collectFilesInBucket({
			Prefix: `raw/`,
			notAfterDate: dateFns.format(new Date(), "yyyy-MM-dd'T'HH"),
			fileNameToDate: (filename) => {
				const m = dateRx.exec(filename)
				if (m) {
					const [, year, month, day, hour] = m
					return `${year}-${month}-${day}T${hour}`
				}
				return dateFns.format(new Date(), "yyyy-MM-dd'T'HH") // No date found
			},
		}),
		dateToFileName: (date) => `hours/${date}.txt`,
	})
	// Concatenate days
	await concatDays({
		files: await collectFilesInBucket({
			Prefix: `hours/`,
			notAfterDate: dateFns.format(new Date(), 'yyyy-MM-dd'),
			fileNameToDate: (filename) => {
				const [year, month, day] = path
					.parse(filename)
					.name.split('T')[0]
					.split('-')
				return `${year}-${month}-${day}`
			},
		}),
		dateToFileName: (date) => `days/${date}.txt`,
	})
	// Concatenate months
	await concatMonths({
		files: await collectFilesInBucket({
			Prefix: `days/`,
			notAfterDate: dateFns.format(new Date(), 'yyyy-MM-01'),
			fileNameToDate: (filename) => {
				const [year, month] = path.parse(filename).name.split('-')
				return `${year}-${month}-01`
			},
		}),
		dateToFileName: (date) => {
			const [year, month] = date.split('-')
			return `months/${year}-${month}.txt`
		},
	})
}
