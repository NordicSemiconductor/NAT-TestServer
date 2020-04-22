package main

import (
	"testing"
	"net"
	"os"
	"time"
	"bytes"
	"strings"
	"io/ioutil"
	"strconv"
	"log"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/aws/session"
	"encoding/json"
)

const testInterval = 10
const testIP = "0.0.0.0"
var testBuffer []byte = []byte("{\"op\":\"24201\",\"ip\":\"" + testIP + "\",\"cell_id\":21229824,\"ue_mode\":2,\"iccid\":\"8931089318104314834F\",\"interval\":"+ strconv.Itoa(testInterval) +"}\n")
var errorCases [][]byte = [][]byte{
	[]byte("{\"op\":,\"ip\":\"10.160.73.64\",\"cell_id\":21229824,\"ue_mode\":2,\"iccid\":\"8931089318104314834F\",\"interval\":10}"),
	[]byte("{\"op\":\"24201\",\"ip\":,\"cell_id\":21229824,\"ue_mode\":2,\"iccid\":\"8931089318104314834F\",\"interval\":10}"),
	[]byte("{\"op\":\"24201\",\"ip\":\"10.160.73.64\",\"cell_id\":,\"ue_mode\":2,\"iccid\":\"8931089318104314834F\",\"interval\":10}"),
	[]byte("{\"op\":\"24201\",\"ip\":\"10.160.73.64\",\"cell_id\":21229824,\"ue_mode\":,\"iccid\":\"8931089318104314834F\",\"interval\":10}"),
	[]byte("{\"op\":\"24201\",\"ip\":\"10.160.73.64\",\"cell_id\":21229824,\"ue_mode\":2,\"iccid\":,\"interval\":10}"),
	[]byte("{\"op\":\"24201\",\"ip\":\"10.160.73.64\",\"cell_id\":21229824,\"ue_mode\":2,\"iccid\":\"8931089318104314834F\",\"interval\":}"),
	[]byte("{\"op\":\"24201\",\"ip\":\"10.160.73.64\",\"cell_id\":21229824,\"ue_mode\":2,\"iccid\":\"8931089318104314834F\",\"interval\":10,\"temp\":0}"),
	[]byte("{\"op\":\"24201\",\"ip\":\"10.160.73.64\",\"cell_id\":21229824,\"ue_mode\":2,\"iccid\":\"8931089318104314834F\"}"),
	[]byte("{\"op\":\"24201\",\"ip\":\"10.160.73.64\",\"cell_id\":21229824,\"ue_mode\":2,\"iccid\":\"\",\"interval\":10}"),
	[]byte("{\"op\":\"24201\",\"ip\":\"10.160.73.64\",\"cell_id\":21229824,\"ue_mode\":2,\"iccid\":\"8931089318104314834F\",\"interval\":-1}"),
	[]byte("{\"op\":\"24201\",\"ip\":\"256.256.256.256\",\"cell_id\":21229824,\"ue_mode\":2,\"iccid\":\"8931089318104314834F\",\"interval\":10}"),
	[]byte("{\"op\":\"24201\",\"ip\":\"10-160-73-64\",\"cell_id\":21229824,\"ue_mode\":2,\"iccid\":\"8931089318104314834F\",\"interval\":10}"),
	[]byte("{\"op\":\"24201\",\"ip\":\"10.160.73.64.10\",\"cell_id\":21229824,\"ue_mode\":2,\"iccid\":\"8931089318104314834F\",\"interval\":10}"),
	[]byte("{\"op\":\"24201\",\"ip\":\"10.160.73.64\",\"cell_id\":21229824,\"ue_mode\":2,\"iccid\":\"2331089318104314834F\",\"interval\":10}"),
	[]byte("{\"op\":\"100000\",\"ip\":\"10.160.73.64\",\"cell_id\":21229824,\"ue_mode\":2,\"iccid\":\"8931089318104314834F\",\"interval\":10}"),
	[]byte("{\"op\":\"24201\",\"ip\":\"10.160.73.64\",\"cell_id\":21229824,\"ue_mode\":3,\"iccid\":\"8931089318104314834F\",\"interval\":10}"),
}
var startTime time.Time
const threadCount = 3
const sendPacketCount = 2
// thread count * protocol count * packets saved (2 * received & 1 * timeout)
const expectedPacketCount = threadCount * 2 * (sendPacketCount + 1)

func TestMain(m *testing.M) {
	startTime = time.Now()

	log.SetOutput(ioutil.Discard)
	go main()

	// Make sure server has started before trying to run tests
	time.Sleep(time.Duration(5)*time.Second)

	code := m.Run()

	os.Exit(code)
}

func TestTCP(t *testing.T) {
	for i := 0; i < threadCount; i++ {
		t.Run("TCP Client", TCPFunc)
	}
}

func TCPFunc(t *testing.T) {
	t.Parallel()

	doneChan := make(chan bool)

    conn, err := net.Dial("tcp", ":3051")
    if err != nil {
        t.Error("could not connect to server: ", err)
	}
	defer conn.Close()
	
	for i := 0; i < sendPacketCount; i++ {

		if _, err = conn.Write(testBuffer); err != nil {
			conn.Close()
			t.Error("Failed to write")
		}

		timer := time.NewTimer(time.Duration(testInterval + 1) * time.Second);
		go func() {
			select{
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
		if err != nil {
			conn.Close()
			t.Error("Error reading connection")
			return
		} else if bytes.Compare(tempBuf[:n], dcBuffer) == 0 {
			conn.Close()
			t.Error("Wrong format in packet")
			return
		}
		doneChan<-true
	}
}

func TestUDP(t *testing.T) {
	for i := 0; i < threadCount; i++ {
		t.Run("UDP Client", UDPFunc)
	}
}

func UDPFunc(t *testing.T) {
	t.Parallel()

	doneChan := make(chan bool)

    ServerAddr,err := net.ResolveUDPAddr("udp","127.0.0.1:3050")
    if err != nil {
		t.Error("Error resolving remote address")
	}
 
    LocalAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
    if err != nil {
		t.Error("Error resolving local address")
	}
  
	conn, err := net.DialUDP("udp", LocalAddr, ServerAddr)
	if err != nil {
		t.Error("Error connecting to server")
	}
 
	defer conn.Close()
	
	for i := 0; i < sendPacketCount; i++ {
		if _, err = conn.Write(testBuffer); err != nil {
			conn.Close()
			t.Error("Failed to write")
		}

		timer := time.NewTimer(time.Duration(testInterval + 1) * time.Second);
		go func() {
			select{
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
		if err != nil {
			conn.Close()
			t.Error("Error reading connection")
			return
		} else if bytes.Compare(tempBuf[:n], dcBuffer) == 0 {
			conn.Close()
			t.Error("Wrong format in packet")
			return
		}
		doneChan<-true
	}
}

func TestHandleData(t *testing.T) {
	for _,errorCase := range errorCases {
		ret, _, err := HandleData(errorCase, "UDP", testIP)
		if err == nil && strings.Compare(string(ret), string(dcBuffer)) != 0{
			t.Errorf("Wrong format was accepted by server. Sent: %s\n", errorCase)
		}
	}
}

func TestOutput(t *testing.T) {
	// Wait for timeout + 10% for timeout packets to be written
	time.Sleep(time.Duration(newPacketTimeout *1.1)*time.Second)
	endTime := time.Now()

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(os.Getenv("AWS_REGION"))},
	)
	if err != nil {
		log.Fatal("Error creating session ", err)
	} 
	svc := s3.New(sess, &aws.Config{
		Region: aws.String(os.Getenv("AWS_REGION"))},
	)

    resp, err := svc.ListObjectsV2(&s3.ListObjectsV2Input{Bucket: aws.String(os.Getenv("AWS_BUCKET"))})
    if err != nil {
        t.Error("Unable to get bucket items", err)
    }

	var foundCount int
    for _, item := range resp.Contents {
		tempTime := *item.LastModified
		if tempTime.After(startTime) && tempTime.Before(endTime)  {
			obj, err := svc.GetObject(&s3.GetObjectInput{Bucket: aws.String(os.Getenv("AWS_BUCKET")), Key: aws.String(*item.Key)})
			if err != nil {
				t.Error("Unable to read bucket item", err)
			}
			body, err := ioutil.ReadAll(obj.Body)
			if err != nil {
				t.Error("Failed to read body of file")
			}

			var data SaveData
			err = json.Unmarshal(body, &data)
			if err != nil {
				t.Error("Failed to read json data")
			} else if data.Data.IP == testIP {
				foundCount++	
			}
		}
	}

	if foundCount != expectedPacketCount{
		t.Errorf("Expected number of files: %d, Found:%d\n", expectedPacketCount, foundCount)
	}
}
