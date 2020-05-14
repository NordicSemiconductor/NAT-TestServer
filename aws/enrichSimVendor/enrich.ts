import { identifyIssuer } from 'e118-iin-list'
import { isNone } from 'fp-ts/lib/Option'

export const enrichWithSimIssuer = (body: string) => {
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
}
