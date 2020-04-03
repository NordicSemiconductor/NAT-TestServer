package main

import (
	"fmt"
	"net"
	"time"
	"log"
	"encoding/json"
	"os"
)

const UDPport = ":3050"
const TCPport = ":3051"
const timeout = 10
const maxBufferSize = 256
var dcBuffer []byte = []byte("Error occured.\nConnection closed.\n")
var tasks []SaveStruct

type Packet struct{
	Protocol string
	Operator string
	IP string
	CellID string
	MCC int
	MNC int
	UEMode int
	Duration int
}

type SaveStruct struct {
	Received string
	Data Packet
}

func saveFunc(c chan SaveStruct){
	f, err := os.OpenFile("data.json", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}

	defer f.Close()

	for i := range c {
		if buffer, err := json.Marshal(i); err != nil {
			fmt.Printf("JSON invalid. Cannot write to file, error:%d\n", err)
		} else if _, err := f.Write(buffer); err != nil {
			f.Close() // ignore error; Write error takes precedence
			log.Fatal(err)
		}
	}
}

func handleUDP(pc net.PacketConn,addr net.Addr, buffer []byte, c chan SaveStruct){
	var packet Packet	

	startTime := time.Now().Format("2006-01-02 15:04:05")

	err := json.Unmarshal(buffer, &packet)
	if err != nil {
		_, err := pc.WriteTo(dcBuffer, addr)
		if err != nil {}
		fmt.Printf("temp\n");
		return
	}

	fmt.Printf("UDP Packet received from %s with duration %d\n", addr.String(), packet.Duration)

	saveStruct := SaveStruct{Received: startTime, Data: packet}

	c <- saveStruct

	time.Sleep(time.Duration(packet.Duration)*time.Second)

	endTime := time.Now().Format("2006-01-02 15:04:05")
	retString := "Duration: " + string(2) + "\nReceived:" + startTime + "\nReturned: " + endTime +"\n";
	retBuffer := []byte(retString)
	_, err = pc.WriteTo(retBuffer, addr)
	if err != nil {
		fmt.Printf("UDP write to %s failed, error: %s\n", addr.String(), err.Error())
		return
	}
	fmt.Printf("Packet sent to %s\n", addr.String())

}


func handleTCP(conn net.Conn, c chan SaveStruct){
	donechan := make(chan bool)
	first := true
	for {
		buffer := make([]byte, maxBufferSize)
				
		_, err := conn.Read(buffer)
		if err != nil {
			_, err = conn.Write(dcBuffer)
			if err != nil {}
			conn.Close()
			return
		}

		startTime := time.Now().Format("2006-01-02 15:04:05")
 
		if !first {
			donechan<-true
		}	else {
			first = false
		}

		var packet Packet
		err = json.Unmarshal(buffer, &packet)
		if err != nil {
			_, err = conn.Write(dcBuffer)
			if err != nil {}
			conn.Close()
			return
		}
	
		saveStruct := SaveStruct{Received: startTime, Data: packet}
	
		fmt.Printf("TCP Packet received from %s with duration %d\n", conn.RemoteAddr().String(), packet.Duration)
	
		c <- saveStruct

		time.Sleep(time.Duration(packet.Duration)*time.Second)
		
		endTime := time.Now().Format("2006-01-02 15:04:05");
		retString := "Duration: " + string(2) + "\nReceived:" + startTime + "\nReturned: " + endTime +"\n";
		retBuffer := []byte(retString)
		
		_, err = conn.Write(retBuffer)
		if err != nil {
			fmt.Printf("TCP write to %s failed, error: %s\n", conn.RemoteAddr().String(), err.Error())
			conn.Close()
			return
		}
		fmt.Printf("Packet sent to %s\n", conn.RemoteAddr().String())

		timer := time.NewTimer(timeout * time.Second);
		go func() {
			fmt.Printf("Waiting...\n")
			select{
				case <-timer.C:
					fmt.Printf("Connection to %s terminated after %d seconds.\n", conn.RemoteAddr().String(), packet.Duration)
					conn.Close()
				case <-donechan:
					timer.Stop()
					return
			}
		}()
	}
}

func acceptUDP(pc net.PacketConn, c chan SaveStruct){
	for {
		buffer := make([]byte, maxBufferSize)

		n, addr, err := pc.ReadFrom(buffer)
		if err != nil {
			continue
		}

		go handleUDP(pc, addr, buffer[:n], c)
	}
}

func acceptTCP(l net.Listener, c chan SaveStruct){
	for {
		conn, err := l.Accept()
		if err != nil {
			continue
		}
		
		go handleTCP(conn, c)
	}
}

func main(){
	done := make(chan bool)
	saveChan := make(chan SaveStruct)

	pc, err := net.ListenPacket("udp", UDPport)
	if err != nil {
		log.Fatal(err)
	}

    l, err := net.Listen("tcp", TCPport)
    if err != nil {
        log.Fatal(err)
	}

	defer pc.Close()
	defer l.Close()
	
	go acceptUDP(pc, saveChan)
	go acceptTCP(l, saveChan)

	go saveFunc(saveChan)

	<-done
}
