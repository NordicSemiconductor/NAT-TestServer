import * as path from 'path'
import { promises as fs } from 'fs'
import { getLambdaSourceCodeBucketName } from './getLambdaSourceCodeBucketName'
import {
	packBaseLayer,
	packLayeredLambdas,
	WebpackMode,
} from '@bifravst/package-layered-lambdas'

export type Lambdas = {
	updateDNS: string
	concatenateLogFiles: string
}

export const prepareResources = async () => {
	const rootDir = process.cwd()
	const outDir = path.resolve(rootDir, 'dist', 'lambdas')
	try {
		await fs.stat(outDir)
	} catch {
		await fs.mkdir(outDir)
	}
	const sourceCodeBucketName = await getLambdaSourceCodeBucketName()
	const baseLayerZipFileName = await packBaseLayer({
		srcDir: rootDir,
		outDir,
		Bucket: sourceCodeBucketName,
	})
	const lambdas = await packLayeredLambdas<Lambdas>({
		id: 'bifravst',
		mode: WebpackMode.production,
		srcDir: rootDir,
		outDir,
		Bucket: sourceCodeBucketName,
		lambdas: {
			updateDNS: path.resolve(rootDir, 'aws', 'updateDNS.ts'),
			concatenateLogFiles: path.resolve(
				rootDir,
				'aws',
				'concatenateLogFiles',
				'lambda.ts',
			),
		},
		tsConfig: path.resolve(rootDir, 'tsconfig.json'),
	})

	return {
		sourceCodeBucketName,
		baseLayerZipFileName,
		lambdas,
	}
}
