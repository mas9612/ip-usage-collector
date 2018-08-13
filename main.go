package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"net"
	"os"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

type echoReply struct {
	Data    []byte
	Address net.Addr
	Bytes   int
}

func main() {
	args := os.Args
	if len(args) < 2 {
		fmt.Printf("Usage: %s TARGET\n", args[0])
		fmt.Printf("Example: %s 192.168.0.0/24\n", args[0])
		os.Exit(1)
	}

	targetNet := args[1]
	_, v4net, err := net.ParseCIDR(targetNet)
	if err != nil {
		log.Fatalln(err)
	}
	networkAddress := binary.BigEndian.Uint32(v4net.IP)
	networkMask := binary.BigEndian.Uint32(v4net.Mask)
	naddr := uint32(math.MaxUint32) - networkMask

	packet := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID:   os.Getpid() & 0xffff,
			Seq:  1,
			Data: []byte("test"),
		},
	}
	pb, err := packet.Marshal(nil)
	if err != nil {
		log.Fatalln(err)
	}

	for {
		c := make(chan []string)
		go run(networkAddress, naddr, pb, c)

		select {
		case upHosts := <-c:
			fmt.Println(upHosts)
			break
		}

		time.Sleep(time.Second * 600)
	}
}

func run(networkAddress, naddr uint32, pb []byte, hosts chan []string) {
	upHosts := make([]string, naddr)
	upCount := 0

	// except first and last address (network address and broadcast address)
	for i := uint32(1); i < naddr; i++ {
		addr := make([]byte, 4)
		binary.BigEndian.PutUint32(addr, networkAddress+i)
		host := net.IPv4(addr[0], addr[1], addr[2], addr[3])

		done := false
		timeout := time.NewTicker(time.Millisecond * 500)
		reply := make(chan echoReply)
		go ping(host, pb, reply)

		for {
			select {
			case <-timeout.C:
				done = true
				break
			case recv := <-reply:
				upHosts[upCount] = recv.Address.String()
				upCount++
				done = true
				break
			}

			if done {
				break
			}
		}
	}

	hosts <- upHosts[:upCount]
}

func ping(host net.IP, data []byte, echo chan echoReply) {
	log.Println("pinging", host)
	c, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		log.Fatalln(err)
	}
	defer c.Close()

	_, err = c.WriteTo(data, &net.IPAddr{IP: host})
	if err != nil {
		log.Fatalln(err)
	}

	rb := make([]byte, 1500)
	n, peer, err := c.ReadFrom(rb)
	if err != nil {
		log.Println("aiueo")
		log.Fatalln(err)
	}

	echo <- echoReply{Data: rb, Address: peer, Bytes: n}
}
