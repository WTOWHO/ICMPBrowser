package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv6"
)

type PingPayload struct {
	SessionID int
	SeqNum    int
	Total     uint32
	Data      []byte
}

func (p *PingPayload) Send(c *icmp.PacketConn, addr net.Addr) {
	bufSize := 4 + len(p.Data)
	dataBuf := make([]byte, bufSize)
	binary.BigEndian.PutUint32(dataBuf[0:4], p.Total)
	copy(dataBuf[4:], p.Data)
	wm := icmp.Message{
		Type: ipv6.ICMPTypeEchoReply, Code: 0,
		Body: &icmp.Echo{
			ID: p.SessionID, Seq: p.SeqNum,
			Data: dataBuf,
		},
	}
	wb, err := wm.Marshal(nil)
	if err != nil {
		log.Fatal(err)
	}
	if _, err := c.WriteTo(wb, addr); err != nil {
		log.Printf("Error sending ICMP Echo: %v", err)
		return
	}
	log.Printf("Sent ICMP Echo Num: %d", p.SeqNum)
}

func ParseMessage(n int, b []byte) (string, int, error) {
	m, err := icmp.ParseMessage(58, b[:n])
	if err != nil {
		return "", 0, err
	}

	switch body := m.Body.(type) {
	case *icmp.Echo:
		data := string(body.Data)
		return data, body.ID, nil
	}
	return "", 0, fmt.Errorf("Unknown icmp message")
}

func GetURL(url string) ([]byte, error) {
	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(res.Body)
	res.Body.Close()

	if res.StatusCode > 299 {
		return nil, fmt.Errorf("Response failed with status code: %d and\nbody: %s\n", res.StatusCode, body)
	}
	if err != nil {
		return nil, err
	}

	return body, nil

}

func isValidURL(str string) bool {
	u, err := url.ParseRequestURI(str)
	if err != nil {
		return false
	}
	return u.Scheme != "" && u.Host != ""
}

func main() {
	ipFlag := flag.String("ip", "::1", "Endereço IPv6 para rodar o proxy")
	flag.Parse()

	localAddr := *ipFlag
	c, err := icmp.ListenPacket("ip6:58", localAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	b := make([]byte, 65535)
	log.Println("READING")
	for {
		n, addr, err := c.ReadFrom(b)
		if err != nil {
			log.Panicf("Error reading payload from %s: %s", addr.String(), err)
		}

		if b[0] == 128 {
			data, sessionID, err := ParseMessage(n, b)
			if err != nil {
				log.Printf("Error parsing ICMP message: %s", err)
				continue
			}

			log.Printf("Parsed ICMP Message: %s", data)

			isValid := isValidURL(data)
			if !isValid {
				log.Printf("Expected a link but received: %s", data)
				continue
			}

			site, err := GetURL(data)
			if err != nil {
				log.Printf("Error in getting URL: %s", err)
				continue
			}

			totalPackets := len(site) / 1024
			if len(site)%1024 != 0 {
				totalPackets++
			}
			if totalPackets == 0 {
				totalPackets = 1
			}

			for i := 0; i < totalPackets; i++ {
				chunckSize := 1024

				if len(site) < chunckSize {
					chunckSize = len(site)
				}

				packet := PingPayload{
					SeqNum:    i + 1,
					SessionID: sessionID,
					Data:      site[:chunckSize],
					Total:     uint32(totalPackets),
				}

				site = site[chunckSize:]

				packet.Send(c, addr)

				time.Sleep(30 * time.Millisecond)

			}
		} else {
			log.Println("Unknown ICMP message")
		}

	}
}
