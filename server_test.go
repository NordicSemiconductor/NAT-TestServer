package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

const testInterval = 5
const testIPv4 = "0.0.0.0"
const testIPv6 = "0000:0000:0000:0000:0000:0000:0000:0000"

var testCases [][]byte = [][]byte{
	[]byte("{\"op\":\"24201\",\"ip\":[\"" + testIPv4 + "\"],\"cell_id\":21229824,\"ue_mode\":2,\"lte_mode\":1,\"nbiot_mode\":1,\"iccid\":\"8931089318104314834F\",\"imei\":\"352656100367872\",\"interval\":" + strconv.Itoa(testInterval) + "}\n"),
	[]byte("{\"op\":\"24201\",\"ip\":[\"" + testIPv4 + "\"],\"cell_id\":21229824,\"ue_mode\":2,\"lte_mode\":1,\"nbiot_mode\":1,\"iccid\":\"8931089318104314834\",\"imei\":\"352656100367872\",\"interval\":" + strconv.Itoa(testInterval) + "}\n"),
	[]byte("{\"op\":\"24201\",\"ip\":[\"" + testIPv6 + "\"],\"cell_id\":21229824,\"ue_mode\":2,\"lte_mode\":1,\"nbiot_mode\":1,\"iccid\":\"8931089318104314834F\",\"imei\":\"352656100367872\",\"interval\":" + strconv.Itoa(testInterval) + "}\n"),
	[]byte("{\"op\":\"242011\",\"ip\":[\"" + testIPv4 + "\",\"" + testIPv6 + "\"],\"cell_id\":21229824,\"ue_mode\":2,\"lte_mode\":1,\"nbiot_mode\":1,\"iccid\":\"8931089318104314834F\",\"imei\":\"352656100367872\",\"interval\":" + strconv.Itoa(testInterval) + "}\n"),
}
var errorCases [][]byte = [][]byte{
	[]byte("{\"op\":,\"ip\":\"10.160.73.64\",\"cell_id\":21229824,\"ue_mode\":2,\"lte_mode\":1,\"nbiot_mode\":1,\"iccid\":\"8931089318104314834F\",\"imei\":\"352656100367872\",\"interval\":10}"),
	[]byte("{\"op\":\"24201\",\"ip\":,\"cell_id\":21229824,\"ue_mode\":2,\"lte_mode\":1,\"nbiot_mode\":1,\"iccid\":\"8931089318104314834F\",\"imei\":\"352656100367872\",\"interval\":10}"),
	[]byte("{\"op\":\"24201\",\"ip\":[\"10.160.73.64\"],\"cell_id\":,\"ue_mode\":2,\"lte_mode\":1,\"nbiot_mode\":1,\"iccid\":\"8931089318104314834F\",\"imei\":\"352656100367872\",\"interval\":10}"),
	[]byte("{\"op\":\"24201\",\"ip\":[\"10.160.73.64\"],\"cell_id\":21229824,\"ue_mode\":,\"lte_mode\":1,\"nbiot_mode\":1,\"iccid\":\"8931089318104314834F\",\"imei\":\"352656100367872\",\"interval\":10}"),
	[]byte("{\"op\":\"24201\",\"ip\":[\"10.160.73.64\"],\"cell_id\":21229824,\"ue_mode\":2,\"lte_mode\":,\"nbiot_mode\":1,\"iccid\":\"8931089318104314834F\",\"imei\":\"352656100367872\",\"interval\":10}"),
	[]byte("{\"op\":\"24201\",\"ip\":[\"10.160.73.64\"],\"cell_id\":21229824,\"ue_mode\":2,\"lte_mode\":1,\"nbiot_mode\":,\"iccid\":\"8931089318104314834F\",\"imei\":\"352656100367872\",\"interval\":10}"),
	[]byte("{\"op\":\"24201\",\"ip\":[\"10.160.73.64\"],\"cell_id\":21229824,\"ue_mode\":2,\"lte_mode\":1,\"nbiot_mode\":1,\"iccid\":,\"imei\":\"352656100367872\",\"interval\":10}"),
	[]byte("{\"op\":\"24201\",\"ip\":[\"10.160.73.64\"],\"cell_id\":21229824,\"ue_mode\":2,\"lte_mode\":1,\"nbiot_mode\":,\"iccid\":\"8931089318104314834F\",\"imei\":,\"interval\":10}"),
	[]byte("{\"op\":\"24201\",\"ip\":[\"10.160.73.64\"],\"cell_id\":21229824,\"ue_mode\":2,\"lte_mode\":1,\"nbiot_mode\":1,\"iccid\":\"8931089318104314834F\",\"imei\":\"352656100367872\",\"interval\":}"),
	[]byte("{\"op\":\"24201\",\"ip\":[\"10.160.73.64\"],\"cell_id\":21229824,\"ue_mode\":2,\"lte_mode\":1,\"nbiot_mode\":1,\"iccid\":\"8931089318104314834F\",\"imei\":\"352656100367872\",\"interval\":10,\"temp\":0}"),
	[]byte("{\"op\":\"24201\",\"ip\":[\"10.160.73.64\"],\"cell_id\":21229824,\"ue_mode\":2,\"lte_mode\":1,\"nbiot_mode\":1,\"iccid\":\"8931089318104314834F\",\"imei\":\"352656100367872\",}"),
	[]byte("{\"op\":\"24201\",\"ip\":[\"10.160.73.64\"],\"cell_id\":21229824,\"ue_mode\":2,\"lte_mode\":1,\"nbiot_mode\":1,\"iccid\":\"\",\"imei\":\"352656100367872\",\"interval\":10}"),
	[]byte("{\"op\":\"24201\",\"ip\":[\"10.160.73.64\"],\"cell_id\":21229824,\"ue_mode\":2,\"lte_mode\":1,\"nbiot_mode\":1,\"iccid\":\"8931089318104314834F\",\"imei\":\"352656100367872\",\"interval\":-1}"),
	[]byte("{\"op\":\"24201\",\"ip\":[\"10.160.73.64\"],\"cell_id\":21229824,\"ue_mode\":2,\"lte_mode\":1,\"nbiot_mode\":1,\"iccid\":\"2331089318104314834F\",\"imei\":\"352656100367872\",\"interval\":10}"),
	[]byte("{\"op\":\"1000000\",\"ip\":[\"10.160.73.64\"],\"cell_id\":21229824,\"ue_mode\":2,\"lte_mode\":1,\"nbiot_mode\":1,\"iccid\":\"8931089318104314834F\",\"imei\":\"352656100367872\",\"interval\":10}"),
	[]byte("{\"op\":\"1000\",\"ip\":[\"10.160.73.64\"],\"cell_id\":21229824,\"ue_mode\":2,\"lte_mode\":1,\"nbiot_mode\":1,\"iccid\":\"8931089318104314834F\",\"imei\":\"352656100367872\",\"interval\":10}"),
	[]byte("{\"op\":\"24201\",\"ip\":[\"10.160.73.64\"],\"cell_id\":21229824,\"ue_mode\":3,\"lte_mode\":1,\"nbiot_mode\":1,\"iccid\":\"8931089318104314834F\",\"imei\":\"352656100367872\",\"interval\":10}"),
	[]byte("{\"op\":\"24201\",\"ip\":[\"O:0db8:85a3:08d3:1319:8a2e:0370:7344\"],\"cell_id\":21229824,\"ue_mode\":3,\"lte_mode\":1,\"nbiot_mode\":1,\"iccid\":\"8931089318104314834F\",\"imei\":\"352656100367872\",\"interval\":10}"),
	[]byte("{\"op\":\"24201\",\"ip\":[\"10.160.73.64\"],\"cell_id\":21229824,\"ue_mode\":2,\"lte_mode\":-1,\"nbiot_mode\":1,\"iccid\":\"8931089318104314834F\",\"imei\":\"352656100367872\",\"interval\":10}"),
	[]byte("{\"op\":\"24201\",\"ip\":[\"10.160.73.64\"],\"cell_id\":21229824,\"ue_mode\":2,\"lte_mode\":1,\"nbiot_mode\":2,\"iccid\":\"8931089318104314834F\",\"imei\":\"352656100367872\",\"interval\":10}"),
	[]byte("{\"op\":\"24201\",\"ip\":[\"10.160.73.64\"],\"cell_id\":21229824,\"ue_mode\":2,\"lte_mode\":1,\"nbiot_mode\":1,\"iccid\":\"8931089318104314834F\",\"imei\":\"3526561003678720\",\"interval\":10}"),
}

const threadCount = 3

var testPrefix string

func TestMain(m *testing.M) {
	newUUID, err := uuid.NewRandom()
	if err != nil {
		log.Printf("Failed to create new UUID: %d\n", err)
		os.Exit(1)
		return
	}
	testPrefix = fmt.Sprintf("%s", newUUID)

	os.Setenv("LOG_PREFIX", testPrefix)
	defer os.Unsetenv("LOG_PREFIX")

	go main()

	// Make sure server has started before trying to run tests
	time.Sleep(time.Duration(5) * time.Second)

	code := m.Run()

	os.Exit(code)
}

func TestTCP(t *testing.T) {
	for i := 0; i < threadCount; i++ {
		t.Run("TCP Client", TCPFunc)
	}
}

func TCPFunc(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()

	doneChan := make(chan bool)

	conn, err := net.Dial("tcp", ":3051")
	assert.NoError(err, "It should be able to connect to the server")
	defer conn.Close()

	for _, v := range testCases {

		if _, err = conn.Write(v); err != nil {
			conn.Close()
			t.Error("Failed to write")
			return
		}

		timer := time.NewTimer(time.Duration(testInterval+1) * time.Second)
		go func() {
			select {
			case <-timer.C:
				t.Error("Server failed to answer packet")
				conn.Close()
			case <-doneChan:
				timer.Stop()
			}
		}()

		tempBuf := make([]byte, 256)
		n, err := conn.Read(tempBuf)
		assert.NoError(err, "It should read the response")
		assert.NotEqual(tempBuf[:n], genericErrorMessage, "it should return an error message")
		doneChan <- true
	}
}

func TestUDP(t *testing.T) {
	for i := 0; i < threadCount; i++ {
		t.Run("UDP Client", UDPFunc)
	}
}

func UDPFunc(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()

	doneChan := make(chan bool)

	ServerAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:3050")
	LocalAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	conn, err := net.DialUDP("udp", LocalAddr, ServerAddr)
	assert.NoError(err, "It should be able to connect to the server")
	defer conn.Close()

	for _, v := range testCases {
		if _, err = conn.Write(v); err != nil {
			conn.Close()
			t.Error("Failed to write")
			return
		}

		timer := time.NewTimer(time.Duration(testInterval+1) * time.Second)
		go func() {
			select {
			case <-timer.C:
				t.Error("Server failed to answer packet")
				conn.Close()
			case <-doneChan:
				timer.Stop()
				return
			}
		}()

		tempBuf := make([]byte, 256)
		n, err := conn.Read(tempBuf)
		assert.NoError(err, "It should read the response")
		assert.NotEqual(tempBuf[:n], genericErrorMessage, "it should return an error message")
		doneChan <- true
	}
}

func TestHandleData(t *testing.T) {
	assert := assert.New(t)
	for _, errorCase := range errorCases {
		_, _, err := HandleData(errorCase, "UDP", testIPv4)
		assert.Error(err, "An invalid message should not be accepted by the server: %s", errorCase)
	}
}

func TestOutput(t *testing.T) {
	assert := assert.New(t)
	// Wait for timeout + 10% for timeout packets to be written
	time.Sleep(time.Duration(newUDPMessageTimeoutInSeconds*1.1) * time.Second)

	sess, err := session.NewSession(&aws.Config{})
	assert.NoError(err, "A session should be created")
	svc := s3.New(sess, &aws.Config{})

	resp, err := svc.ListObjectsV2(&s3.ListObjectsV2Input{Bucket: aws.String(os.Getenv("AWS_BUCKET")), Prefix: aws.String(testPrefix)})
	assert.NoError(err, "Items in the bucket should be listed")

	var foundCount = 0
	var timedOutCount = 0
	for _, item := range resp.Contents {
		obj, err := svc.GetObject(&s3.GetObjectInput{Bucket: aws.String(os.Getenv("AWS_BUCKET")), Key: aws.String(*item.Key)})
		assert.NoError(err, "The item should be read")
		body, err := ioutil.ReadAll(obj.Body)
		assert.NoError(err, "The item's body should be read")

		var log NATLogEntry
		err = json.Unmarshal(body, &log)
		assert.NoError(err, "The item should be parsed to JSON")
		if log.Message.IP[0] == testIPv4 || log.Message.IP[0] == testIPv6 {
			foundCount++
		}
		if log.Timeout {
			timedOutCount++
		}
	}

	assert.Equal(threadCount*2*len(testCases), foundCount, "The number of log entries should be equal.")
	// The TCP messages will not timeout because the server sends the response in sync, while for UDP it waits for the *next* message to arrive before registering a success/timeout.
	// This message never arrives, because the test client is terminated after the last test case.
	assert.Equal(threadCount, timedOutCount, "Only the last UDP message should be registered as a timeout")
}
