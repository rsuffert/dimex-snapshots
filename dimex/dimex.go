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

package dimex

import (
	"fmt"
	"pucrs/sd/common"
	"pucrs/sd/pp2plink"
	"pucrs/sd/snapshots"
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

type Opt func(*Dimex)

type Dimex struct {
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
	fail      bool // flag to simulate failures and trigger snapshot invariant violations

	Pp2plink *pp2plink.PP2PLink // acesso aa comunicacao enviar por PP2PLinq.Req  e receber por PP2PLinq.Ind

	lastSnapshot *snapshots.Snapshot
}

// ------------------------------------------------------------------------------------
// ------- options to customize the module
// ------------------------------------------------------------------------------------

// WithFailOpt is an option to enable failure simulation for the DIMEX module
// and trigger snapshot invariant violations.
func WithFailOpt() Opt {
	return func(m *Dimex) {
		m.fail = true
	}
}

// ------------------------------------------------------------------------------------
// ------- inicializacao
// ------------------------------------------------------------------------------------

func NewDimex(_addresses []string, _id int, opts ...Opt) *Dimex {
	p2p := pp2plink.NewPP2PLink(_addresses[_id], false)

	dmx := &Dimex{
		Req: make(chan dmxReq, 1),
		Ind: make(chan dmxResp, 1),

		addresses: _addresses,
		id:        _id,
		st:        common.NoMX,
		waiting:   make([]bool, len(_addresses)),
		lcl:       0,
		reqTs:     0,
		dbg:       false,
		fail:      false,

		Pp2plink: p2p,
	}

	for _, opt := range opts {
		opt(dmx)
	}

	dmx.Start()
	dmx.outDbg("Init DIMEX!")

	return dmx
}

// ------------------------------------------------------------------------------------
// ------- nucleo do funcionamento
// ------------------------------------------------------------------------------------

func (m *Dimex) Start() {
	go func() {
		for {
			select {
			case dmxR := <-m.Req: // vindo da  aplicação
				if dmxR == ENTER {
					m.outDbg("app pede mx")
					m.handleUponReqEntry()

				} else if dmxR == EXIT {
					m.outDbg("app libera mx")
					m.handleUponReqExit()
				}
			case msgOutro := <-m.Pp2plink.Ind: // vindo de outro processo
				if strings.Contains(msgOutro.Message, SNAP) {
					m.outDbg("         <<<---- snap!")
					m.handleIncomingSnap(
						m.messagesMiddleware(msgOutro),
					)
				} else if strings.Contains(msgOutro.Message, RESP_OK) {
					m.outDbg("         <<<---- responde! " + msgOutro.Message)
					m.handleUponDeliverRespOk(
						m.messagesMiddleware(msgOutro),
					)
				} else if strings.Contains(msgOutro.Message, REQ_ENTRY) {
					m.outDbg("          <<<---- pede??  " + msgOutro.Message)
					m.handleUponDeliverReqEntry(
						m.messagesMiddleware(msgOutro),
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
			turn := (t.Unix() / int64(snapshotIntervalSec)) % int64(len(m.addresses))
			if int(turn) != m.id {
				continue
			}

			logrus.Infof("=========== P%d initiating a snapshot ===========\n", m.id)
			snapId := 0
			if m.lastSnapshot != nil {
				snapId = m.lastSnapshot.ID + 1
			}
			// send a snapshot message to myself
			m.sendToLink(
				m.addresses[m.id],
				fmt.Sprintf("%s;%d", SNAP, snapId),
				fmt.Sprintf("PID %d", m.id),
			)
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
func (m *Dimex) handleUponReqEntry() {
	m.lcl++
	m.reqTs = m.lcl
	m.nbrResps = 0
	for i := 0; i < len(m.addresses); i++ {
		if i != m.id {
			m.sendToLink(
				m.addresses[i],
				fmt.Sprintf("%s;%d;%d", REQ_ENTRY, m.id, m.reqTs),
				fmt.Sprintf("PID %d", m.id),
			)
		}
	}
	m.st = common.WantMX
}

/*
upon event [ dmx, Exit  |  r  ]  do

	para todo [p, r, ts ] em waiting
		trigger [ pl, Send | p , [ respOk, r ]  ]
	estado := naoQueroSC
	waiting := {}
*/
func (m *Dimex) handleUponReqExit() {
	for i := 0; i < len(m.waiting); i++ {
		if m.waiting[i] && i != m.id {
			m.sendToLink(
				m.addresses[i],
				fmt.Sprintf("%s;%d", RESP_OK, m.id),
				fmt.Sprintf("PID %d", m.id),
			)
		}
	}
	m.st = common.NoMX
	m.waiting = make([]bool, len(m.addresses))
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
func (m *Dimex) handleUponDeliverRespOk(msgOutro pp2plink.IndMsg) {
	m.nbrResps++

	if m.fail {
		// simulate a failure by counting the response twice
		m.nbrResps++
	}

	if m.nbrResps == len(m.addresses)-1 {
		m.st = common.InMX
		m.Ind <- dmxResp{}
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
func (m *Dimex) handleUponDeliverReqEntry(msgOutro pp2plink.IndMsg) {
	parts := strings.Split(msgOutro.Message, ";")
	otherId, _ := strconv.Atoi(parts[1])
	otherReqTs, _ := strconv.Atoi(parts[2])

	if m.st == common.NoMX || (m.st == common.WantMX && after(m.reqTs, m.id, otherReqTs, otherId)) {
		m.sendToLink(
			m.addresses[otherId],
			fmt.Sprintf("%s;%d", RESP_OK, m.id),
			fmt.Sprintf("PID %d", m.id),
		)
	} else {
		m.waiting[otherId] = true
	}

	m.lcl = max(m.lcl, otherReqTs)
}

func (m *Dimex) handleIncomingSnap(msg pp2plink.IndMsg) {
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

func (m *Dimex) sendToLink(address string, content string, space string) {
	m.outDbg(space + " ---->>>>   to: " + address + "     msg: " + content)
	m.Pp2plink.Req <- pp2plink.ReqMsg{
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

func (m *Dimex) outDbg(s string) {
	if m.dbg {
		fmt.Println(". . . . . . . . . . . . [ DIMEX : " + s + " ]")
	}
}

func (m *Dimex) takeSnapshot(snapId int) {
	waiting := make([]bool, len(m.waiting))
	copy(waiting, m.waiting)
	m.lastSnapshot = &snapshots.Snapshot{
		ID:              snapId,
		PID:             m.id,
		State:           m.st,
		Waiting:         waiting,
		LocalClock:      m.lcl,
		ReqTs:           m.reqTs,
		NbrResps:        m.nbrResps,
		InterceptedMsgs: make([]pp2plink.IndMsg, 0),
	}

	for i, addr := range m.addresses {
		if i == m.id {
			continue
		}
		m.sendToLink(
			addr,
			fmt.Sprintf("%s;%d", SNAP, snapId),
			fmt.Sprintf("PID %d", m.id),
		)
		logrus.Debugf("P%d: sent SNAP to %s\n", m.id, addr)
	}
}

func (m *Dimex) messagesMiddleware(msg pp2plink.IndMsg) pp2plink.IndMsg {
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
