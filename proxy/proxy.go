package proxy

import (
	"bufio"
	"errors"
	"fmt"
	"gowebproxy/info"
	"gowebproxy/log"
	"io"
	"net"
	"strconv"
	"time"
)

type HttpRequest struct {
	Method, URI, HttpVer string
	Headers              map[string]string
}

type HttpResponse struct {
	HttpVer, Reason string
	StatusCode      int
	Headers         map[string]string
	Body            []byte
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

	for {
		// loop infinito esperando por conexoes
		conn, err := listen.Accept()

		if err != nil {
			// se ocorrer um erro, imprimir e esperar por novas conexoes
			log.PrintError(err)
		} else {
			// se nao houver erro, tratar conexao em outra goroutine
			go handler(connCount, conn, statsChan)
			connCount++
		}
	}
}

func handler(connId int, conn net.Conn, statsChan chan info.Stats) {
	defer conn.Close()

	statsChan <- info.Stats{CountActiveConn: 1}

	clientHostAddr := conn.RemoteAddr().String()

	log.LogInfo(connId, "Connection from %s\n", clientHostAddr)

	// criando leitor de mensagens da conexao
	var reader = bufio.NewReader(conn)
	var writer = bufio.NewWriter(conn)

	const N = 1024
	buf := make([]byte, N)

	var serverConn net.Conn
	var serverReader *bufio.Reader
	var serverWriter *bufio.Writer

	// loop de leitura de mensagens
OUTERLOOP:
	for {
		request := HttpRequest{}

		err := getRequestLine(reader, &request)

		if err != nil {
			log.LogInfo(connId, "Error in parse HTTP request line: %v\n", err)
			break OUTERLOOP
		}

		headers, err := getHeaderMap(reader)

		if err != nil {
			log.LogInfo(connId, "Error in parse HTTP request header: %v\n", err)
			break OUTERLOOP
		}

		request.Headers = headers

		host, ok := request.Headers["Host"]

		if ok {
			// cria conexao com servidor
			serverConn, err = net.Dial("tcp", host+":80")

			if err != nil {
				log.LogInfo(connId, "Error when trying to connect to host server %s: %v\n", host, err)
				break OUTERLOOP
			}

			serverReader = bufio.NewReader(serverConn)
			serverWriter = bufio.NewWriter(serverConn)

		} else {
			log.LogInfo(connId, "Host do not exist, get URI %s\n", request.URI)
			break OUTERLOOP
		}

		// faz requisicao a host server
		log.LogInfo(connId, "Requesting to host %s the resource %s\n", host, request.URI)

		// enviando corpo de requisicao http para o host server
		line := fmt.Sprintf("%s %s %s\r\n", request.Method, request.URI, request.HttpVer)
		serverWriter.Write([]byte(line))
		for name, value := range request.Headers {
			line = fmt.Sprintf("%s: %s\r\n", name, value)
			serverWriter.Write([]byte(line))
		}
		serverWriter.Write([]byte("\r\n"))
		serverWriter.Flush()

		statsChan <- info.Stats{
			LastHostsVisited:    []string{host},
			LastResourceVisited: []string{request.URI},
		}

		log.LogInfo(connId, "Processing host %s http response\n", host)

		response := HttpResponse{}

		err = getResponseStatusLine(serverReader, &response)

		if err != nil {
			log.LogInfo(connId, "(Host: %s) Error on parse HTTP response status line: %v\n", host, err)
			break OUTERLOOP
		}

		headers, err = getHeaderMap(serverReader)

		if err != nil {
			log.LogInfo(connId, "(Host: %s) Error on parse HTTP response header: %v\n", host, err)
			break OUTERLOOP
		}

		response.Headers = headers

		// lendo o corpo da resposta
	INNERLOOP:
		for {
			n, err := serverReader.Read(buf)

			switch err {
			case io.EOF:
				break INNERLOOP

			case nil:
				response.Body = append(response.Body, buf[:n]...)

			default:
				log.LogInfo(connId, "(Host: %s) Error when parsing response body: %v\n", host, err)
				break OUTERLOOP
			}
		}

		// enviando corpo de resposta http do servidor para o cliente do proxy
		line = fmt.Sprintf("%s %d %s\r\n", response.HttpVer, response.StatusCode, response.Reason)
		writer.Write([]byte(line))
		for name, value := range response.Headers {
			line = fmt.Sprintf("%s: %s\r\n", name, value)
			writer.Write([]byte(line))
		}
		writer.Write([]byte("\r\n"))
		writer.Write(response.Body)
		writer.Flush()

		// por enquanto, sempre fechar conexao com servidor
		serverConn.Close()
		serverConn = nil

		// decide se mantem conexao com cliente proxy
		if connValue, ok := response.Headers["Connection"]; ok && connValue == "close" {
			break OUTERLOOP
		} else {
			log.LogInfo(connId, "Keeping connection with %s\n", clientHostAddr)
		}
	}

	/*
		if serverConn != nil {
			LogInfo(connId, "Closing connection with host server %s\n", prevHost)
			serverConn.Close()
		}
	*/

	log.LogInfo(connId, "Closing connection with %s\n", clientHostAddr)

	statsChan <- info.Stats{CountActiveConn: -1}
}

func getRequestLine(reader *bufio.Reader, request *HttpRequest) error {
	buf, err := reader.ReadBytes('\n')

	if err == nil {
		line := string(buf)

		var method, uri, httpVer string
		n, err := fmt.Sscanf(line, "%s %s %s", &method, &uri, &httpVer)

		if n != 3 {
			err = errors.New("Mismatch status line of HTTP Request: " + line)
		}

		if err != nil {
			return err
		}

		request.Method = method
		request.URI = uri
		request.HttpVer = httpVer
	}

	return err
}

func getHeaderMap(reader *bufio.Reader) (map[string]string, error) {
	headers := make(map[string]string)

LOOP:
	for {
		buf, err := reader.ReadBytes('\n')

		switch err {
		case io.EOF:
			break LOOP

		case nil:
			line := string(buf)

			if len(line) <= 2 {
				break LOOP
			}

			var name, value string
			n, err := fmt.Sscanf(line, "%s %s", &name, &value)

			if n != 2 {
				err = errors.New("Mismatch header line: " + line)
			}

			if err != nil {
				return nil, err
			}

			name = name[:len(name)-1]
			headers[name] = value

		default:
			return nil, err
		}
	}

	return headers, nil
}

func getResponseStatusLine(reader *bufio.Reader, response *HttpResponse) error {
	buf, err := reader.ReadBytes('\n')

	if err == nil {
		var httpVer, statusCodeStr, reason string
		line := string(buf)
		n, err := fmt.Sscanf(line, "%s %s %s", &httpVer, &statusCodeStr, &reason)

		if n != 3 {
			err = errors.New("Mismatch status line of host server response: " + line)
		}

		if err != nil {
			return err
		}

		response.HttpVer = httpVer
		response.StatusCode, err = strconv.Atoi(statusCodeStr)

		if err != nil {
			return err
		}

		response.Reason = reason
	}

	return err
}
