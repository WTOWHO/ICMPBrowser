package sender

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv6"
)

var socketLock sync.Mutex

func SendMessage(c *icmp.PacketConn, msg string, dstIP string) (string, error) {
	socketLock.Lock()
	defer socketLock.Unlock()

	mySessionID := rand.Intn(65535)

	wm := icmp.Message{
		Type: ipv6.ICMPTypeEchoRequest,
		Code: 0,
		Body: &icmp.Echo{
			ID:   mySessionID,
			Seq:  1,
			Data: []byte(msg),
		},
	}

	wb, err := wm.Marshal(nil)
	if err != nil {
		log.Fatal(err)
	}

	if _, err := c.WriteTo(wb, &net.IPAddr{IP: net.ParseIP(dstIP)}); err != nil {
		log.Fatal(err)
	}

	receivedChunks := make(map[int][]byte)
	expectedTotal := -1

	for {
		c.SetReadDeadline(time.Now().Add(5 * time.Second))

		responseData := make([]byte, 1500)
		n, _, err := c.ReadFrom(responseData)
		if err != nil {
			return "", fmt.Errorf("A conexão expirou (Timeout) ou falhou: %s", err)
		}

		responseData = responseData[:n]

		if responseData[0] == 129 {
			parsedMsg, err := icmp.ParseMessage(58, responseData)
			if err != nil {
				log.Printf("Error parsing icmp message: %s", err)
				continue
			}

			echo, ok := parsedMsg.Body.(*icmp.Echo)
			if !ok || len(echo.Data) < 4 {
				continue
			}

			if echo.ID != mySessionID {
				continue
			}

			if msg == string(echo.Data) {
				continue
			}

			if expectedTotal == -1 {
				expectedTotal = int(binary.BigEndian.Uint32(echo.Data[0:4]))
				log.Printf("Total chunks expected: %d", expectedTotal)
			}

			currentSeq := echo.Seq

			if _, exists := receivedChunks[currentSeq]; !exists {
				data := make([]byte, len(echo.Data)-4)
				copy(data, echo.Data[4:])

				receivedChunks[currentSeq] = data
				log.Printf("Received packet %d of %d", len(receivedChunks), expectedTotal)

				if len(receivedChunks) == expectedTotal {
					break
				}
			}
		}
	}

	var htmlData bytes.Buffer
	for i := 1; i <= expectedTotal; i++ {
		htmlData.Write(receivedChunks[i])
	}
	log.Printf("Assembly finished! HTML size: %d bytes", htmlData.Len())
	return htmlData.String(), nil
}
