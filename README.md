# NAT-TestServer

![Test](https://github.com/NordicSemiconductor/NAT-TestServer/workflows/Test/badge.svg)

Receives NAT test messages from the [NAT Test Firmware](https://github.com/NordicSemiconductor/NAT-TestFirmware/) and logs them and timeout occurances to S3.

## Testing

To test if the server is listening on local ports and saves the correct data, execute the command

```
go test
```

or add the `-v` option for more detailed output.
