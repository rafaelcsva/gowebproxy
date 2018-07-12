package info

import (
	"strconv"
	"time"
	"net"
	"fmt"
	"gowebproxy/log"
)

type Stats struct {
	LastHostsVisited []string
	LastResourceVisited []string
	CountActiveConn int
	StartTime time.Time
}

func handler(conn net.Conn, statChan chan Stats){
	// espera por uma resposta do servidor proxy

	
}

func InfoServer(port int, statChan chan Stats) {
	host := ":" + strconv.Itoa(port)
	// cria socket tcp na porta port
	listen, err := net.Listen("tcp", host)

	if err != nil {

		return
	}

	defer listen.Close()

	fmt.Printf("Information Server listening in port %d\n", port)

	for {
		// loop infinito esperando por conexoes
		conn, err := listen.Accept()

		if err != nil {
			// se ocorrer um erro, imprimir e esperar por novas conexoes
			log.PrintError(err)
		} else {
			// se nao houver erro, tratar conexao em outra goroutine
			go handler(conn)
		}
	}	
}
