export const stackName = (type?: 'ecr' | 'sourcecode') =>
	`${process.env.STACK_ID || 'nat-test-resources'}${type ? `-${type}` : ''}`
