/*  Construido como parte da disciplina: FPPD - PUCRS - Escola Politecnica
    Professor: Fernando Dotti  (https://fldotti.github.io/)
    Modulo representando Algoritmo de Exclusão Mútua Distribuída:
    Semestre 2023/1
	Aspectos a observar:
	   mapeamento de módulo para estrutura
	   inicializacao
	   semantica de concorrência: cada evento é atômico
	   							  módulo trata 1 por vez
	Q U E S T A O
	   Além de obviamente entender a estrutura ...
	   Implementar o núcleo do algoritmo ja descrito, ou seja, o corpo das
	   funcoes reativas a cada entrada possível:
	   			handleUponReqEntry()  // recebe do nivel de cima (app)
				handleUponReqExit()   // recebe do nivel de cima (app)
				handleUponDeliverRespOk(msgOutro)   // recebe do nivel de baixo
				handleUponDeliverReqEntry(msgOutro) // recebe do nivel de baixo
*/

package DIMEX

import (
	PP2PLink "SD/PP2PLink"
	"SD/common"
	"SD/snapshots"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

const snapshotIntervalSec int = 1

// ------------------------------------------------------------------------------------
// ------- principais tipos
// ------------------------------------------------------------------------------------

const (
	REQ_ENTRY string = "reqEntry"
	RESP_OK   string = "respOk"
	SNAP      string = "snap"
)

type dmxReq int // enumeracao dos estados possiveis de um processo
const (
	ENTER dmxReq = iota
	EXIT
)

type dmxResp struct { // mensagem do módulo DIMEX infrmando que pode acessar - pode ser somente um sinal (vazio)
	// mensagem para aplicacao indicando que pode prosseguir
}

type DIMEX_Module struct {
	Req       chan dmxReq  // canal para receber pedidos da aplicacao (REQ e EXIT)
	Ind       chan dmxResp // canal para informar aplicacao que pode acessar
	addresses []string     // endereco de todos, na mesma ordem
	id        int          // identificador do processo - é o indice no array de enderecos acima
	st        common.State // estado deste processo na exclusao mutua distribuida
	waiting   []bool       // processos aguardando tem flag true
	lcl       int          // relogio logico local
	reqTs     int          // timestamp local da ultima requisicao deste processo
	nbrResps  int
	dbg       bool

	Pp2plink *PP2PLink.PP2PLink // acesso aa comunicacao enviar por PP2PLinq.Req  e receber por PP2PLinq.Ind

	lastSnapshot *snapshots.Snapshot
}

// ------------------------------------------------------------------------------------
// ------- inicializacao
// ------------------------------------------------------------------------------------

func NewDIMEX(_addresses []string, _id int, _dbg bool) *DIMEX_Module {

	p2p := PP2PLink.NewPP2PLink(_addresses[_id], _dbg)

	dmx := &DIMEX_Module{
		Req: make(chan dmxReq, 1),
		Ind: make(chan dmxResp, 1),

		addresses: _addresses,
		id:        _id,
		st:        common.NoMX,
		waiting:   make([]bool, len(_addresses)),
		lcl:       0,
		reqTs:     0,
		dbg:       _dbg,

		Pp2plink: p2p}

	for i := 0; i < len(dmx.waiting); i++ {
		dmx.waiting[i] = false
	}
	dmx.Start()
	dmx.outDbg("Init DIMEX!")
	return dmx
}

// ------------------------------------------------------------------------------------
// ------- nucleo do funcionamento
// ------------------------------------------------------------------------------------

func (module *DIMEX_Module) Start() {

	go func() {
		for {
			select {
			case dmxR := <-module.Req: // vindo da  aplicação
				if dmxR == ENTER {
					module.outDbg("app pede mx")
					module.handleUponReqEntry() // ENTRADA DO ALGORITMO

				} else if dmxR == EXIT {
					module.outDbg("app libera mx")
					module.handleUponReqExit() // ENTRADA DO ALGORITMO
				}

			case msgOutro := <-module.Pp2plink.Ind: // vindo de outro processo
				if strings.Contains(msgOutro.Message, SNAP) {
					module.outDbg("         <<<---- snap!")
					module.handleIncomingSnap(
						module.messagesMiddleware(msgOutro),
					)
				} else if strings.Contains(msgOutro.Message, RESP_OK) {
					module.outDbg("         <<<---- responde! " + msgOutro.Message)
					module.handleUponDeliverRespOk(
						module.messagesMiddleware(msgOutro),
					)

				} else if strings.Contains(msgOutro.Message, REQ_ENTRY) {
					module.outDbg("          <<<---- pede??  " + msgOutro.Message)
					module.handleUponDeliverReqEntry(
						module.messagesMiddleware(msgOutro),
					)

				}
			}
		}
	}()

	// every snapshotIntervalSec seconds, a process raises a snapshot
	go func() {
		ticker := time.NewTicker(time.Duration(snapshotIntervalSec) * time.Second)
		defer ticker.Stop()
		for t := range ticker.C {
			turn := (t.Unix() / int64(snapshotIntervalSec)) % int64(len(module.addresses))
			if int(turn) == module.id {
				logrus.Infof("=========== P%d initiating a snapshot ===========\n", module.id)
				snapId := 0
				if module.lastSnapshot != nil {
					snapId = module.lastSnapshot.ID + 1
				}
				// send a snapshot message to myself
				module.sendToLink(
					module.addresses[module.id],
					fmt.Sprintf("%s;%d", SNAP, snapId),
					fmt.Sprintf("PID %d", module.id),
				)
			}
		}
	}()
}

// ------------------------------------------------------------------------------------
// ------- tratamento de pedidos vindos da aplicacao
// ------- UPON ENTRY
// ------- UPON EXIT
// ------------------------------------------------------------------------------------

/*
upon event [ dmx, Entry  |  r ]  do

	lts.ts++
	myTs := lts
	resps := 0
	para todo processo p
		trigger [ pl , Send | [ reqEntry, r, myTs ]
	estado := queroSC
*/
func (module *DIMEX_Module) handleUponReqEntry() {
	module.lcl++
	module.reqTs = module.lcl
	module.nbrResps = 0
	for i := 0; i < len(module.addresses); i++ {
		if i != module.id {
			module.sendToLink(
				module.addresses[i],
				fmt.Sprintf("%s;%d;%d", REQ_ENTRY, module.id, module.reqTs),
				fmt.Sprintf("PID %d", module.id),
			)
		}
	}
	module.st = common.WantMX
}

/*
upon event [ dmx, Exit  |  r  ]  do

	para todo [p, r, ts ] em waiting
		trigger [ pl, Send | p , [ respOk, r ]  ]
	estado := naoQueroSC
	waiting := {}
*/
func (module *DIMEX_Module) handleUponReqExit() {
	for i := 0; i < len(module.waiting); i++ {
		if module.waiting[i] && i != module.id {
			module.sendToLink(
				module.addresses[i],
				fmt.Sprintf("%s;%d", RESP_OK, module.id),
				fmt.Sprintf("PID %d", module.id),
			)
		}
	}
	module.st = common.NoMX
	module.waiting = make([]bool, len(module.addresses))
}

// ------------------------------------------------------------------------------------
// ------- tratamento de mensagens de outros processos
// ------- UPON respOK
// ------- UPON reqEntry
// ------------------------------------------------------------------------------------

/*
upon event [ pl, Deliver | p, [ respOk, r ] ]

	resps++
	se resps = N
	então trigger [ dmx, Deliver | free2Access ]
		estado := estouNaSC
*/
func (module *DIMEX_Module) handleUponDeliverRespOk(msgOutro PP2PLink.PP2PLink_Ind_Message) {
	module.nbrResps++
	if module.nbrResps == len(module.addresses)-1 {
		module.st = common.InMX
		module.Ind <- dmxResp{}
	}
}

/*
upon event [ pl, Deliver | p, [ reqEntry, r, rts ]  do

	se (estado == naoQueroSC)   OR
			(estado == QueroSC AND  myTs >  ts)
	então  trigger [ pl, Send | p , [ respOk, r ]  ]
	senão
		se (estado == estouNaSC) OR
				(estado == QueroSC AND  myTs < ts)
		então  postergados := postergados + [p, r ]
		lts.ts := max(lts.ts, rts.ts)
*/
func (module *DIMEX_Module) handleUponDeliverReqEntry(msgOutro PP2PLink.PP2PLink_Ind_Message) {
	parts := strings.Split(msgOutro.Message, ";")
	otherId, _ := strconv.Atoi(parts[1])
	otherReqTs, _ := strconv.Atoi(parts[2])

	if module.st == common.NoMX || (module.st == common.WantMX && after(module.reqTs, module.id, otherReqTs, otherId)) {
		module.sendToLink(
			module.addresses[otherId],
			fmt.Sprintf("%s;%d", RESP_OK, module.id),
			fmt.Sprintf("PID %d", module.id),
		)
	} else {
		module.waiting[otherId] = true
	}

	module.lcl = max(module.lcl, otherReqTs)
}

func (m *DIMEX_Module) handleIncomingSnap(msg PP2PLink.PP2PLink_Ind_Message) {
	parts := strings.Split(msg.Message, ";")
	snapId, _ := strconv.Atoi(parts[1])

	takeSnapshot := m.lastSnapshot == nil || m.lastSnapshot.ID < snapId
	if takeSnapshot {
		logrus.Debugf("\t\tP%d: taking snapshot %d\n", m.id, snapId)
		m.takeSnapshot(snapId)
	}

	m.lastSnapshot.CollectedResps++

	snapshotOver := m.lastSnapshot.CollectedResps == (len(m.addresses) - 1)
	if snapshotOver {
		logrus.Debugf("\t\tP%d: snapshot %d completed. Dumping to file...\n", m.id, snapId)
		if err := m.lastSnapshot.DumpToFile(); err != nil {
			logrus.Errorf("P%d: error dumping snapshot %d to file: %v\n", m.id, snapId, err)
		}
	}
}

// ------------------------------------------------------------------------------------
// ------- funcoes de ajuda
// ------------------------------------------------------------------------------------

func (module *DIMEX_Module) sendToLink(address string, content string, space string) {
	module.outDbg(space + " ---->>>>   to: " + address + "     msg: " + content)
	module.Pp2plink.Req <- PP2PLink.PP2PLink_Req_Message{
		To:      address,
		Message: content}
}

func after(oneTs, oneId, otherTs, otherId int) bool {
	return oneTs > otherTs || (oneTs == otherTs && oneId > otherId)
}

func max(one, oth int) int {
	if one > oth {
		return one
	}
	return oth
}

func (module *DIMEX_Module) outDbg(s string) {
	if module.dbg {
		fmt.Println(". . . . . . . . . . . . [ DIMEX : " + s + " ]")
	}
}

func (m *DIMEX_Module) takeSnapshot(snapId int) {
	waiting := make([]bool, len(m.waiting))
	copy(waiting, m.waiting)
	m.lastSnapshot = &snapshots.Snapshot{
		ID:              snapId,
		PID:             m.id,
		State:           int(m.st),
		Waiting:         waiting,
		LocalClock:      m.lcl,
		ReqTs:           m.reqTs,
		NbrResps:        m.nbrResps,
		InterceptedMsgs: make([]PP2PLink.PP2PLink_Ind_Message, 0),
	}

	for i, addr := range m.addresses {
		if i != m.id {
			m.sendToLink(
				addr,
				fmt.Sprintf("%s;%d", SNAP, snapId),
				fmt.Sprintf("PID %d", m.id),
			)
			logrus.Debugf("P%d: sent SNAP to %s\n", m.id, addr)
		}
	}
}

func (m *DIMEX_Module) messagesMiddleware(msg PP2PLink.PP2PLink_Ind_Message) PP2PLink.PP2PLink_Ind_Message {
	if strings.Contains(msg.Message, SNAP) {
		logrus.Debugf("\tP%d: received SNAP from %s\n", m.id, msg.From)
		return msg
	}

	if m.lastSnapshot == nil {
		return msg
	}

	m.lastSnapshot.InterceptedMsgs = append(m.lastSnapshot.InterceptedMsgs, msg)

	return msg
}
