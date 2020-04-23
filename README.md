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

    export AWS_REGION=<...>
    export AWS_BUCKET=<...>
    export AWS_ACCESS_KEY_ID=<...>
    export AWS_SECRET_ACCESS_KEY=<...>

Receives NAT test messages from the [NAT Test Firmware](https://github.com/NordicSemiconductor/NAT-TestFirmware/) and logs them and timeout occurances to S3.

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
        -e AWS_REGION \
        -e AWS_ACCESS_KEY_ID \
        -e AWS_SECRET_ACCESS_KEY \
        --rm --net=host -P nordicsemiconductor/nat-testserver:latest

    # Send a package
    echo '{"op": "310410", "ip": ["10.160.1.82"], "cell_id": 84486415, "ue_mode": 2, "iccid": "8931080019073497795F", "interval":1}' | nc -w1 -u 127.0.0.1 3050
