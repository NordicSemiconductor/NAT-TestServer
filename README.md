# NAT-TestServer

![Test](https://github.com/NordicSemiconductor/NAT-TestServer/workflows/Test/badge.svg)
![Docker](https://github.com/NordicSemiconductor/NAT-TestServer/workflows/Test%20Docker%20Image/badge.svg)
[![semantic-release](https://img.shields.io/badge/%20%20%F0%9F%93%A6%F0%9F%9A%80-semantic--release-e10079.svg)](https://github.com/semantic-release/semantic-release)
[![Commitizen friendly](https://img.shields.io/badge/commitizen-friendly-brightgreen.svg)](http://commitizen.github.io/cz-cli/)
[![code style: golint](https://img.shields.io/badge/code_style-golint-00acd7.svg)](https://github.com/golang/lint)
[![code style: prettier](https://img.shields.io/badge/code_style-prettier-ff69b4.svg)](https://github.com/prettier/prettier/)
[![ESLint: TypeScript](https://img.shields.io/badge/ESLint-TypeScript-blue.svg)](https://github.com/typescript-eslint/typescript-eslint)

Receives NAT test messages from the
[NAT Test Firmware](https://github.com/NordicSemiconductor/NAT-TestFirmware/)
and logs them and timeout occurances to S3.

## NAT Timeout determination

The server listens for TCP and UPD connections. Clients can send messages
according to the [`schema.json`](./schema.json) which contain an `interval`
property. This instructs the server to wait that amount in seconds before
returning a response.

On _TCP connections_ the connection is considered to be timed out when the
server cannot reply to the client after the interval has passed. The TCP
connection will ensure that the client receives the message from the server if
it is still connected.

On _UDP connections_ the connection is considered to be timed out when the
server does not receive a new message from the client within 60 seconds after
having sent the response for the previous message. There is no other way to
ensure that the connection is intact.

## Testing

Make these environment variable available:

> ℹ️ Linux users can use [direnv](https://direnv.net/) to simplify the process.

    export AWS_REGION=<...>
    export AWS_BUCKET=<...>
    export AWS_ACCESS_KEY_ID=<...>
    export AWS_SECRET_ACCESS_KEY=<...>

To test if the server is listening on local ports and saves the correct data,
execute the command

```
go get -v -t -d ./...
go test -v
```

or add the `-v` option for more detailed output.

## Running in Docker

    docker build -t nordicsemiconductor/nat-testserver .
    docker run \
        -e AWS_BUCKET \
        -e AWS_REGION \
        -e AWS_ACCESS_KEY_ID \
        -e AWS_SECRET_ACCESS_KEY \
        --rm --net=host -P nordicsemiconductor/nat-testserver:latest

    # Send a package
    echo '{"op": "310410", "ip": ["10.160.1.82"], "cell_id": 84486415, "ue_mode": 2, "iccid": "8931080019073497795F", "interval":1}' | nc -w1 -u 127.0.0.1 3050

## Deploy to AWS

### Note on the public IP

Currently there is no Load Balancer in front of the server (after all this
service was developed to test NAT timeouts on UDP and TCP connections, so we
need to have the actual servers instance terminate the connection, not the load
balancer).

Therefore the public IP needs to be manually updated in the DNS record used by
the firmware because there is no other way right now when using Fargate.

For this there is a lambda function which listens to change events from the
cluster in the _service account_ and updates a DNS record on a Route53 Hosted
Zone which is in a separate _DNS account_. Unfortunately it is currently not
possible to limit access to subdomains in a Hosted Zone.

Prepare a role in the _domain account_ with these permissions, replace

- `<Zone ID>` with the ID of your hosted zone.

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "VisualEditor0",
      "Effect": "Allow",
      "Action": "route53:ChangeResourceRecordSets",
      "Resource": "arn:aws:route53:::hostedzone/<Zone ID>"
    }
  ]
}
```

Create a trust relationship for the _service account_, replace

- `<Account ID of the service account>` with the account id of the _service
  account_
- `<External ID>` with a random string (e.g. use a UUIDv4)

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "AWS": "arn:aws:iam::<Account ID of the service account>:root"
      },
      "Action": "sts:AssumeRole",
      "Condition": {
        "StringEquals": {
          "sts:ExternalId": "<External ID>"
        }
      }
    }
  ]
}
```

### Deploy

Make these environment variable available:

> ℹ️ Linux users can use [direnv](https://direnv.net/) to simplify the process.

    export AWS_REGION=<...>
    export AWS_ACCESS_KEY_ID=<Access Key ID of the service account>
    export AWS_SECRET_ACCESS_KEY=<Secret Access Key of the service account>
    export STS_ROLE_ARN=<ARN of the role created in the domain account>
    export STS_EXTERNAL_ID=<External ID from above>
    export HOSTED_ZONE_ID=<Zone ID from above>
    export RECORD_NAME=<FQDN of the A record to update>

Install dependencies

    npm ci

Set the ID of the stack

    export STACK_NAME="${STACK_NAME:-nat-test-resources}"

Prepare the account for CDK resources:

    npx cdk -a 'node dist/cdk-sourcecode.js' deploy
    npx cdk bootstrap

Deploy the ECR stack to an AWS Account

    npx cdk -a 'node dist/cdk-ecr.js' deploy ${STACK_NAME}-ecr

Publish the docker image to AWS Elastic Container Registry

    ECR_REPOSITORY_NAME=`aws cloudformation describe-stacks --stack-name ${STACK_NAME}-ecr | jq -r '.Stacks[0].Outputs[] | select(.OutputKey == "cdEcrRepositoryName") | .OutputValue'`
    ECR_REPOSITORY_URI=`aws cloudformation describe-stacks --stack-name ${STACK_NAME}-ecr | jq -r '.Stacks[0].Outputs[] | select(.OutputKey == "cdEcrRepositoryUri") | .OutputValue'`
    aws ecr get-login-password --region ${AWS_REGION} | docker login --username AWS --password-stdin ${ECR_REPOSITORY_URI}
    docker tag nordicsemiconductor/nat-testserver:latest ${ECR_REPOSITORY_URI}:latest
    docker push ${ECR_REPOSITORY_URI}:latest

Deploy the server stack to an AWS account

    npx cdk deploy $STACK_NAME

## Continuous Deployment

Continuous Deployment of releases is done
[through GitHub Actions](.github/workflows/cd.yaml). Configure these secrets:

- `CD_AWS_REGION`: Region where the stack is deployed
- `CD_AWS_ACCESS_KEY_ID`: Access key ID for the CD user
- `CD_AWS_SECRET_ACCESS_KEY`: Secret access key for the CD user
- `STS_ROLE_ARN`: ARN of the role created in the domain account
- `STS_EXTERNAL_ID`: External ID from above
- `HOSTED_ZONE_ID`: Zone ID from above
- `RECORD_NAME`: FQDN of the A record to update
- `USER_GITHUB_TOKEN_FOR_ACTION_TRIGGER`: In order to be able to trigger this
  action, a GitHub user token with the permissions `public_repo`, `repo:status`,
  `repo_deployment` is needed (the default Actions credentials
  [can't trigger other Actions](https://help.github.com/en/actions/reference/events-that-trigger-workflows#triggering-new-workflows-using-a-personal-access-token)).

Afterwards the [Test Action](.github/workflows/test.yml) will trigger a
deployment.

## Deploying a new version of the server manually

Publish a new version of the image to ECR (see above), then trigger a new
deployment:

    SERVICE_ID=`aws cloudformation describe-stacks --stack-name ${STACK_NAME} | jq -r '.Stacks[0].Outputs[] | select(.OutputKey == "fargateServiceArn") | .OutputValue'`
    CLUSTER_NAME=`aws cloudformation describe-stacks --stack-name ${STACK_NAME} | jq -r '.Stacks[0].Outputs[] | select(.OutputKey == "clusterArn") | .OutputValue'`
    aws ecs update-service --service $SERVICE_ID --cluster $CLUSTER_NAME --force-new-deployment
