export const stackName = (type?: 'ecr' | 'sourcecode') =>
	`${process.env.STACK_NAME || 'nat-test-resources'}${type ? `-${type}` : ''}`
