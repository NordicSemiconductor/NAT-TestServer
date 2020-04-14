package main

import (
	"testing"
	"net"
	"os"
	"time"
	"bytes"
	"strings"
	"io/ioutil"
	"log"
)

const testInterval = 10
var testBuffer []byte = []byte("{\"op\":\"24201\",\"ip\":\"10.160.73.64\",\"cell_id\":21229824,\"ue_mode\":2,\"iccid\":\"8931089318104314834F\",\"interval\":10}\n")

const threadCount = 3

func TestMain(m *testing.M) {
	err := os.Remove("./data.log")
	if err != nil {}

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

func TestOutput(t *testing.T) {
	f, err := os.Open("./data.log")
	if err != nil {
		t.Fatal("Unable to open file")
	}

	defer f.Close()

	buffer := make([]byte, 1024)
	_, err = f.Read(buffer) 
	if err != nil {
		t.Error("Could not read file")
	}

	if strings.Count(string(buffer), "\"ip\":\"10.160.73.64\"") != 2*threadCount {
		t.Error("Wrong data written")
	}
}
