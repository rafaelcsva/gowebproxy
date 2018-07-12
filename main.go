package main

import "fmt"
import "os"
import "os/signal"

func main() {
	const webProxyPort = 54321
	const infoPort = 54322

	// inicia o servidor web proxy na porta 54321
	go ProxyWebServer(webProxyPort)

	// inicia o servidor de infos na porta 54322
	go InfoServer(infoPort)

	// capturar Ctrl-C (Interrupt Signal)
	// para encerrar o programa

	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt)

	// bloqueia para manter as goroutines executando
	<-interruptChan
	fmt.Println("Server terminated.")
}
