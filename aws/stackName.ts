export const stackName = (type?: 'ecr') =>
	`${process.env.STACK_ID || 'nat-test-resources'}${type ? `-${type}` : ''}`
