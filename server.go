package main

import (
	"log"
	"net"
	"time"
	"encoding/json"
	"os"
	"strconv"
)

type Packet struct{
	Operator string `json:"op"`
	IP string `json:"ip"`
	CellId int `json:"cell_id"`
	UEMode int `json:"ue_mode"`
	ICCID string `json:"iccid"`
	Interval int `json:"interval"`
}

type SaveStruct struct {
	Received string
	Protocol string
	Data Packet
}

const UDPport = ":3050"
const TCPport = ":3051"
const timeout = 10
const maxBufferSize = 256
var dcBuffer []byte = []byte("Error occured.\nConnection closed.\n")
var saveChan chan SaveStruct

func SaveFunc(){
	f, err := os.OpenFile("data.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}

	defer f.Close()

	for i := range saveChan {
		if buffer, err := json.Marshal(i); err != nil {
			log.Printf("JSON invalid. Cannot write to file, error:%d\n", err)
		} else if _, err := f.Write(append(buffer, '\n')); err != nil {
			f.Close() // ignore error; Write error takes precedence
			log.Fatal(err)
		}
	}
}

func HandleData(buffer []byte, protocol string) ([]byte, error) {
	startTime := time.Now().Format("2006-01-02 15:04:05.000 MST")

	var packet Packet
	err := json.Unmarshal(buffer, &packet)
	if err != nil {
		return nil, err
	}

	saveStruct := SaveStruct{Received: startTime, Protocol: protocol, Data: packet}

	saveChan <- saveStruct

	time.Sleep(time.Duration(packet.Interval)*time.Second)

	endTime := time.Now().Format("2006-01-02 15:04:05.000 MST")
	retString := "Interval: " + strconv.Itoa(packet.Interval) + "\nReceived:" + startTime + "\nReturned: " + endTime +"\n";
	return []byte(retString), nil
}

func HandleUDP(pc net.PacketConn,addr net.Addr, buffer []byte){
	log.Printf("UDP Packet received from %s\n", addr.String())

	retBuffer, err := HandleData(buffer, "UDP")
	if err != nil {
		_, err = pc.WriteTo(dcBuffer, addr)
		if err != nil {}
		return
	}

	_, err = pc.WriteTo(retBuffer, addr)
	if err != nil {
		log.Printf("UDP write to %s failed, error: %s\n", addr.String(), err.Error())
		return
	}
	log.Printf("Packet sent to %s\n", addr.String())

}


func HandleTCP(conn net.Conn){
	doneChan := make(chan bool)
	first := true
	for {
		buffer := make([]byte, maxBufferSize)
		
		log.Printf("TCP packet received from %s\n", conn.RemoteAddr().String())

		n, err := conn.Read(buffer)
		if err != nil {
			_, err = conn.Write(dcBuffer)
			if err != nil {}
			conn.Close()
			return
		}
 
		if !first {
			doneChan<-true
		}	else {
			first = false
		}

		retBuffer, err := HandleData(buffer[:n-1], "TCP")
		if err != nil {
			_, err = conn.Write(dcBuffer)
			if err != nil {}
			conn.Close()
			return
		}
		
		_, err = conn.Write(retBuffer)
		if err != nil {
			log.Printf("TCP write to %s failed, error: %s\n", conn.RemoteAddr().String(), err.Error())
			conn.Close()
			return
		}
		log.Printf("Packet sent to %s\n", conn.RemoteAddr().String())

		timer := time.NewTimer(timeout * time.Second);
		go func() {
			log.Printf("Waiting...\n")
			select{
				case <-timer.C:
					log.Printf("Connection to %s terminated.\n", conn.RemoteAddr().String())
					conn.Close()
				case <-doneChan:
					timer.Stop()
					return
			}
		}()
	}
}

func AcceptUDP(pc net.PacketConn){
	for {
		buffer := make([]byte, maxBufferSize)

		n, addr, err := pc.ReadFrom(buffer)
		if err != nil {
			continue
		}

		go HandleUDP(pc, addr, buffer[:n-1])
	}
}

func AcceptTCP(l net.Listener){
	for {
		conn, err := l.Accept()
		if err != nil {
			continue
		}
		
		go HandleTCP(conn)
	}
}

func main(){
	done := make(chan bool)
	saveChan = make(chan SaveStruct)

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
	
	go AcceptUDP(pc)
	go AcceptTCP(l)
	go SaveFunc()

	<-done
}
