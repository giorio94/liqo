package main

import "net"

func main() {
	f, _ := New(&net.UDPAddr{Port: 8080}, []*net.UDPAddr{
		{IP: net.IPv4(169, 254, 4, 1), Port: 8080},
		{IP: net.IPv4(169, 254, 5, 1), Port: 8080},
	})

	f.start()
}
