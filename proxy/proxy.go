package proxy

import (
	"bufio"
	"fmt"
	"gowebproxy/cache"
	"gowebproxy/info"
	"gowebproxy/log"
	"gowebproxy/parser"
	"net"
	"strconv"
	"strings"
	"time"
)

func isCacheable(control string) bool {
	if strings.Contains(control, "no-store") ||
		strings.Contains(control, "private") ||
		strings.Contains(control, "no-cache") ||
		strings.Contains(control, "must-revalidate") {
		return false
	}

	return true
}

func getExpiresTime(response *parser.HttpResponse) time.Time {
	if value, ok := response.Headers["Cache-Control"]; ok {
		if strings.Contains(value, "max-age") {
			ind := strings.Index(value, "=")
			str := value[ind+1:]

			if str[len(str)-1] == ',' {
				str = str[:len(str)-1]
			}

			seconds, err := strconv.Atoi(str)

			if err == nil {
				return time.Now().Add(time.Second * time.Duration(seconds))
			}
		}
	}

	if value, ok := response.Headers["Expires"]; ok {
		t, err := time.Parse("Wed, 21 Oct 2015 07:28:00 GMT", value)

		if err == nil {
			return t
		}
	}

	return time.Now()
}

func isExpired(response *parser.HttpResponse) bool {
	expiresTime := response.ExpiresTime
	now := time.Now()

	// expiresTime < now
	return expiresTime.Before(now)
}

func ProxyWebServer(port int, statsChan chan info.Stats) {
	host := ":" + strconv.Itoa(port)
	// cria socket tcp na porta port
	listen, err := net.Listen("tcp", host)

	if err != nil {
		log.PrintError(err)
		return
	}

	defer listen.Close()

	fmt.Printf("Web Proxy listening in port %d\n", port)

	// enviando informação de inicio de execução
	statsChan <- info.Stats{StartTime: time.Now()}

	var connCount = 0
	cache := cache.NewCache()

	for {
		// loop infinito esperando por conexoes
		conn, err := listen.Accept()

		if err != nil {
			// se ocorrer um erro, imprimir e esperar por novas conexoes
			log.PrintError(err)
		} else {
			// se nao houver erro, tratar conexao em outra goroutine
			go handler(connCount, conn, statsChan, &cache)
			connCount++
		}
	}
}

func handler(connId int, conn net.Conn, statsChan chan info.Stats, cache *cache.Cache) {
	defer conn.Close()

	statsChan <- info.Stats{ActiveConn: 1}

	clientHostAddr := conn.RemoteAddr().String()

	log.LogInfo(connId, "Connection from %s\n", clientHostAddr)

	// criando leitor de mensagens da conexao
	var reader = bufio.NewReader(conn)
	var writer = bufio.NewWriter(conn)

	var serverConn net.Conn
	var serverReader *bufio.Reader
	var serverWriter *bufio.Writer

	// loop de leitura de mensagens
OUTERLOOP:
	for {
		request, err := parser.NewHttpRequest(reader)

		if err != nil {
			log.LogInfo(connId, "Error in parse HTTP request: %v\n", err)
			break OUTERLOOP
		}

		host, ok := request.Headers["Host"]

		if ok == false {
			log.LogInfo(connId, "Host do not exist, get URI %s\n", request.URI)
			break OUTERLOOP
		}

		cacheControl := request.Headers["Cache-Control"]
		log.LogInfo(connId, "Client request Cache-Control: %s\n", cacheControl)

		// verificar se cache para esta request existe
		response, ok := cache.Get(request.Method, request.URI)

		// se nao for autorizado cache pelo cliente ou
		// nao existe cache ou
		// o cache foi expirado
		if isCacheable(cacheControl) == false || ok == false || isExpired(&response) {
			log.LogInfo(connId, "Resource %s not found in cache.\n", request.URI)

			// nao foi encontrado cache
			// cria conexao com servidor
			serverConn, err = net.Dial("tcp", host+":80")

			if err != nil {
				log.LogInfo(connId, "Error when trying to connect to host server %s: %v\n", host, err)
				break OUTERLOOP
			}

			serverReader = bufio.NewReader(serverConn)
			serverWriter = bufio.NewWriter(serverConn)

			// faz requisicao a host server
			log.LogInfo(connId, "Requesting to host %s the resource %s\n", host, request.URI)

			// enviando requisicao http para o host server
			parser.WriteHttpRequest(serverWriter, &request)

			log.LogInfo(connId, "Processing host %s http response\n", host)

			response, err = parser.NewHttpResponse(serverReader)

			if err != nil {
				log.LogInfo(connId, "(Host: %s) Error on parse HTTP response: %v\n", host, err)
				break OUTERLOOP
			}

			// por enquanto, sempre fechar conexao com servidor
			serverConn.Close()
			serverConn = nil

			serverCacheControl := response.Headers["Cache-Control"]
			log.LogInfo(connId, "Server response Cache-Control: %s\n", serverCacheControl)

			if isCacheable(cacheControl) && isCacheable(serverCacheControl) {
				response.ExpiresTime = getExpiresTime(&response)
				log.LogInfo(connId, "Storing HTTP response from %s in cache with expires time %v\n", host, response.ExpiresTime)
				cache.Set(request.Method, request.URI, response)
			}

		} else {
			log.LogInfo(connId, "Resource %s FOUND in cache.\n", request.URI)
		}

		// enviando corpo de resposta http do servidor (ou cache disp.) para o cliente do proxy
		parser.WriteHttpResponse(writer, &response)

		contentLengthStr, ok := response.Headers["Content-Length"]
		contentLength := 0
		if ok {
			contentLength, err = strconv.Atoi(contentLengthStr)
			if err != nil {
				log.LogInfo(connId, "Error: Content-Length is not numeric.\n")
				contentLength = 0
			}
		}

		statsChan <- info.Stats{
			LastHostsVisited:    []string{host},
			LastResourceVisited: []info.Resource{{request.URI, contentLength}},
		}

		// decide se mantem conexao com cliente proxy
		if connValue, ok := response.Headers["Connection"]; ok && connValue == "close" {
			break OUTERLOOP
		} else {
			log.LogInfo(connId, "Keeping connection with %s\n", clientHostAddr)
		}
	}

	log.LogInfo(connId, "Closing connection with %s\n", clientHostAddr)

	statsChan <- info.Stats{ActiveConn: -1}
}
