package main

import (
	"testing"
	"net"
	"os/exec"
	"os"
	"fmt"
	"time"
	"bytes"
	"strings"
)

const testInterval = 10
var testBuffer []byte = []byte("{\"proto\":\"Test\",\"op\":\"24201\",\"ip\":\"10.160.107.86\",\"cell_id\":21229824,\"mcc\":242,\"mnc\":1,\"ue_mode\":2,\"interval\":10}\n")
const threadCount = 3

var dcBuffer []byte = []byte("Error occured.\nConnection closed.\n")

func TestMain(m *testing.M) {
	err := os.Remove("./data.log")
	if err != nil {
		fmt.Printf("Failed to delete file")
		return
	}

	cmd := exec.Command("./server.exe")

	err = cmd.Start()
	if err != nil {
		fmt.Printf("Cannot start server")
		return
	}

	code := m.Run()

	if err := cmd.Process.Kill(); err != nil {
		fmt.Printf("Failed to kill server process")
		return
	}

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
		fmt.Printf("%s\n", testBuffer)
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
	} else if bytes.Compare(tempBuf[:n], dcBuffer) == 0 {
		conn.Close()
		t.Error("Wrong format in packet")
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
		fmt.Printf("%s\n", testBuffer)
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
	} else if bytes.Compare(tempBuf[:n], dcBuffer) == 0 {
		fmt.Printf(string(tempBuf[:n]))
		conn.Close()
		t.Error("Wrong format in packet")
	}
	doneChan<-true
}

func TestOutput(t *testing.T) {
	f, err := os.Open("data.log")
	if err != nil {
		t.Fatal("Unable to open file")
	}

	defer f.Close()

	buffer := make([]byte, 1024)
	_, err = f.Read(buffer) 
	if err != nil {
		t.Error("Could not read file")
	}

	if strings.Count(string(buffer), "\"proto\":\"Test\"") != 2*threadCount {
		t.Error("Wrong data written")
	}
}
