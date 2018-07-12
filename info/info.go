package info

import (
	"strconv"
	"time"
	"net"
	"fmt"
	"gowebproxy/log"
	"sort"
)

type Resource struct{
	Name string
	Size int
}

type ResourceCount struct{
	Name string
	Count int
}

type HostCount struct{
	Host string
	Count int
}

type Stats struct {
	LastHostsVisited []string
	LastResourceVisited []Resource
	ActiveConn int
	StartTime time.Time
}

type List struct{
	HostsVisited []string
	ResourceVisited []Resource 
	StartTime time.Time
	CountActiveConn int
}

var memory List

func handler(conn net.Conn, statChan chan Stats){
	defer conn.Close()

	//Map para descobrir quantas vezes um host foi visitado
	mpHost := make(map[string]int)
	//Iterando para obter quantas vezes cada host foi visitado
	for _, host := range memory.HostsVisited {
		mpHost[host] += 1
	}

	//Array para preparar as estatística sobre o número de visitas em cada host
	var hostsStatistic []HostCount
	for k, v := range mpHost {
		hostsStatistic = append(hostsStatistic, HostCount{k, v})
	}

	//Map para cada objeto associar com seu tamanho
	mpResourceSize := make(map[string]int)
	//Map para cada objeto contar o número de vezes que ele foi requisitado
	mpResource := make(map[string]int)

	for _, resource := range memory.ResourceVisited {
		mpResourceSize[resource.Name] = resource.Size
		mpResource[resource.Name] += 1
	}

	//Array para guardar para cada objeto requisitado seu tamanho
	var resourceStatisticSize []Resource
	for k, v := range mpResourceSize {
		resourceStatisticSize = append(resourceStatisticSize, Resource{k, v})
	}
	
	//Array para guardar para cada objeto o número de vezes que ele foi requisitado
	var resourceStatistic []ResourceCount
	for k, v := range mpResource{
		resourceStatistic = append(resourceStatistic, ResourceCount{k, v})
	}

	//Agoro ordeno minhas estatísticas
	sort.Slice(hostsStatistic, func(i, j int) bool{//Coloco os hosts com maiores números de requisições primeiro
		return hostsStatistic[i].Count > hostsStatistic[j].Count
	})

	sort.Slice(resourceStatistic, func(i, j int) bool{//Coloco os objetos com maiores números de requisições primeiro
		return resourceStatistic[i].Count > resourceStatistic[j].Count
	})

	sort.Slice(resourceStatisticSize, func(i, j int) bool{//Coloco os objetos de maior tamanho primeiro
		return resourceStatisticSize[i].Size > resourceStatisticSize[j].Size;
	})
}

func ListenProxy(statChan chan Stats){
	for{
		// Espera por uma resposta do servidor proxy
		st := <-statChan

		// Atualizo o número de conexões ativas
		memory.CountActiveConn += st.ActiveConn

		// Atualizo o tempo da conexão
		if time.Time.IsZero(st.StartTime) == false {
			memory.StartTime = st.StartTime
		}
		
		// Adiciono os últimos hosts requisitados na lista
		memory.HostsVisited = append(memory.HostsVisited, st.LastHostsVisited...)
		
		// Adiciono os últimos objetos requisitados na lista
		memory.ResourceVisited = append(memory.ResourceVisited, st.LastResourceVisited...)
	}
}

func InfoServer(port int, statChan chan Stats) {
	// esperar respostas do servidor proxy
	go ListenProxy(statChan)

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
			go handler(conn, statChan)
		}
	}	
}
