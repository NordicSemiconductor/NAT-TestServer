package main

import (
	"container/list"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/google/uuid"
	"github.com/xeipuuv/gojsonschema"
)

type DeviceMessage struct {
	Operator  string   `json:"op"`
	IP        []string `json:"ip"`
	CellId    int      `json:"cell_id"`
	UEMode    int      `json:"ue_mode"`
	LTEMode   int      `json:"lte_mode"`
	NBIotMode int      `json:"nbiot_mode"`
	ICCID     string   `json:"iccid"`
	IMEI      string   `json:"imei"`
	Interval  int      `json:"interval"`
}

type NATLogEntry struct {
	Protocol  string
	IP        string
	Timeout   bool
	Timestamp time.Time
	Message   DeviceMessage
}

type UDPClient struct {
	Addr     net.Addr
	Timer    *time.Timer
	DoneChan chan bool
}

type UDPClientList struct {
	ClientList *list.List
	Mux        sync.Mutex
}

const UDPport = 3050
const TCPport = 3051
const newPacketTimeout = 60
const maxBufferSize = 256
const schemaFile = "schema.json"
const timeFormat = "2006-01-02T15:04:05.00-0700"

var dcBuffer []byte = []byte("Error occured.\nConnection closed.\n")
var writeLog chan NATLogEntry
var schemaLoader gojsonschema.JSONLoader
var SafeUDPClients UDPClientList
var Version string = "0.0.0-development"

func saveLog(awsBucket string, prefix string) {
	sess, err := session.NewSession(&aws.Config{})
	if err != nil {
		log.Fatal("Error creating session ", err)
	}
	svc := s3.New(sess, &aws.Config{})

	for i := range writeLog {
		buffer, err := json.Marshal(i)
		if err != nil {
			log.Printf("JSON invalid. Cannot write to file, error: %d\n", err)
			return
		}

		newUUID, err := uuid.NewRandom()
		if err != nil {
			log.Printf("Failed to create new UUID: %d\n", err)
			return
		}

		key := fmt.Sprintf("%s/%s-%s-%s.json", i.Timestamp.Format("2006/01/02"), i.IP, i.Timestamp.Format("150405"), newUUID)
		log.Printf("Uploading %s: %s", key, buffer)

		ctx := context.Background()
		var cancelFn func()
		ctx, cancelFn = context.WithTimeout(ctx, 60*time.Second)

		go func() {
			var Key = key
			if len(prefix) > 0 {
				Key = fmt.Sprintf("%s/%s", prefix, key)
			}
			_, err = svc.PutObjectWithContext(ctx, &s3.PutObjectInput{
				Bucket: aws.String(awsBucket),
				Key:    aws.String(Key),
				Body:   strings.NewReader(string(buffer)),
			})
			if err != nil {
				if aerr, ok := err.(awserr.Error); ok && aerr.Code() == request.CanceledErrorCode {
					log.Printf("Upload canceled due to timeout, %s\n", err.Error())
				} else {
					log.Printf("Failed to upload object, %s\n", err.Error())
				}
			}

			if cancelFn != nil {
				cancelFn()
			}
		}()
	}
}

func HandleData(buffer []byte, protocol string, addr string) ([]byte, NATLogEntry, error) {
	timestamp := time.Now()

	documentLoader := gojsonschema.NewStringLoader(string(buffer))
	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return nil, NATLogEntry{}, err
	} else if !result.Valid() {
		return nil, NATLogEntry{}, errors.New("Message uses wrong format.\n")
	}

	log.Printf("%s Message received from %s\n", protocol, addr)

	var message DeviceMessage
	err = json.Unmarshal(buffer, &message)
	if err != nil {
		return nil, NATLogEntry{}, err
	}

	time.Sleep(time.Duration(message.Interval) * time.Second)

	endTime := time.Now().Format(timeFormat)
	retString := "Interval: " + strconv.Itoa(message.Interval) + "\nReturned: " + endTime + "\n"
	saveData := NATLogEntry{Timestamp: timestamp, Protocol: protocol, IP: addr, Timeout: false, Message: message}
	return []byte(retString), saveData, nil
}

func HandleUDP(pc net.PacketConn, addr net.Addr, buffer []byte) {
	doneChan := make(chan bool)

	retBuffer, recvData, err := HandleData(buffer, "UDP", addr.String())
	if err != nil {
		log.Printf("HandleData Error: %s\nConnection to %s terminated.\n", err.Error(), addr.String())
		pc.WriteTo(dcBuffer, addr)
		return
	}

	log.Printf("UDP Packet received from %s\n", addr.String())

	_, err = pc.WriteTo(retBuffer, addr)
	if err != nil {
		log.Printf("UDP write to %s failed, error: %s\n", addr.String(), err.Error())
		recvData.Timeout = true
		writeLog <- recvData
		return
	}
	log.Printf("UDP Packet sent to %s\n", addr.String())

	timer := time.NewTimer(newPacketTimeout * time.Second)

	SafeUDPClients.Mux.Lock()
	clientRef := SafeUDPClients.ClientList.PushBack(UDPClient{Addr: addr, Timer: timer, DoneChan: doneChan})
	SafeUDPClients.Mux.Unlock()
	select {
	case <-timer.C:
		SafeUDPClients.Mux.Lock()
		SafeUDPClients.ClientList.Remove(clientRef)
		SafeUDPClients.Mux.Unlock()
		log.Printf("UDP connection to %s timed out. Connection terminated.\n", addr.String())
		recvData.Timeout = true
		writeLog <- recvData
	case <-doneChan:
		writeLog <- recvData
		timer.Stop()
	}
}

func StopUDPClientTimeout(addr net.Addr) {
	SafeUDPClients.Mux.Lock()
	for e := SafeUDPClients.ClientList.Front(); e != nil; e = e.Next() {
		client := e.Value.(UDPClient)
		if strings.Compare(client.Addr.String(), addr.String()) == 0 {
			client.Timer.Stop()
			SafeUDPClients.ClientList.Remove(e)
			break
		}
	}
	SafeUDPClients.Mux.Unlock()
}

func HandleTCP(conn net.Conn) {
	var recvData NATLogEntry
	for {
		buffer := make([]byte, maxBufferSize)

		n, err := conn.Read(buffer)
		if err != nil {
			log.Printf("Error reading TCP connection %s, error: %s", conn.RemoteAddr().String(), err.Error())
			conn.Close()
			break
		}

		var retBuffer []byte
		retBuffer, recvData, err = HandleData(buffer[:n-1], "TCP", conn.RemoteAddr().String())
		if err != nil {
			log.Printf("HandleData Error: %s\nConnection to %s terminated.\n", err.Error(), conn.RemoteAddr().String())
			conn.Write(dcBuffer)
			conn.Close()
			break
		}

		_, err = conn.Write(retBuffer)
		if err != nil {
			log.Printf("TCP write to %s failed. Connection terminated\n", conn.RemoteAddr().String())
			recvData.Timeout = true
			writeLog <- recvData
			conn.Close()
			break
		}
		writeLog <- recvData
		log.Printf("TCP Packet sent to %s\n", conn.RemoteAddr().String())
	}
}

func AcceptUDP(pc net.PacketConn) {
	for {
		buffer := make([]byte, maxBufferSize)

		n, addr, err := pc.ReadFrom(buffer)
		if err != nil {
			log.Printf("Error reading UDP connection %s, error: %s", addr.String(), err.Error())
			continue
		}

		go HandleUDP(pc, addr, buffer[:n-1])
		go StopUDPClientTimeout(addr)
	}
}

func AcceptTCP(l net.Listener) {
	for {
		conn, err := l.Accept()
		if err != nil {
			continue
		}

		go HandleTCP(conn)
	}
}

func main() {
	log.SetFlags(0) // Do not prefix with date, this is handled by the operating system

	awsBucket := os.Getenv("AWS_BUCKET")
	awsRegion := os.Getenv("AWS_REGION")
	awsAccessKeyId := os.Getenv("AWS_ACCESS_KEY_ID")
	awsSecretAccessKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	if len(awsBucket) == 0 {
		log.Fatal("AWS_BUCKET not defined")
	}
	if len(awsRegion) == 0 {
		log.Fatal("AWS_REGION not defined")
	}
	if len(awsAccessKeyId) == 0 {
		log.Fatal("AWS_ACCESS_KEY_ID not defined")
	}
	if len(awsSecretAccessKey) == 0 {
		log.Fatal("AWS_SECRET_ACCESS_KEY not defined")
	}

	done := make(chan bool)
	writeLog = make(chan NATLogEntry)
	SafeUDPClients = UDPClientList{ClientList: list.New()}

	absPath, _ := filepath.Abs(schemaFile)
	absPath = "file:///" + strings.ReplaceAll(absPath, "\\", "/")
	schemaLoader = gojsonschema.NewReferenceLoader(absPath)

	pc, err := net.ListenPacket("udp", fmt.Sprintf(":%d", UDPport))
	if err != nil {
		log.Fatal(err)
	}

	l, err := net.Listen("tcp", fmt.Sprintf(":%d", TCPport))
	if err != nil {
		log.Fatal(err)
	}

	defer pc.Close()
	defer l.Close()

	go AcceptUDP(pc)
	go AcceptTCP(l)

	logPrefix := os.Getenv("LOG_PREFIX")
	go saveLog(awsBucket, logPrefix)

	log.Printf("NAT Test Server %s started.\n", Version)
	log.Printf("TCP Port:       %d\n", TCPport)
	log.Printf("UDP Port:       %d\n", UDPport)
	log.Printf("AWS Bucket:     %s\n", os.Getenv("AWS_BUCKET"))
	if len(logPrefix) > 0 {
		log.Printf("Log prefix:     %s\n", logPrefix)
	}
	log.Printf("AWS Region:     %s\n", os.Getenv("AWS_REGION"))
	log.Printf("AWS Access Key: %s\n", os.Getenv("AWS_ACCESS_KEY_ID"))

	<-done
}
