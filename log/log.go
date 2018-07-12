package log

import (
	"fmt"
	"strconv"
)

func PrintError(err error) {
	fmt.Printf("[ERROR] %s\n", err)
}

func LogInfo(connId int, format string, params ...interface{}) {
	fmt.Printf("[ID: "+strconv.Itoa(connId)+"] "+format, params...)
}