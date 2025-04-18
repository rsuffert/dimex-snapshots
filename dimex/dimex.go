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
	return func(d *Dimex) {
		d.fail = true
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

func (d *Dimex) Start() {
	go func() {
		for {
			select {
			case dmxR := <-d.Req: // vindo da  aplicação
				if dmxR == ENTER {
					d.outDbg("app pede mx")
					d.handleUponReqEntry()

				} else if dmxR == EXIT {
					d.outDbg("app libera mx")
					d.handleUponReqExit()
				}
			case msgOutro := <-d.Pp2plink.Ind: // vindo de outro processo
				if strings.Contains(msgOutro.Message, SNAP) {
					d.outDbg("         <<<---- snap!")
					d.handleIncomingSnap(
						d.messagesMiddleware(msgOutro),
					)
				} else if strings.Contains(msgOutro.Message, RESP_OK) {
					d.outDbg("         <<<---- responde! " + msgOutro.Message)
					d.handleUponDeliverRespOk(
						d.messagesMiddleware(msgOutro),
					)
				} else if strings.Contains(msgOutro.Message, REQ_ENTRY) {
					d.outDbg("          <<<---- pede??  " + msgOutro.Message)
					d.handleUponDeliverReqEntry(
						d.messagesMiddleware(msgOutro),
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
			turn := (t.Unix() / int64(snapshotIntervalSec)) % int64(len(d.addresses))
			if int(turn) != d.id {
				continue
			}

			logrus.Infof("=========== P%d initiating a snapshot ===========\n", d.id)
			snapId := 0
			if d.lastSnapshot != nil {
				snapId = d.lastSnapshot.ID + 1
			}
			// send a snapshot message to myself
			d.sendToLink(
				d.addresses[d.id],
				fmt.Sprintf("%s;%d", SNAP, snapId),
				fmt.Sprintf("PID %d", d.id),
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
func (d *Dimex) handleUponReqEntry() {
	d.lcl++
	d.reqTs = d.lcl
	d.nbrResps = 0
	for i := 0; i < len(d.addresses); i++ {
		if i != d.id {
			d.sendToLink(
				d.addresses[i],
				fmt.Sprintf("%s;%d;%d", REQ_ENTRY, d.id, d.reqTs),
				fmt.Sprintf("PID %d", d.id),
			)
		}
	}
	d.st = common.WantMX
}

/*
upon event [ dmx, Exit  |  r  ]  do

	para todo [p, r, ts ] em waiting
		trigger [ pl, Send | p , [ respOk, r ]  ]
	estado := naoQueroSC
	waiting := {}
*/
func (d *Dimex) handleUponReqExit() {
	for i := 0; i < len(d.waiting); i++ {
		if d.waiting[i] && i != d.id {
			d.sendToLink(
				d.addresses[i],
				fmt.Sprintf("%s;%d", RESP_OK, d.id),
				fmt.Sprintf("PID %d", d.id),
			)
		}
	}
	d.st = common.NoMX
	d.waiting = make([]bool, len(d.addresses))
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
func (d *Dimex) handleUponDeliverRespOk(msgOutro pp2plink.PP2PLink_Ind_Message) {
	d.nbrResps++

	if d.fail {
		// simulate a failure by counting the response twice
		d.nbrResps++
	}

	if d.nbrResps == len(d.addresses)-1 {
		d.st = common.InMX
		d.Ind <- dmxResp{}
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
func (d *Dimex) handleUponDeliverReqEntry(msgOutro pp2plink.PP2PLink_Ind_Message) {
	parts := strings.Split(msgOutro.Message, ";")
	otherId, _ := strconv.Atoi(parts[1])
	otherReqTs, _ := strconv.Atoi(parts[2])

	if d.st == common.NoMX || (d.st == common.WantMX && after(d.reqTs, d.id, otherReqTs, otherId)) {
		d.sendToLink(
			d.addresses[otherId],
			fmt.Sprintf("%s;%d", RESP_OK, d.id),
			fmt.Sprintf("PID %d", d.id),
		)
	} else {
		d.waiting[otherId] = true
	}

	d.lcl = max(d.lcl, otherReqTs)
}

func (m *Dimex) handleIncomingSnap(msg pp2plink.PP2PLink_Ind_Message) {
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

func (d *Dimex) sendToLink(address string, content string, space string) {
	d.outDbg(space + " ---->>>>   to: " + address + "     msg: " + content)
	d.Pp2plink.Req <- pp2plink.PP2PLink_Req_Message{
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

func (d *Dimex) outDbg(s string) {
	if d.dbg {
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
		InterceptedMsgs: make([]pp2plink.PP2PLink_Ind_Message, 0),
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

func (m *Dimex) messagesMiddleware(msg pp2plink.PP2PLink_Ind_Message) pp2plink.PP2PLink_Ind_Message {
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
