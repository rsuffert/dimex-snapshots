/*
  Construido como parte da disciplina: Sistemas Distribuidos - PUCRS - Escola Politecnica
  Professor: Fernando Dotti  (https://fldotti.github.io/)
  Modulo representando Perfect Point to Point Links tal como definido em:
    Introduction to Reliable and Secure Distributed Programming
    Christian Cachin, Rachid Gerraoui, Luis Rodrigues
  * Semestre 2018/2 - Primeira versao.  Estudantes:  Andre Antonitsch e Rafael Copstein
  * Semestre 2019/1 - Reaproveita conexões TCP já abertas - Estudantes: Vinicius Sesti e Gabriel Waengertner
  * Semestre 2020/1 - Separa mensagens de qualquer tamanho atee 4 digitos.
  Sender envia tamanho no formato 4 digitos (preenche com 0s a esquerda)
  Receiver recebe 4 digitos, calcula tamanho do buffer a receber,
  e recebe com io.ReadFull o tamanho informado - Dotti
  * Semestre 2022/1 - melhorias eliminando retorno de erro aos canais superiores.
  se conexao fecha nao retorna nada.   melhorias em comentarios.   adicionado modo debug. - Dotti
*/

package pp2plink

import (
	"fmt"
	"io"
	"net"
	"strconv"
)

type ReqMsg struct {
	To      string
	Message string
}

type IndMsg struct {
	From    string
	Message string
}

type PP2PLink struct {
	Ind   chan IndMsg
	Req   chan ReqMsg
	Run   bool
	dbg   bool
	Cache map[string]net.Conn // cache de conexoes - reaproveita conexao com destino ao inves de abrir outra
}

func NewPP2PLink(_address string, _dbg bool) *PP2PLink {
	p2p := &PP2PLink{
		Req:   make(chan ReqMsg, 1),
		Ind:   make(chan IndMsg, 1),
		Run:   true,
		dbg:   _dbg,
		Cache: make(map[string]net.Conn)}
	p2p.outDbg(" Init PP2PLink!")
	p2p.Start(_address)
	return p2p
}

func (m *PP2PLink) outDbg(s string) {
	if m.dbg {
		fmt.Println(". . . . . . . . . . . . . . . . . [ PP2PLink msg : " + s + " ]")
	}
}

func (m *PP2PLink) Start(address string) {

	// PROCESSO PARA RECEBIMENTO DE MENSAGENS
	go func() {
		listen, _ := net.Listen("tcp4", address)
		for {
			// aceita repetidamente tentativas novas de conexao
			conn, err := listen.Accept()
			m.outDbg("ok   : conexao aceita com outro processo.")
			// para cada conexao lanca rotina de tratamento
			go func() {
				// repetidamente recebe mensagens na conexao TCP (sem fechar)
				// e passa para modulo de cima
				for { //                              // enquanto conexao aberta
					if err != nil {
						fmt.Println(".", err)
						break
					}
					bufTam := make([]byte, 4) //       // le tamanho da mensagem
					_, err := io.ReadFull(conn, bufTam)
					if err != nil {
						m.outDbg("erro : " + err.Error() + " conexao fechada pelo outro processo.")
						break
					}
					tam, _ := strconv.Atoi(string(bufTam))
					bufMsg := make([]byte, tam)        // declara buffer do tamanho exato
					_, err = io.ReadFull(conn, bufMsg) // le do tamanho do buffer ou da erro
					if err != nil {
						fmt.Println("@", err)
						break
					}
					msg := IndMsg{
						From:    conn.RemoteAddr().String(),
						Message: string(bufMsg)}
					// ATE AQUI:  procedimentos para receber msg
					m.Ind <- msg //               // repassa mensagem para modulo superior
				}
			}()
		}
	}()

	// PROCESSO PARA ENVIO DE MENSAGENS
	go func() {
		for {
			message := <-m.Req
			m.Send(message)
		}
	}()
}

func (m *PP2PLink) Send(message ReqMsg) {
	var conn net.Conn
	var ok bool
	var err error

	// ja existe uma conexao aberta para aquele destinatario?
	if conn, ok = m.Cache[message.To]; ok {
	} else { // se nao existe, abre e guarda na cache
		conn, err = net.Dial("tcp", message.To)
		m.outDbg("ok   : conexao iniciada com outro processo")
		if err != nil {
			fmt.Println(err)
			return
		}
		m.Cache[message.To] = conn
	}
	// calcula tamanho da mensagem e monta string de 4 caracteres numericos com o tamanho.
	// completa com 0s aa esquerda para fechar tamanho se necessario.
	str := strconv.Itoa(len(message.Message))
	for len(str) < 4 {
		str = "0" + str
	}
	if !(len(str) == 4) {
		m.outDbg("ERROR AT PPLINK MESSAGE SIZE CALCULATION - INVALID MESSAGES MAY BE IN TRANSIT")
	}
	payload := append([]byte(str), []byte(message.Message)...) // escreve 4 caracteres com tamanho da mensagem e a mensagem
	_, err = conn.Write(payload)
	if err != nil {
		m.outDbg("erro : " + err.Error() + ". Conexao fechada. 1 tentativa de reabrir:")
		conn, err = net.Dial("tcp", message.To)
		if err != nil {
			//fmt.Println(err)
			m.outDbg("       " + err.Error())
			return
		} else {
			m.outDbg("ok   : conexao iniciada com outro processo.")
		}
		m.Cache[message.To] = conn
		conn.Write(payload)
	}
}
