package parser

import (
	"bufio"
	"errors"
	"fmt"
	"io"
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
	ExpiresTime     time.Time
}

func NewHttpRequest(reader *bufio.Reader) (HttpRequest, error) {
	request := HttpRequest{}

	err := getRequestLine(reader, &request)

	if err != nil {
		return request, err
	}

	headers, err := getHeaderMap(reader)

	if err != nil {
		return request, err
	}

	request.Headers = headers

	return request, nil
}

func NewHttpResponse(reader *bufio.Reader) (HttpResponse, error) {
	const N = 1024
	buf := make([]byte, N)
	response := HttpResponse{}

	err := getResponseStatusLine(reader, &response)

	if err != nil {
		return response, err
	}

	headers, err := getHeaderMap(reader)

	if err != nil {
		return response, err
	}

	response.Headers = headers

	// lendo o corpo da resposta
INNERLOOP:
	for {
		n, err := reader.Read(buf)

		switch err {
		case io.EOF:
			break INNERLOOP

		case nil:
			response.Body = append(response.Body, buf[:n]...)

		default:
			return response, err
		}
	}

	return response, nil
}

func WriteHttpResponse(writer *bufio.Writer, response *HttpResponse) {
	line := fmt.Sprintf("%s %d %s\r\n", response.HttpVer, response.StatusCode, response.Reason)
	writer.Write([]byte(line))
	for name, value := range response.Headers {
		line = fmt.Sprintf("%s: %s\r\n", name, value)
		writer.Write([]byte(line))
	}
	writer.Write([]byte("\r\n"))
	writer.Write(response.Body)
	writer.Flush()
}

func WriteHttpRequest(writer *bufio.Writer, request *HttpRequest) {
	line := fmt.Sprintf("%s %s %s\r\n", request.Method, request.URI, request.HttpVer)
	writer.Write([]byte(line))
	for name, value := range request.Headers {
		line = fmt.Sprintf("%s: %s\r\n", name, value)
		writer.Write([]byte(line))
	}
	writer.Write([]byte("\r\n"))
	writer.Flush()
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
