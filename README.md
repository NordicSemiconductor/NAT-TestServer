# NAT-TestServer

![Test](https://github.com/NordicSemiconductor/NAT-TestServer/workflows/Test/badge.svg)
![Docker](https://github.com/NordicSemiconductor/NAT-TestServer/workflows/Test%20Docker%20Image/badge.svg)
[![Commitizen friendly](https://img.shields.io/badge/commitizen-friendly-brightgreen.svg)](http://commitizen.github.io/cz-cli/)
[![code style: gofmt](https://img.shields.io/badge/code_style-gofmt-00acd7.svg)](https://golang.org/cmd/gofmt/)
[![code style: prettier](https://img.shields.io/badge/code_style-prettier-ff69b4.svg)](https://github.com/prettier/prettier/)
[![ESLint: TypeScript](https://img.shields.io/badge/ESLint-TypeScript-blue.svg)](https://github.com/typescript-eslint/typescript-eslint)

## Configuration

Make these environment variable available:

> ℹ️ Linux users can use [direnv](https://direnv.net/) to simplify the process.

    export AWS_DEFAULT_REGION=<...>
    export AWS_BUCKET=<...>
    export AWS_ACCESS_KEY_ID=<...>
    export AWS_SECRET_ACCESS_KEY=<...>

Receives NAT test messages from the
[NAT Test Firmware](https://github.com/NordicSemiconductor/NAT-TestFirmware/)
and logs them and timeout occurances to S3.

## Testing

To test if the server is listening on local ports and saves the correct data,
execute the command

```
go test
```

or add the `-v` option for more detailed output.

## Running in Docker

    docker build -t nordicsemiconductor/nat-testserver .
    docker run \
        -e AWS_BUCKET \
        -e AWS_DEFAULT_REGION \
        -e AWS_ACCESS_KEY_ID \
        -e AWS_SECRET_ACCESS_KEY \
        --rm --net=host -P nordicsemiconductor/nat-testserver:latest

    # Send a package
    echo '{"op": "310410", "ip": ["10.160.1.82"], "cell_id": 84486415, "ue_mode": 2, "iccid": "8931080019073497795F", "interval":1}' | nc -w1 -u 127.0.0.1 3050

## Continuous Deployment

Install dependencies

    npm ci

Deploy the stack to an AWS account

    npx cdk deploy

Publish the docker image to AWS Elastic Container Registry

    export STACK_ID="${STACK_ID:-nat-test-resources}"
    ECR_REPOSITORY_NAME=`aws cloudformation describe-stacks --stack-name $STACK_ID | jq -r '.Stacks[0].Outputs[] | select(.OutputKey == "cdEcrRepositoryName") | .OutputValue'`
    ECR_REPOSITORY_URI=`aws cloudformation describe-stacks --stack-name $STACK_ID | jq -r '.Stacks[0].Outputs[] | select(.OutputKey == "cdEcrRepositoryUri") | .OutputValue'`
    aws ecr get-login-password --region ${AWS_REGION} | docker login --username AWS --password-stdin ${ECR_REPOSITORY_URI}
    docker tag nordicsemiconductor/nat-testserver:latest ${ECR_REPOSITORY_URI}:latest
    docker push ${ECR_REPOSITORY_URI}:latest

### Public IP

Currently there is no Load Balancer in front of the server, so the public IP
needs to be manually updated in the DNS record used by the firmware.

The IP can be extracted using:

    CLUSTER_NAME=`aws cloudformation describe-stacks --stack-name $STACK_ID | jq -r '.Stacks[0].Outputs[] | select(.OutputKey == "clusterArn") | .OutputValue'`
    TASK_ARN=`aws ecs list-tasks --cluster $CLUSTER_NAME | jq -r '.taskArns[0]'`
    NETWORK_INTERFACE_ID=`aws ecs describe-tasks --task $TASK_ARN --cluster $CLUSTER_NAME | jq -r '.tasks[0].attachments[0].details[] | select(.name == "networkInterfaceId") | .value'`
    PUBLIC_IP=`aws ec2 describe-network-interfaces --network-interface-id $NETWORK_INTERFACE_ID | jq -r '.NetworkInterfaces[0].Association.PublicIp'`
    echo Public IP: $PUBLIC_IP

### Deploying a new version of the server

Publish a new version of the image to ECR (see above), then

    SERVICE_ID=`aws cloudformation describe-stacks --stack-name $STACK_ID | jq -r '.Stacks[0].Outputs[] | select(.OutputKey == "fargateArn") | .OutputValue'`
    CLUSTER_NAME=`aws cloudformation describe-stacks --stack-name $STACK_ID | jq -r '.Stacks[0].Outputs[] | select(.OutputKey == "clusterArn") | .OutputValue'`
    aws ecs update-service --service $SERVICE_ID --cluster $CLUSTER_NAME --force-new-deployment
