package mimicry

import (
	"net"
)

type Config struct {
	IPAddr  net.IP
	UDPPort int
	TCPPort int
	PrivKey string
}
