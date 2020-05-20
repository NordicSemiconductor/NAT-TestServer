package main

import (
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

type deviceMessage struct {
	Operator  string   `json:"op"`
	IP        []string `json:"ip"`
	CellID    int      `json:"cell_id"`
	UEMode    int      `json:"ue_mode"`
	LTEMode   int      `json:"lte_mode"`
	NBIotMode int      `json:"nbiot_mode"`
	ICCID     string   `json:"iccid"`
	IMEI      string   `json:"imei"`
	Interval  int      `json:"interval"`
}

type atMessage struct {
	Operator string `json:"op"`
	ICCID    string `json:"iccid"`
	IMEI     string `json:"imei"`
	Cmd      string `json:"cmd"`
	Result   string `json:"result"`
}

type logEntry interface {
	getKey() string
}

// ATLogEntry gets logged to S3
type ATLogEntry struct {
	IP            string
	Timestamp     time.Time
	Message       atMessage
	ServerVersion string
	TraceID       string
}

// NATLogEntry gets logged to S3
type NATLogEntry struct {
	Protocol      string
	IP            string
	Timeout       bool
	Timestamp     time.Time
	Message       deviceMessage
	ServerVersion string
	TraceID       string
}

type udpClientTimeout struct {
	Timeout *time.Timer
	Log     NATLogEntry
}

type udpClientTimeoutMap struct {
	Map map[string]udpClientTimeout
	Mux sync.Mutex
}

var udpPort = 3050
var tcpPort = 3051
var atPort = 3060
var version = "0.0.0-development"

const newUDPMessageTimeoutInSeconds = 60
const maxBufferSize = 256
const natSchemaFile = "nat_schema.json"
const atSchemaFile = "at_schema.json"
const timeFormat = "2006-01-02T15:04:05.00-0700"

var genericErrorMessage []byte = []byte(fmt.Sprintf("Error occured.\nConnection closed.\nVersion: %s\n", version))
var writeLog chan logEntry
var natSchemaLoader gojsonschema.JSONLoader
var atSchemaLoader gojsonschema.JSONLoader

// updClientTimeouts stores timers to wait for UDP client responses
var updClientTimeouts udpClientTimeoutMap

func (e NATLogEntry) getKey() string {
	return fmt.Sprintf("%s/%s/%s-%s-%s.json", "NATLog", e.Timestamp.Format("2006/01/02/15"), e.IP, e.Timestamp.Format("150405"), e.TraceID)
}

func (e ATLogEntry) getKey() string {
	return fmt.Sprintf("%s/%s/%s-%s-%s.json", "ATLog", e.Timestamp.Format("2006/01/02/15"), e.IP, e.Timestamp.Format("150405"), e.TraceID)
}

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

		key := i.getKey()
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

// HandleAT Handle AT cmd messages
func handleAT(conn net.Conn) {
	for {
		buffer := make([]byte, maxBufferSize)

		n, err := conn.Read(buffer)
		if err != nil {
			conn.Close()
			log.Printf("Error reading TCP connection %s, error: %s", conn.RemoteAddr().String(), err.Error())
			break
		}
		timestamp := time.Now()

		traceID, err := uuid.NewRandom()
		if err != nil {
			log.Printf("Failed to create new UUID: %d\n", err)
			conn.Write(genericErrorMessage)
			conn.Close()
			break
		}

		documentLoader := gojsonschema.NewStringLoader(string(buffer))
		result, err := gojsonschema.Validate(atSchemaLoader, documentLoader)
		if err != nil {
			log.Printf("JSON validation error: %d\nConnection to %s terminated.\n", err, conn.RemoteAddr().String())
			conn.Write(genericErrorMessage)
			conn.Close()
			break
		} else if !result.Valid() {
			log.Printf("Invalid AT-cmd JSON format.\nConnection to %s terminated.\n", conn.RemoteAddr().String())
			conn.Write(genericErrorMessage)
			conn.Close()
			break
		}

		var message atMessage
		err = json.Unmarshal(buffer[:n-1], &message)
		if err != nil {
			log.Printf("Failed to unmarshal JSON, error: %d.\n", err)
			conn.Write(genericErrorMessage)
			conn.Close()
			break
		}

		log.Printf("AT-cmd Message received from %s\n", conn.RemoteAddr().String())

		saveData := ATLogEntry{
			IP:            conn.RemoteAddr().String(),
			Timestamp:     timestamp,
			Message:       message,
			ServerVersion: version,
			TraceID:       traceID.String(),
		}

		writeLog <- saveData

		retString := fmt.Sprintf(
			"AT-cmd message received.\nVersion:  %s\nTraceID:  %s\n", version, traceID,
		)
		conn.Write([]byte(retString))
	}
}

// HandleData read incoming data from the handed buffer and pause execution based on the requested interval
func HandleData(buffer []byte, protocol string, addr string) ([]byte, NATLogEntry, error) {
	timestamp := time.Now()

	traceID, err := uuid.NewRandom()
	if err != nil {
		log.Printf("Failed to create new UUID: %d\n", err)
		return nil, NATLogEntry{}, err
	}

	documentLoader := gojsonschema.NewStringLoader(string(buffer))
	result, err := gojsonschema.Validate(natSchemaLoader, documentLoader)
	if err != nil {
		return nil, NATLogEntry{}, err
	} else if !result.Valid() {
		return nil, NATLogEntry{}, errors.New("Message uses wrong format")
	}

	var message deviceMessage
	err = json.Unmarshal(buffer, &message)
	if err != nil {
		return nil, NATLogEntry{}, err
	}

	log.Printf("[%s] %s Message received from %s: interval %s\n", traceID, protocol, addr, strconv.Itoa(message.Interval))

	time.Sleep(time.Duration(message.Interval) * time.Second)

	endTime := time.Now().Format(timeFormat)

	retString := fmt.Sprintf(
		"Interval: %s\nReturned: %s\nVersion:  %s\nTraceID:  %s\n", strconv.Itoa(message.Interval), endTime, version, traceID,
	)
	saveData := NATLogEntry{
		Timestamp:     timestamp,
		Protocol:      protocol,
		IP:            addr,
		Timeout:       false,
		Message:       message,
		ServerVersion: version,
		TraceID:       traceID.String(),
	}
	return []byte(retString), saveData, nil
}

// handleUDP handle UDP messages.
// Timeouts are detected by waiting for a client to send a new message withing 60 seconds after having sent the delayed response.
func handleUDP(pc net.PacketConn, addr net.Addr, buffer []byte) {
	retBuffer, logEntry, err := HandleData(buffer, "UDP", addr.String())
	if err != nil {
		log.Printf("HandleData Error: %s\nConnection to %s terminated.\n", err.Error(), addr.String())
		pc.WriteTo(genericErrorMessage, addr)
		return
	}

	updClientTimeouts.Mux.Lock()
	v, ok := updClientTimeouts.Map[addr.String()]
	delete(updClientTimeouts.Map, addr.String())
	updClientTimeouts.Mux.Unlock()
	if ok {
		v.Timeout.Stop()
		if logEntry.Message.Interval <= v.Log.Message.Interval {
			// The device did not receive our response and now starts with the binary search
			// so it will send a message, but with a lower interval
			v.Log.Timeout = true
		}
		writeLog <- v.Log
	}

	_, err = pc.WriteTo(retBuffer, addr)
	if err != nil {
		log.Printf("[%s] UDP write to %s failed, error: %s\n", logEntry.TraceID, addr.String(), err.Error())
		return
	}
	log.Printf("[%s] UDP Packet sent to %s. Interval: %s.\n", logEntry.TraceID, addr.String(), strconv.Itoa(logEntry.Message.Interval))

	timer := time.NewTimer(newUDPMessageTimeoutInSeconds * time.Second)
	updClientTimeouts.Mux.Lock()
	updClientTimeouts.Map[addr.String()] = udpClientTimeout{Timeout: timer, Log: logEntry}
	updClientTimeouts.Mux.Unlock()
	select {
	case <-timer.C:
		updClientTimeouts.Mux.Lock()
		delete(updClientTimeouts.Map, addr.String())
		updClientTimeouts.Mux.Unlock()
		log.Printf("[%s] UDP connection to %s timed out. Connection terminated. Interval: %s.\n", logEntry.TraceID, addr, strconv.Itoa(logEntry.Message.Interval))
		logEntry.Timeout = true
		writeLog <- logEntry
	}
}

// handleTCP handle TCP messages.
// Timouts are detected by checking for successfull TCP writes.
func handleTCP(conn net.Conn) {
	var logEntry NATLogEntry
	for {
		buffer := make([]byte, maxBufferSize)

		n, err := conn.Read(buffer)
		if err != nil {
			conn.Close()
			if logEntry.Protocol != "" {
				log.Printf("[%s] Error reading TCP connection %s, error: %s. Interval: %s.", logEntry.TraceID, conn.RemoteAddr().String(), err.Error(), strconv.Itoa(logEntry.Message.Interval))
				// Store log from previous interval
				logEntry.Timeout = true
				writeLog <- logEntry
			} else {
				log.Printf("Error reading TCP connection %s, error: %s", conn.RemoteAddr().String(), err.Error())
			}
			break
		}
		if logEntry.Protocol != "" {
			// Store log from previous interval
			writeLog <- logEntry
		}

		var retBuffer []byte
		retBuffer, logEntry, err = HandleData(buffer[:n-1], "TCP", conn.RemoteAddr().String())
		if err != nil {
			log.Printf("HandleData Error: %s\nConnection to %s terminated.\n", err.Error(), conn.RemoteAddr().String())
			conn.Write(genericErrorMessage)
			conn.Close()
			break
		}

		_, err = conn.Write(retBuffer)
		if err != nil {
			log.Printf("[%s] TCP write to %s failed. Connection terminated. Interval: %s.\n", logEntry.TraceID, conn.RemoteAddr().String(), strconv.Itoa(logEntry.Message.Interval))
			logEntry.Timeout = true
			writeLog <- logEntry
			conn.Close()
			break
		}
		log.Printf("[%s] TCP Packet sent to %s. Interval: %s.\n", logEntry.TraceID, conn.RemoteAddr().String(), strconv.Itoa(logEntry.Message.Interval))
	}
}

func acceptUDP(pc net.PacketConn) {
	for {
		buffer := make([]byte, maxBufferSize)

		n, addr, err := pc.ReadFrom(buffer)
		if err != nil {
			log.Printf("Error reading UDP connection %s, error: %s", addr.String(), err.Error())
			continue
		}

		go handleUDP(pc, addr, buffer[:n-1])
	}
}

func acceptTCP(l net.Listener) {
	for {
		conn, err := l.Accept()
		if err != nil {
			continue
		}

		go handleTCP(conn)
	}
}

func acceptAT(l net.Listener) {
	for {
		conn, err := l.Accept()
		if err != nil {
			continue
		}

		go handleAT(conn)
	}
}

func main() {
	log.SetFlags(0) // Do not prefix with date, this is handled by the operating system

	awsBucket := os.Getenv("AWS_BUCKET")
	awsRegion := os.Getenv("AWS_REGION")
	awsAccessKeyID := os.Getenv("AWS_ACCESS_KEY_ID")
	awsSecretAccessKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	if len(awsBucket) == 0 {
		log.Fatal("AWS_BUCKET not defined")
	}
	if len(awsRegion) == 0 {
		log.Fatal("AWS_REGION not defined")
	}
	if len(awsAccessKeyID) == 0 {
		log.Fatal("AWS_ACCESS_KEY_ID not defined")
	}
	if len(awsSecretAccessKey) == 0 {
		log.Fatal("AWS_SECRET_ACCESS_KEY not defined")
	}

	done := make(chan bool)
	writeLog = make(chan logEntry)
	updClientTimeouts = udpClientTimeoutMap{Map: make(map[string]udpClientTimeout)}

	// Initialize the schema loaders
	absPath, err := filepath.Abs(natSchemaFile)
	if err != nil {
		log.Fatal(err)
	}
	natSchemaLoader = gojsonschema.NewReferenceLoader(fmt.Sprintf("file://%s", absPath))

	absPath, err = filepath.Abs(atSchemaFile)
	if err != nil {
		log.Fatal(err)
	}
	atSchemaLoader = gojsonschema.NewReferenceLoader(fmt.Sprintf("file://%s", absPath))

	// Start listening on ports
	pc, err := net.ListenPacket("udp", fmt.Sprintf(":%d", udpPort))
	if err != nil {
		log.Fatal(err)
	}

	l, err := net.Listen("tcp", fmt.Sprintf(":%d", tcpPort))
	if err != nil {
		log.Fatal(err)
	}

	atL, err := net.Listen("tcp", fmt.Sprintf(":%d", atPort))
	if err != nil {
		log.Fatal(err)
	}

	defer pc.Close()
	defer l.Close()
	defer atL.Close()

	go acceptUDP(pc)
	go acceptTCP(l)
	go acceptAT(atL)

	logPrefix := os.Getenv("LOG_PREFIX")
	go saveLog(awsBucket, logPrefix)

	log.Printf("NAT Test Server %s started.\n", version)
	log.Printf("TCP Port:       %d\n", tcpPort)
	log.Printf("UDP Port:       %d\n", udpPort)
	log.Printf("AWS Bucket:     %s\n", os.Getenv("AWS_BUCKET"))
	if len(logPrefix) > 0 {
		log.Printf("Log prefix:     %s\n", logPrefix)
	}
	log.Printf("AWS Region:     %s\n", os.Getenv("AWS_REGION"))
	log.Printf("AWS Access Key: %s\n", os.Getenv("AWS_ACCESS_KEY_ID"))

	<-done
}
